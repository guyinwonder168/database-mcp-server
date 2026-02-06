# Database MCP Server - Architecture

## System Architecture Overview

```text
MCP Client
  -> MCP Server Layer (internal/mcp)
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
  - registration of 18 tools
  - core handlers for profile, SQL, schema, and metadata operations
- `insights_*.go`
  - KPI/trend/anomaly/distribution analysis
- `schema_tracker*.go`, `schema_storage.go`, `schema_migrations.go`
  - schema snapshots, drift detection, migration generation
- `federation_*.go`
  - federated query parsing, planning, execution, result joins
- `errors.go`
  - structured error analysis and suggestions

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

### Registered Tools (18)
- `configure-profile`, `list-profiles`, `execute-sql`
- `list-tables`, `describe-table`, `list-databases`
- `analyze-schema`, `smart-query-builder`, `discover-joins`, `sample-data`
- `optimize-query`, `validate-query`, `analyze-data-lineage`, `discover-insights`
- `track-schema-changes`, `federated-query`
- `mcp-info`, `list-tools`

### Resources
- `tools://list` (registered tools snapshot)
- `profile://{profile}` (redacted profile metadata)

## Architectural Decisions

1. Stateless handler execution for scalability and clarity
2. Tool-based MCP contracts for interoperability
3. Read-only policy as first-class guardrail
4. Separation of concern between handler orchestration and feature-specific engines

## File Layout (High-Level)

- `cmd/server/main.go`
- `internal/config/*`
- `internal/db/*`
- `internal/log/*`
- `internal/mcp/*`
- `docs/*`
- `.kilocode/rules/memory-bank/*`

## Source of Truth

When architecture docs diverge from implementation, defer to:
1. `internal/mcp/server.go`
2. `docs/mcp-openapi.yaml`
