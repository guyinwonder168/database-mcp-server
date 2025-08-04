# Database MCP Server - Architecture

## System Architecture Overview

The Database MCP Server follows a layered architecture pattern with clear separation of concerns:

```
┌─────────────────────────────────────┐
│         MCP Client (AI Agent)       │
└─────────────────┬───────────────────┘
                  │ stdio/JSON-RPC
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
│  ┌──────┬──────┬────────────────┐  │
│  │ Log  │ AES  │ DB Drivers     │  │
│  └──────┴──────┴────────────────┘  │
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
  - Registers all MCP tools/actions
  - Routes requests to appropriate handlers
  - Uses official Go MCP SDK
  - Handler methods (not found in current codebase):
    - handleConfigureProfile
    - handleListProfiles
    - handleExecuteSQL
    - handleListTables
    - handleDescribeTable
    - handleListDatabases
    - handleMCPInfo (implemented)

#### Configuration Management (internal/config/)
- **config.go**: Profile and configuration management
  - Profile struct: database connection details
  - Config struct: profiles list, pool size, encryption key
  - LoadConfig/SaveConfig: YAML file operations
  - PromptForProfiles: Interactive CLI setup
  - AES-GCM encryption/decryption helpers

#### Database Abstraction (internal/db/)
- **driver.go**: Database connection management
  - OpenConnectionWithPool: Creates pooled connections
  - DSN: Builds connection strings for different DB types
  - Supports: MySQL, MariaDB, PostgreSQL, SQLite

#### Logging (internal/log/)
- **logger.go**: Structured JSON logging
  - File rotation (500KB limit)
  - Multi-output (stdout + file)
  - JSON format with timestamps

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
├── internal/
│   ├── config/
│   │   ├── config.go         # Configuration management
│   │   └── config_template.yaml
│   ├── db/
│   │   └── driver.go         # Database abstraction
│   ├── log/
│   │   └── logger.go         # Logging infrastructure
│   └── mcp/
│       ├── server.go         # MCP server core
│       └── server_test.go    # Unit tests
├── go.mod                    # Go module definition
├── go.sum                    # Dependency lock file
├── config.yaml              # Runtime configuration (generated)
└── mcp-provider.log         # Runtime logs (generated)
```

## Critical Implementation Paths

1. **First Run**: main.go → PromptForProfiles → SaveConfig
2. **Profile Management**: MCP request → handleConfigureProfile → LoadConfig → SaveConfig
3. **SQL Execution**: MCP request → handleExecuteSQL → OpenConnection → Execute → Return results
4. **Schema Discovery**: MCP request → handleListTables → Query information_schema → Return metadata