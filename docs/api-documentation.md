# Database MCP Server - API Documentation

## Overview

This document describes the current runtime MCP tool contracts for Database MCP Server.

Canonical sources:
1. `internal/mcp/server.go` (runtime registration)
2. `docs/mcp-openapi.yaml` (machine-readable schema)

## Protocol

- Protocol: MCP (JSON-RPC over stdio)
- Primary transport: stdio
- Optional transport: HTTP/SSE when `MCP_SSE_ADDR` is configured

## Request/Response Shape

Typical request:

```json
{
  "jsonrpc": "2.0",
  "method": "tool-name",
  "params": {"...": "..."},
  "id": "req_001"
}
```

Typical success response:

```json
{
  "jsonrpc": "2.0",
  "result": {"...": "..."},
  "id": "req_001"
}
```

Typical error response:

```json
{
  "jsonrpc": "2.0",
  "error": {
    "code": "ERROR_CODE",
    "message": "Human-readable message",
    "details": {"...": "..."},
    "suggestions": ["...", "..."]
  },
  "id": "req_001"
}
```

## Tool Catalog (Current)

### Profile and Connectivity

1. `configure-profile`
- Purpose: create, update, delete, or clone a database connection profile
- Key params: `profile_name`, `action` (`delete`|`clone`|`""`), `source_profile` (for clone), `db_type`, `database_name`, `readonly`, plus host/port/user/pass for non-sqlite

2. `list-profiles`
- Purpose: list configured profiles
- Key params: none

3. `mcp-info`
- Purpose: return provider version/author metadata
- Key params: none

4. `list-tools`
- Purpose: list runtime-registered tools and descriptions
- Key params: none

### Database Discovery and Execution

5. `list-databases`
- Purpose: list accessible databases/schemas for a profile
- Key params: `profile_name`

6. `list-tables`
- Purpose: list tables/views in a specific database
- Key params: `profile_name`, `database_name`

7. `describe-table`
- Purpose: return comprehensive table schema metadata
- Key params: `profile_name`, `database_name`, `table_name`

8. `sample-data`
- Purpose: fetch sample rows for schema/value understanding
- Key params: `profile_name`, `database_name`, `table_name`, optional `sample_size`

9. `execute-sql`
- Purpose: execute SQL statement/query
- Key params: `profile_name`, `database_name`, `sql`, optional `params`

10. `discover-joins`
- Purpose: detect joinable FK relationships and suggest joins
- Key params: `profile_name`, optional `tables`

### Intelligence and Analysis

11. `analyze-schema`
- Purpose: multi-level schema analysis with optional advanced profiling
- Key params: `profile_name`, `analysis_level` (`basic|detailed|comprehensive`), optional filters and `profiling`

12. `smart-query-builder`
- Purpose: generate SQL from natural-language intent
- Key params: `profile_name`, `intent`, optional `database_name`, `table_names`

13. `validate-query`
- Purpose: validate syntax/risky patterns without execution
- Key params: `profile_name`, `sql`, optional `database_name`, `params`

14. `optimize-query`
- Purpose: run explain-style optimization analysis
- Key params: `profile_name`, `database_name`, `sql`, optional `params`

15. `analyze-data-lineage`
- Purpose: trace upstream/downstream table dependencies
- Key params: `profile_name`, `table_name`, optional `database_name`, `scope`

16. `discover-insights`
- Purpose: discover KPI/trend/anomaly/distribution insights from table data
- Key params: `profile_name`, `table_name`, optional `columns`, `insight_types`, `max_results`

17. `track-schema-changes`
- Purpose: schema snapshot/history/migration/drift workflows
- Key params: `profile_name`, optional `operation`, `database_name`, snapshot and migration parameters

18. `federated-query`
- Purpose: read-only multi-profile query federation with join/aggregation
- Key params: either federated `sql` or `sub_queries`, optional `joins`, `aggregations`, pagination and concurrency

## Planned Tools (v1.4.0)

The following tools are planned for the Data Migration feature (Phase 4):

19. `start-migration-job`
- Purpose: Start an asynchronous cross-database migration job
- Key params: `source_profile`, `source_database`, `target_profile`, `target_database`, optional `source_table`, `include_tables`, `truncate_target`, `stop_on_error`
- Returns: `job_id` for status tracking

20. `check-migration-status`
- Purpose: Poll migration job progress and results
- Key params: `job_id`
- Returns: `status`, `progress_percent`, `tables_status`, `rows_migrated`, `rows_failed`, error file paths

21. `generate-schema-copy`
- Purpose: Generate schema translation SQL between database dialects (helper for LLM-guided migration)
- Key params: `source_profile`, `source_database`, `target_profile`, `target_database`, optional `source_table`, `include_tables`
- Returns: SQL files for tables, indexes, foreign keys, triggers, views; skipped items for unsupported objects

**Design Document**: `docs/data-migration-design.md`

## End-to-End Usage Patterns

### Safe SQL flow
1. `smart-query-builder`
2. `validate-query`
3. `optimize-query`
4. `execute-sql`

### Unknown database onboarding
1. `list-databases`
2. `list-tables`
3. `describe-table`
4. `sample-data`

### Schema governance
1. `track-schema-changes` (`track`)
2. `track-schema-changes` (`history`)
3. `track-schema-changes` (`detect_drift`)
4. `track-schema-changes` (`generate_migration`)

### Cross-database analytics
1. Ensure each source has a configured profile
2. Use `federated-query`
3. Review metadata for partial failures/errors

## Parameter and Schema Reference

For full request/response schema details, use:
- `docs/mcp-openapi.yaml`
- `docs/mcp-examples.md`

These files are the maintained detailed contract references.
