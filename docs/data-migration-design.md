# Data Migration Feature Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add cross-database data migration capability supporting both single-table and whole-database migrations via three new MCP tools (`start-migration-job`, `check-migration-status`, `generate-schema-copy`) with asynchronous background jobs, smart memory buffering, LLM-guided schema translation, and robust error handling.

**Architecture:** 
- Go-native ETL engine (no external dependencies like `pg_dump` or Python scripts)
- LLM handles schema translation via `generate-schema-copy` helper tool + `execute-sql`, NOT hardcoded type maps
- Background goroutine per migration job with streaming cursor, smart buffer (row count OR memory cap), and bulk parameterized inserts
- Automatic topological sort for FK dependency ordering
- Per-table checkpointing for resume-from-failure
- Error isolation: failing rows/tables written to `.tmp/migrations/{job_id}/` files, job continues

**Tech Stack:** Go 1.26.0, existing `internal/mcp` patterns, `database/sql` streaming, `encoding/json` for checkpoint/error files

---

## Design Decisions (from Brainstorming)

### 1. Migration Scope: Single-Table vs Whole-Database

**Optional `source_table` parameter:**
- If `source_table` is provided → migrate that single table only
- If `source_table` is omitted → migrate ALL tables in `source_database`
- Optional `include_tables` array to filter which tables to migrate

```json
// Single table migration
{
  "source_profile": "prod",
  "source_database": "app",
  "source_table": "users",
  "target_profile": "dev",
  "target_database": "app"
}

// Whole database migration
{
  "source_profile": "prod",
  "source_database": "app",
  "target_profile": "dev",
  "target_database": "app"
}

// Selective tables migration
{
  "source_profile": "prod",
  "source_database": "app",
  "include_tables": ["users", "orders", "products"],
  "target_profile": "dev",
  "target_database": "app"
}
```

### 2. Job Model: Single Job, Sequential Tables

**Single job per migration:**
- One `job_id` for entire operation (not child jobs per table)
- Tables processed sequentially in topological order (FK dependencies respected)
- Progress = overall percentage + per-table breakdown
- Status shows current table being migrated

```json
// check-migration-status response for whole-database migration
{
  "job_id": "mig_123",
  "status": "running",
  "progress_percent": 35,
  "current_table": "orders",
  "tables_completed": 5,
  "tables_total": 15,
  "tables_status": [
    {"table": "users", "status": "completed", "rows_migrated": 10000},
    {"table": "addresses", "status": "completed", "rows_migrated": 5000},
    {"table": "products", "status": "completed", "rows_migrated": 2000},
    {"table": "orders", "status": "in_progress", "rows_migrated": 3000}
  ],
  "rows_migrated": 20000,
  "rows_failed": 0
}
```

### 3. Progress Visibility: Dual Output

**JSON logs to stderr:**
- Real-time progress via existing `log.JSONLog()` pattern
- Viewable by operators watching terminal/logs
```json
{"level":"info","message":"Migration progress","job_id":"mig_123","table":"orders","rows":5000,"percent":25}
```

**Structured status via `check-migration-status`:**
- LLM polls for detailed progress
- Per-table breakdown for whole-database migrations
- No new protocols needed

### 4. Error Handling: Configurable per Job

**`stop_on_error` parameter:**
- `stop_on_error: false` (default) → continue to next table, mark as `completed_with_errors`
- `stop_on_error: true` → fail entire job on first error

**Error reporting for multi-table:**
```json
{
  "status": "completed_with_errors",
  "tables_completed": 9,
  "tables_total": 10,
  "failed_tables": [
    {
      "table": "orders",
      "error": "constraint violation",
      "rows_failed": 5,
      "error_file": ".tmp/migrations/mig_123/orders_failed.jsonl"
    }
  ]
}
```

### 5. Schema Creation: LLM-Driven with Helper Tool

**`generate-schema-copy` tool:**
- Produces target-dialect-aware CREATE statements
- LLM reviews/modifies before executing
- Full schema support: tables, indexes, FKs, triggers, views

**Output includes:**
- Dialect-specific type mappings (MySQL → Postgres: `AUTO_INCREMENT` → `SERIAL`)
- Index definitions
- Foreign key constraints
- Triggers (with warnings for untranslatable syntax)
- Views (with warnings for dialect-specific functions)
- Skipped items (stored procedures, functions) exported to files

### 6. Schema Copy Output: Separate Files per Type

**Directory structure:**
```
.tmp/schema-copy/{timestamp}/
├── tables.sql           # CREATE TABLE statements
├── indexes.sql          # CREATE INDEX statements  
├── foreign_keys.sql     # ALTER TABLE ADD CONSTRAINT statements
├── triggers.sql         # CREATE TRIGGER (converted where possible)
├── views.sql            # CREATE VIEW statements
└── skipped/
    ├── stored_procedures.sql   # Original source + comments
    ├── functions.sql           # Original source + comments
    └── untranslatable_triggers.sql
```

**Tool params:**
```json
{
  "profile_name": "source",
  "database_name": "app",
  "target_db_type": "postgres",
  "output_dir": ".tmp/schema-copy/migration_20260221"
}
```

### 7. Table Ordering: Automatic Topological Sort

**Dependency resolution:**
- Read FK relationships from `information_schema`
- Build dependency graph
- Process tables in order: parents first, children last
- Circular dependencies handled via deferred constraints

**Example order:**
```
users → addresses → categories → products → orders → order_items
```

### 8. Circular FK Handling: Deferred Constraints

**Strategy:**
1. Detect circular FK dependencies
2. Defer FK constraints until end of transaction
3. Insert all rows first
4. Validate FKs at commit
5. If orphans exist → write to error file, mark `completed_with_errors`

**Per-database approach:**
- PostgreSQL: `SET CONSTRAINTS ALL DEFERRED`
- MySQL: `SET FOREIGN_KEY_CHECKS=0` during migration
- SQLite: `PRAGMA foreign_keys=OFF` during migration

**FK violation error file:**
```json
// .tmp/migrations/{job_id}/fk_violations.jsonl
{"table": "orders", "row_index": 42, "column": "customer_id", "value": 999, "error": "references non-existent customer"}
```

### 9. Resume from Failure: Per-Table Checkpointing

**Checkpoint file:**
```
.tmp/migrations/{job_id}/checkpoint.json
```
```json
{
  "job_id": "mig_123",
  "source_profile": "prod",
  "source_database": "app",
  "target_profile": "dev",
  "target_database": "app",
  "completed_tables": ["users", "addresses", "products"],
  "current_table": "orders",
  "current_table_status": "in_progress",
  "tables_remaining": ["order_items", "logs"],
  "started_at": "2026-02-21T10:00:00Z",
  "updated_at": "2026-02-21T10:15:00Z"
}
```

**Resume behavior:**
- Only truncate the failed table (not completed tables)
- Skip tables already marked as completed
- Continue from where it stopped

**Resume call:**
```json
{
  "resume_job_id": "mig_123"
}
```

### 10. Default Settings

| Parameter | Default | Description |
|-----------|---------|-------------|
| `truncate_target` | `false` | Safer default — append mode |
| `stop_on_error` | `false` | Continue on errors, report at end |
| `continue_on_row_error` | `true` | Write failed rows to error file |
| `batch_size` | `50000` | Rows per batch insert |
| `memory_limit_mb` | `50` | Buffer memory cap |

### 11. No Cancellation for v1

**Rationale:**
- Go can stop goroutine, but DB may have in-flight queries
- Partial batch insert = unclear state
- User would need to truncate and restart anyway

**Workaround:**
- Kill process (Ctrl+C)
- Clean up by truncating failed table
- Resume from checkpoint

### 12. One-Way Migration Only

**v1 scope:**
- Source → Target (one direction)
- No bidirectional sync
- LLM can handle reverse migration by swapping params

---

## Migration Workflow

### Single-Table Workflow (5 Steps)

| Step | Tool | Purpose |
|------|------|---------|
| 1 | `describe-table` (source) | Get source schema |
| 2 | `sample-data` (source) | Inspect real data patterns |
| 3 | LLM brain | Translate schema |
| 4 | `execute-sql` (target) | Create target table |
| 5 | `start-migration-job` | Start background migration |
| 6 | `check-migration-status` | Poll progress |

### Whole-Database Workflow (6 Steps)

| Step | Tool | Purpose |
|------|------|---------|
| 1 | `list-tables` (source) | Get all table names |
| 2 | `generate-schema-copy` | Generate dialect-aware CREATE statements |
| 3 | LLM brain | Review/modify generated schema |
| 4 | `execute-sql` (target) | Execute CREATE statements (tables, indexes, etc.) |
| 5 | `start-migration-job` | Start background migration (omit `source_table`) |
| 6 | `check-migration-status` | Poll progress (per-table breakdown) |

---

## Files Overview

| File | Role |
|------|------|
| `internal/mcp/migration.go` | **CREATE** — Job manager, smart buffer, background worker, checkpoint, topological sort |
| `internal/mcp/migration_test.go` | **CREATE** — TDD test suite for migration engine |
| `internal/mcp/schema_copy.go` | **CREATE** — Schema copy generator with dialect translation |
| `internal/mcp/schema_copy_test.go` | **CREATE** — Tests for schema copy generator |
| `internal/mcp/server.go` | **MODIFY** — Register 3 new tools, add param structs, add handlers |
| `internal/mcp/server_test.go` | **MODIFY** — Integration tests for new tool handlers |
| `internal/mcp/tool_help.go` | **MODIFY** — Help entries for new tools |
| `internal/mcp/errors.go` | **MODIFY** — Add `MIGRATION_*` error codes |
| `README.md` | **MODIFY** — Add migration tools to table |
| `docs/mcp-openapi.yaml` | **MODIFY** — Add tool schemas |
| `CHANGELOG.md` | **MODIFY** — Add v1.4.0 section |

---

## API Design

### `start-migration-job`

**Params:**
```go
type StartMigrationJobParams struct {
    // Scope (mutually exclusive patterns)
    SourceTable      string   `json:"source_table,omitempty"`       // Single table mode
    IncludeTables    []string `json:"include_tables,omitempty"`     // Filter tables (whole-DB mode)
    
    // Source (required)
    SourceProfile    string   `json:"source_profile"`
    SourceDatabase   string   `json:"source_database"`
    
    // Target (required)
    TargetProfile    string   `json:"target_profile"`
    TargetDatabase   string   `json:"target_database"`
    
    // Resume
    ResumeJobID      string   `json:"resume_job_id,omitempty"`      // Resume from checkpoint
    
    // Options
    BatchSize        int      `json:"batch_size,omitempty"`         // Default: 50000
    MemoryLimitMB    int      `json:"memory_limit_mb,omitempty"`    // Default: 50
    TruncateTarget   bool     `json:"truncate_target,omitempty"`    // Default: false
    StopOnError      bool     `json:"stop_on_error,omitempty"`      // Default: false
}
```

**Result:**
```go
type StartMigrationJobResult struct {
    JobID        string   `json:"job_id"`
    Status       string   `json:"status"`              // "started" or "resumed"
    Mode         string   `json:"mode"`                // "single_table" or "whole_database"
    TablesCount  int      `json:"tables_count,omitempty"` // For whole-DB mode
    Message      string   `json:"message"`
    Checkpoint   string   `json:"checkpoint,omitempty"` // Path to checkpoint file
}
```

### `check-migration-status`

**Params:**
```go
type CheckMigrationStatusParams struct {
    JobID string `json:"job_id"`
}
```

**Result:**
```go
type CheckMigrationStatusResult struct {
    JobID           string        `json:"job_id"`
    Status          string        `json:"status"`           // "running", "completed", "completed_with_errors", "failed"
    Mode            string        `json:"mode"`             // "single_table" or "whole_database"
    ProgressPercent int           `json:"progress_percent"` // 0-100
    CurrentTable    string        `json:"current_table,omitempty"`
    
    // Single-table mode
    RowsMigrated    int64         `json:"rows_migrated,omitempty"`
    RowsFailed      int64         `json:"rows_failed,omitempty"`
    TotalRows       int64         `json:"total_rows,omitempty"`
    ErrorFile       string        `json:"error_file,omitempty"`
    
    // Whole-database mode
    TablesCompleted int           `json:"tables_completed,omitempty"`
    TablesTotal     int           `json:"tables_total,omitempty"`
    TablesStatus    []TableStatus `json:"tables_status,omitempty"`
    FailedTables    []FailedTable `json:"failed_tables,omitempty"`
    
    // Timestamps
    StartedAt       string        `json:"started_at"`
    CompletedAt     string        `json:"completed_at,omitempty"`
    ErrorMessage    string        `json:"error_message,omitempty"`
}

type TableStatus struct {
    Table         string `json:"table"`
    Status        string `json:"status"`        // "pending", "in_progress", "completed", "failed", "skipped"
    RowsMigrated  int64  `json:"rows_migrated"`
    RowsFailed    int64  `json:"rows_failed"`
    TotalRows     int64  `json:"total_rows"`
}

type FailedTable struct {
    Table       string `json:"table"`
    Error       string `json:"error"`
    RowsFailed  int64  `json:"rows_failed"`
    ErrorFile   string `json:"error_file"`
}
```

### `generate-schema-copy`

**Params:**
```go
type GenerateSchemaCopyParams struct {
    ProfileName    string   `json:"profile_name"`
    DatabaseName   string   `json:"database_name"`
    TargetDBType   string   `json:"target_db_type"`        // "mysql", "postgres", "sqlite"
    OutputDir      string   `json:"output_dir,omitempty"`  // Default: .tmp/schema-copy/{timestamp}
    IncludeTables  []string `json:"include_tables,omitempty"`
    ExcludeTables  []string `json:"exclude_tables,omitempty"`
}
```

**Result:**
```go
type GenerateSchemaCopyResult struct {
    OutputDir       string          `json:"output_dir"`
    TablesGenerated int             `json:"tables_generated"`
    IndexesGenerated int            `json:"indexes_generated"`
    FKsGenerated    int             `json:"foreign_keys_generated"`
    TriggersGenerated int           `json:"triggers_generated"`
    ViewsGenerated  int             `json:"views_generated"`
    Warnings        []SchemaWarning `json:"warnings"`
    SkippedItems    []SkippedItem   `json:"skipped_items"`
    Summary         string          `json:"summary"`
}

type SchemaWarning struct {
    Type    string `json:"type"`    // "trigger", "view", "type"
    Name    string `json:"name"`
    Message string `json:"message"`
}

type SkippedItem struct {
    Type     string `json:"type"`     // "stored_procedure", "function"
    Name     string `json:"name"`
    Reason   string `json:"reason"`
    File     string `json:"file"`     // Path to skipped/{type}.sql
}
```

---

## Error Codes (add to errors.go)

```go
const (
    // Migration errors
    ErrorCodeMigrationNotFound       ErrorCode = "MIGRATION_NOT_FOUND"
    ErrorCodeMigrationAlreadyExists  ErrorCode = "MIGRATION_ALREADY_EXISTS"
    ErrorCodeMigrationFailed         ErrorCode = "MIGRATION_FAILED"
    ErrorCodeSourceTableNotFound     ErrorCode = "SOURCE_TABLE_NOT_FOUND"
    ErrorCodeTargetTableNotFound     ErrorCode = "TARGET_TABLE_NOT_FOUND"
    ErrorCodeColumnMismatch          ErrorCode = "COLUMN_MISMATCH"
    ErrorCodeFKViolation             ErrorCode = "FK_VIOLATION"
    ErrorCodeCheckpointNotFound      ErrorCode = "CHECKPOINT_NOT_FOUND"
    ErrorCodeSchemaCopyFailed        ErrorCode = "SCHEMA_COPY_FAILED"
)
```

---

## Output Files Structure

```
.tmp/
├── migrations/
│   └── {job_id}/
│       ├── checkpoint.json           # Resume state
│       ├── {table}_failed.jsonl      # Per-table failed rows
│       └── fk_violations.jsonl       # FK constraint violations
│
└── schema-copy/
    └── {timestamp}/
        ├── tables.sql
        ├── indexes.sql
        ├── foreign_keys.sql
        ├── triggers.sql
        ├── views.sql
        └── skipped/
            ├── stored_procedures.sql
            ├── functions.sql
            └── untranslatable_triggers.sql
```

---

## Implementation Tasks

### Task 1: Add Error Codes and Core Types
- Add migration error codes to `errors.go`
- Create `migration.go` with param/result types
- Create job state types and job manager skeleton

### Task 2: Implement Job Manager
- `CreateJob()`, `GetJob()`, `UpdateJob()` methods
- Checkpoint read/write functions
- Unit tests for job manager

### Task 3: Implement Smart Buffer
- Row buffering with row count and memory caps
- Row scanner from `sql.Rows`
- Unit tests for buffer

### Task 4: Implement Bulk Insert Builder
- Dialect-aware placeholder generation
- Parameterized query builder
- Error file writer (`.jsonl`)
- Unit tests

### Task 5: Implement Topological Sort
- FK dependency graph builder
- Cycle detection
- Table ordering algorithm
- Unit tests

### Task 6: Implement Schema Copy Generator
- `information_schema` queries for tables/columns/indexes/FKs
- Dialect translation maps
- Trigger/view parsing
- File output generator
- Unit tests

### Task 7: Implement Background Worker
- Single-table migration worker
- Whole-database migration worker (with table iteration)
- Progress tracking and checkpoint updates
- FK deferral handling

### Task 8: Add MCP Tool Handlers
- `handleStartMigrationJob`
- `handleCheckMigrationStatus`
- `handleGenerateSchemaCopy`
- Register tools in `registerAllTools()`

### Task 9: Add Tool Help Entries
- Help entries for all 3 tools
- Examples and common errors

### Task 10: Integration Tests
- SQLite-to-SQLite single-table migration
- SQLite-to-SQLite whole-database migration
- Resume from checkpoint
- FK violation handling
- Schema copy generation

### Task 11: Full Regression + Quality Gates
- `go test ./...`
- `go vet ./...`
- `gofmt -w .`
- `golangci-lint run ./...`

### Task 12: Update Documentation
- README.md (tool count → 21)
- CHANGELOG.md (v1.4.0 section)
- Bump `MCPVersion` to `v1.4.0`
- Sync via `scripts/sync-version-from-server.sh`

---

## Verification Checklist

- [ ] `go test ./...` passes
- [ ] `go vet ./...` passes
- [ ] `gofmt -l .` returns nothing
- [ ] `golangci-lint run ./...` returns 0 issues
- [ ] `go build ./cmd/server/main.go` succeeds
- [ ] Single-table migration works (SQLite-to-SQLite)
- [ ] Whole-database migration works (SQLite-to-SQLite)
- [ ] Resume from checkpoint works
- [ ] Schema copy generates correct files
- [ ] Tool count in README is 21
- [ ] CHANGELOG has v1.4.0 section
- [ ] `MCPVersion` is `v1.4.0`

---

## Summary

This plan implements a complete data migration feature with:

1. **Three new MCP tools** (`start-migration-job`, `check-migration-status`, `generate-schema-copy`) with Gemini-safe schemas
2. **Dual mode support** (single-table and whole-database migration)
3. **LLM-driven schema translation** with helper tool for dialect-aware generation
4. **Background job engine** using Go goroutines with smart memory buffer
5. **Automatic table ordering** via topological sort for FK dependencies
6. **Resume from failure** with per-table checkpointing
7. **Comprehensive error handling** (per-row errors, FK violations, skipped items)
8. **Full test coverage** including integration tests
9. **Documentation updates** for v1.4.0 release
