# Database MCP Server - Technical Specifications

## Overview

Database MCP Server is a production MCP provider for SQL databases, built in Go.

Current implementation baseline:
- Runtime version: `v1.6.2`
- Registered tools: `21`
- Go module baseline: `go 1.26` (`toolchain go1.26.1`)
- MCP SDK: `github.com/modelcontextprotocol/go-sdk v1.2.0`

## Runtime Architecture

```text
MCP Client
  -> MCP Server Layer (internal/mcp)
     -> Config Layer (internal/config)
     -> DB Layer (internal/db)
     -> Logging Layer (internal/log)
```

## Transports

- Stdio: default and recommended for local/client process execution
- HTTP/SSE: optional, enabled when `MCP_SSE_ADDR` is set

## Core Components

- `cmd/server/main.go`
  - startup, config bootstrap, transport wiring
- `internal/mcp/server.go`
  - MCP server creation, tool registration, handlers
  - Schema resolution for analyze operations (`resolveSchemaForAnalyze`)
  - Privilege warning detection for MySQL/MariaDB/PostgreSQL
- `internal/mcp/schema_tracker.go`
  - schema tracking/history/migration/drift flows
- `internal/mcp/federation_*.go`
  - federated query parser/planner/executor/join logic
- `internal/mcp/analyze_*.go`, `internal/mcp/analyze_schema_types.go`
  - analyze-schema request/response types, including `Warnings []string` for privilege issues
- `internal/mcp/insights_*.go`
  - insight discovery and statistics processing
- `internal/config/config.go`
  - profile persistence and encryption helpers
- `internal/db/driver.go`
  - database/sql DSN and pooled connection setup
- `internal/log/logger.go`
  - structured logging and rotation

## Tool Surface (Implemented)

The server registers these 21 tools:
- `configure-profile`
- `list-profiles`
- `execute-sql`
- `list-tables`
- `describe-table`
- `list-databases`
- `get-search-path`
- `analyze-schema`
- `smart-query-builder`
- `optimize-query`
- `validate-query`
- `analyze-data-lineage`
- `discover-insights`
- `track-schema-changes`
- `federated-query`
- `discover-joins`
- `sample-data`
- `mcp-info`
- `list-tools`
- `get-tool-help`

## Security Model

- AES-GCM credential encryption at rest
- Read-only profile enforcement for write protection
- Parameterized SQL support (`params`) to reduce injection risk
- Structured errors and safe logging (no raw credential output)
- Schema-aware query construction: `resolveSchemaForAnalyze` correctly passes database name for MySQL/MariaDB (not empty string)
- Privilege detection warnings in `analyze-schema` response (`Warnings []string`) when tables exist but columns are inaccessible
- Domain inference via naming prefix signals (not hardcoded domain patterns) — `ComputeDomainSignals` extracts table prefix frequencies for LLM interpretation
- FK-based structural table categorization — tables with 2+ outgoing FKs classified as junction; FK signal counts on `TableEntity`
- Index coverage signals — `IndexCoverage` struct reports total/indexed/unindexed FK columns and tables without primary keys
- Error warnings for FK/index/row count fetch failures instead of silent discard

## Performance Characteristics

- Uses `database/sql` pooling via `SetMaxOpenConns` and `SetMaxIdleConns`
- Stateless tool handlers for horizontal scaling compatibility
- Query analysis tools (`optimize-query`, `validate-query`) separate planning from execution

## Database Support

- MySQL and MariaDB (`github.com/go-sql-driver/mysql v1.9.3`)
- PostgreSQL (`github.com/lib/pq v1.11.1`)
- SQLite (`github.com/mattn/go-sqlite3 v1.14.33`)

## Packaging and Deployment

### Container

- Dockerfile is multi-stage and builds `./cmd/server/main.go`
- Published images:
  - `ghcr.io/guyinwonder168/database-mcp-server:v1.5.1`
  - `ghcr.io/guyinwonder168/database-mcp-server:latest`

### Release

- GitHub Releases publish versioned binary assets by tag
- GHCR package workflow publishes container images for tags

## Testing Strategy

- Unit tests adjacent to implementation (`*_test.go`)
- Integration tests in `internal/mcp/*integration*_test.go`
- **MySQL/PostgreSQL mocking** with go-sqlmock for database-specific tests (see `server_analyze_schema_helpers_test.go`)
- **SQLite tests** use real in-memory databases for actual SQLite functionality testing
- Optional live DB tests through `DB_MCP_IT_*` environment variables

## Source-of-Truth Note

If this document conflicts with runtime behavior, runtime code is authoritative:
- `internal/mcp/server.go`
- `docs/mcp-openapi.yaml`
