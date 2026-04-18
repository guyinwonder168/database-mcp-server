# Database MCP Server - Architecture

## System Architecture Overview

```text
MCP Client
  -> MCP Server Layer (internal/mcp)
     -> Analyze Package (internal/mcp/analyze)
     -> Lineage Package (internal/mcp/lineage)
     -> NLP Package (internal/mcp/nlp)
     -> Context Package (internal/mcp/context)
     -> Config Layer (internal/config)
     -> DB Layer (internal/db)
     -> Logging Layer (internal/log)
```

## Core Components

### Entry Point
- `cmd/server/main.go`
  - startup and transport setup
  - first-run configuration bootstrap

### MCP Layer (`internal/mcp`)
- `server.go`
  - server initialization
  - registration of 20 tools
  - core handlers for profile, SQL, schema, and metadata operations
  - thin handler for analyze-schema (delegates to analyze.Run())
- `analyze_schema_types.go`
  - shared types for analyze-schema request/response
- `insights_*.go`
  - KPI/trend/anomaly/distribution analysis
- `schema_tracker*.go`, `schema_storage.go`, `schema_migrations.go`
  - schema snapshots, drift detection, migration generation
- `federation_*.go`
  - federated query parsing, planning, execution, result joins
- `errors.go`
  - structured error analysis and suggestions
- `tool_help.go`
  - tool description/help metadata generation
- `query_validator.go`, `query_optimizer.go`, `performance_estimator.go`
  - query validation, optimization, and performance estimation
- `profiling_engine.go`, `profiling_types.go`
  - data profiling engine for analyze-schema
- `optimization_rules.go`
  - optimization rule definitions
- `helpers.go`
  - shared helper functions

### Analyze Package (`internal/mcp/analyze`)
- `analyzer.go` — Run() orchestration, table schema building, relationship graph, classification signals
- `columns.go` — Bulk column fetching with parameterized queries (TVF for SQLite)
- `relationships.go` — Real FK discovery, implicit relationships, relationship graph building
- `performance.go` — Index analysis, performance optimization recommendations
- `enrichment.go` — Business context inference, data pattern recognition, quality metrics, table categorization
- `rows.go` — Sample row fetching and normalization
- `sanitize.go` — SQL identifier validation and per-DB quoting helpers
- `types.go` — Shared types (TableInfo, SchemaColumnInfo, DataPattern, etc.)
- `doc.go` — Package documentation

### Lineage Package (`internal/mcp/lineage`)
- `analyzer.go` — Data lineage analysis
- `dependency_graph.go` — Table dependency graph construction

### NLP Package (`internal/mcp/nlp`)
- `entity_extractor.go` — Entity extraction for natural language queries
- `intent_classifier.go` — Intent classification for smart-query-builder

### Context Package (`internal/mcp/context`)
- `manager.go` — Conversation context management
- `conversation.go` — Conversation state tracking

### Configuration Layer (`internal/config`)
- encrypted profile persistence (AES-GCM)
- load/save config and first-run defaults

### Database Layer (`internal/db`)
- DSN construction by database type
- pooled `database/sql` connections

### Logging Layer (`internal/log`)
- structured JSON logs
- rotation and controlled stdout behavior

## Runtime Contracts

### Registered Tools (20)
- `configure-profile`, `list-profiles`, `execute-sql`
- `list-tables`, `describe-table`, `list-databases`, `list-schemas`, `get-search-path`
- `analyze-schema`, `smart-query-builder`, `discover-joins`, `sample-data`
- `optimize-query`, `validate-query`, `analyze-data-lineage`, `discover-insights`
- `track-schema-changes`, `federated-query`
- `mcp-info`, `list-tools`, `get-tool-help`

### Resources
- `tools://list` (registered tools snapshot)
- `profile://{profile}` (redacted profile metadata)

## Architectural Decisions

1. Stateless handler execution for scalability and clarity
2. Tool-based MCP contracts for interoperability
3. Read-only policy as first-class guardrail
4. Separation of concern between handler orchestration and feature-specific engines
5. Pure functions in analyze/lineage/nlp/context packages — no MCPServer dependencies for testability
6. SQL injection prevention via `sanitizeIdentifier()` + `quoteForDB()` + parameterized queries
7. SQLite PRAGMAs accessed via table-valued functions for parameterized queries

## File Layout (High-Level)

- `cmd/server/main.go`
- `internal/config/*`
- `internal/db/*`
- `internal/log/*`
- `internal/mcp/*`
- `internal/mcp/analyze/*`
- `internal/mcp/lineage/*`
- `internal/mcp/nlp/*`
- `internal/mcp/context/*`
- `docs/*`
- `.kilocode/rules/memory-bank/*`

## Source of Truth

When architecture docs diverge from implementation, defer to:
1. `internal/mcp/server.go`
2. `internal/mcp/analyze/` (for analyze-schema logic)
3. `docs/mcp-openapi.yaml`
