# Database MCP Server - Architecture

## System Architecture Overview

The Database MCP Server follows a layered architecture pattern with clear separation of concerns:

```
┌─────────────────────────────────────┐
│         MCP Client (AI Agent)       │
└─────────────────┬───────────────────┘
                  │ stdio
┌─────────────────┴───────────────────┐
│          MCP Server Layer           │
│    (internal/mcp/server.go)         │
├─────────────────────────────────────┤
│         Business Logic              │
│  ┌─────────────┬─────────────────┐  │
│  │   Config    │   Database      │  │
│  │  Manager    │   Connector     │  │
│  └─────────────┴─────────────────┘  │
├─────────────────────────────────────┤
│         Infrastructure              │
│  ┌──────┬──────┬────────────────┐   │
│  │ Log  │ AES  │ DB Drivers     │   │
│  └──────┴──────┴────────────────┘   │
└─────────────────────────────────────┘
```

## Component Structure

### Entry Point
- **cmd/server/main.go**: Application entry point
  - Initializes logger
  - Handles first-run configuration wizard
  - Starts MCP server

### Core Components

#### MCP Server (internal/mcp/)
- **server.go**: MCP server implementation
  - Registers all MCP tools/actions (15 total, fully documented in README)
  - Routes requests to appropriate handlers
  - Uses official Go MCP SDK (v1.2.0)
  - Enhanced error handling with structured error responses and actionable suggestions
  - Registers MCP resources:
    - `tools://list` (JSON snapshot of tools registry)
    - `profile://{profile}` (profile metadata, secrets redacted)
  - Optional SSE transport: when `MCP_SSE_ADDR` is set, `cmd/server/main.go` starts `mcpsdk.NewSSEHandler` HTTP server alongside stdio.
  - Handler methods:
    - handleConfigureProfile
    - handleListProfiles
    - handleExecuteSQL (with enhanced read-only enforcement and CTE support)
    - handleListTables
    - handleDescribeTable
    - handleListDatabases
    - handleMCPInfo
    - handleSampleData
    - handleListTools
    - handleDeleteProfile
    - handleUpdateProfile
    - handleAnalyzeSchema (comprehensive analysis with business context inference, data quality metrics, relationship discovery)
    - handleSmartQueryBuilder (natural language intent processing)
    - handleDiscoverJoins (foreign key and semantic relationship detection)
  - Resource handlers:
    - resourceToolsHandler
    - resourceProfileHandler
- **analyze_schema_types.go**: Type system for schema analysis
  - Comprehensive type definitions for all analysis levels
  - Business context inference structures
  - Data quality metrics and pattern recognition
  - Relationship graph visualization
  - AI query suggestions integration
- **errors.go**: Structured error handling system
  - Error codes and categorization
  - Actionable suggestions for recovery
  - Context-aware error messages
- **server_test.go**: Comprehensive unit tests
  - Test coverage for all MCP tools
  - Schema analysis validation tests
- **tools_list_integration_test.go**: MCP tool discovery regression tests
  - Prevents future tools/list bugs
  - Verifies all 15 tools are discoverable by MCP clients
  - Tests capabilities advertisement and tool registry functionality

#### Configuration Management (internal/config/)
- **config.go**: Profile and configuration management
  - Profile struct: database connection details
  - Config struct: profiles list, pool size, encryption key
  - LoadConfig/SaveConfig: YAML file operations
  - PromptForProfiles: Interactive CLI setup
  - AES-GCM encryption/decryption helpers
  - Configuration cleanup: user_key/user_secret fields removed for security and clarity

#### Database Abstraction (internal/db/)
- **driver.go**: Database connection management
  - OpenConnectionWithPool: Creates pooled connections
  - DSN: Builds connection strings for different DB types
  - Supports: MySQL, MariaDB, PostgreSQL, SQLite

#### Logging (internal/log/)
- **logger.go**: Structured JSON logging
  - File rotation (500KB limit)
  - JSON format with timestamps
  - Stdout logging disabled by default to avoid MCP stdio contamination; enable via `MCP_LOG_TO_STDOUT=true`

## Data Flow

### Configuration Flow
1. Check for config.yaml existence
2. If missing, run interactive CLI wizard
3. Encrypt passwords with AES-GCM
4. Save configuration to YAML file

### MCP Action Flow
1. Receive MCP action request via stdio
2. Route to appropriate handler
3. Load configuration
4. Decrypt credentials
5. Open database connection (with pooling)
6. Execute operation
7. Close connection (return to pool)
8. Return structured response

## Security Architecture

### Credential Protection
- Passwords encrypted using AES-GCM
- 32-character encryption key required
- Key stored in config.yaml (should be environment variable in production)
- No plaintext passwords in memory or logs

### Access Control
- Read-only profile flag
- SQL injection prevention through parameterized queries
- Connection-level access control (database permissions)

## Design Patterns

1. **Factory Pattern**: Database driver selection based on profile type
2. **Repository Pattern**: Configuration persistence abstraction
3. **Command Pattern**: MCP action handlers
4. **Singleton Pattern**: Logger instance
5. **Pool Pattern**: Database connection pooling

## File Organization

```
database-mcp-provider/
├── cmd/
│   └── server/
│       └── main.go           # Entry point
├── docs/                      # Comprehensive documentation suite
│   ├── README.md              # Main documentation
│   ├── prd.md                # Product Requirements Document
│   ├── prd-analysis-report.md # PRD analysis with AI perspective
│   ├── technical-specifications.md # Technical architecture details
│   ├── api-documentation.md # API documentation
│   ├── mcp-openapi.yaml      # OpenAPI specification
│   ├── mcp-examples.md       # MCP usage examples
│   ├── schema-introspection-queries.md # Database-specific queries
│   ├── smart-query-builder-implementation-plan.md # Query builder design
│   ├── analyze-schema-design.md # Schema analysis architecture
│   ├── test-enhanced-schema.md # Test schema documentation
│   └── implementation-status.md # Implementation tracking
├── internal/
│   ├── config/
│   │   ├── config.go         # Configuration management
│   │   └── config_template.yaml
│   ├── db/
│   │   └── driver.go         # Database abstraction
│   ├── log/
│   │   └── logger.go         # Logging infrastructure
│   └── mcp/
│       ├── analyze_schema_types.go # Analyze-schema type system
│       ├── server.go              # MCP server core (all handlers + resources)
│       ├── errors.go             # Structured error handling
│       ├── server_test.go         # Comprehensive unit tests
│       ├── tools_list_integration_test.go # tools/list regression tests
│       └── integration_live_test.go       # Live Postgres/MySQL smoke tests (env-driven)
├── go.mod                    # Go module definition (Go 1.25.5 toolchain)
├── go.sum                    # Dependency lock file
├── CHANGELOG.md              # Release notes (v1.0.1 includes MCP tool fix, error payload docs, live DB smoke)
├── config.yaml              # Runtime configuration (generated)
└── mcp-provider.log         # Runtime logs (generated)
```

## Critical Implementation Paths

1. **First Run**: main.go → PromptForProfiles → SaveConfig
2. **Profile Management**: MCP request → handleConfigureProfile → LoadConfig → SaveConfig
3. **SQL Execution**: MCP request → handleExecuteSQL → OpenConnection → Execute → Return results
4. **Schema Discovery**: MCP request → handleListTables → Query information_schema → Return metadata
5. **Sample Data Fetching**: MCP request → handleSampleData → OpenConnection → Execute LIMIT query → Return results
6. **Tool Enumeration**: MCP request → handleListTools → Dynamic tool registry → Return tool list
7. **Analyze-Schema**: MCP request → handleAnalyzeSchema → analyze_schema_types.go → OpenConnection → Analyze schema (BASIC/DETAILED/COMPREHENSIVE) → Infer business context, data quality, relationships → Smart Query Builder → Return analysis results

## Documentation and Error Handling

- All 15 MCP tools are fully documented in README.md, including analyze-schema configuration and usage for all supported databases.
- Enhanced schema introspection, schema analysis, and structured error handling are implemented and documented.
- Configuration cleanup ensures only relevant fields are present, improving security and maintainability.
- MCP tool detection fix implemented: Upgraded to Go SDK v1.2.0 (fixing earlier SDK bugs), with comprehensive regression tests to prevent future issues.
