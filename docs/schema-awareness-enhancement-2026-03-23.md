# Schema Awareness Enhancement Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement multi-schema support for PostgreSQL with automatic schema detection, schema discovery tools, and improved error handling to resolve user friction when working with non-public schemas.

**Architecture:** 
- Add optional `schema` parameter to table-related tools (describe-table, sample-data, list-tables, analyze-schema)
- Implement auto-schema detection using PostgreSQL's `search_path` and `current_schema()` function
- Add three new schema discovery tools: list-schemas, get-search-path, set-search-path
- Enhance error messages with schema context and helpful suggestions
- Maintain backward compatibility with all existing tools

**Tech Stack:**
- Go 1.26 with go1.26.0 toolchain
- Official Go MCP SDK
- PostgreSQL information_schema for schema discovery
- Existing connection pooling and MCP infrastructure

---

## Phase 1: Schema Foundation (v1.3.1) - TDD Implementation

### Task 1: Update list-tables to show schema information

**Files:**
- Create: (none)
- Modify: `internal/mcp/server.go:67` (query definition)
- Modify: `internal/mcp/server.go:3370-3420` (handleListTables function)
- Test: `internal/mcp/server_test.go`

**Step 1: Write the failing test**

```go
func TestListTablesShowsSchemaInfo(t *testing.T) {
    // Setup test database with multiple schemas
    // Call list-tables tool
    // Verify response includes schema names alongside table names
    // Expect: [{"schema":"bitnami_redmine","table":"users"},{"schema":"public","table":"spatial_ref_sys"}]
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/mcp -run TestListTablesShowsSchemaInfo -v`
Expected: FAIL with "TestListTablesShowsSchemaInfo" not defined or assertion error

**Step 3: Write minimal implementation**

Modify the PostgreSQL query in handleListTables to:
```sql
SELECT table_schema, table_name FROM information_schema.tables 
WHERE table_schema NOT IN ('pg_catalog', 'information_schema')
ORDER BY table_schema, table_name
```

Update the response structure to include schema information.

**Step 4: Run test to verify it passes**

Run: `go test ./internal/mcp -run TestListTablesShowsSchemaInfo -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/mcp/server.go internal/mcp/server_test.go
git commit -m "feat(list-tables): show schema information for multi-schema support"
```

### Task 2: Implement auto-schema detection for describe-table

**Files:**
- Modify: `internal/mcp/server.go:3880-3895` (analyzeSchemaColumnQuery function)
- Modify: `internal/mcp/server.go:3215-3220` (describeSQLPostgresTableColumns function call site)
- Test: `internal/mcp/server_test.go`

**Step 1: Write the failing test**

```go
func TestDescribeTableAutoDetectsSchema(t *testing.T) {
    // Setup: Create test table in non-public schema
    // Set search_path to test schema
    // Call describe-table without schema parameter
    // Verify it finds and describes the table correctly
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/mcp -run TestDescribeTableAutoDetectsSchema -v`
Expected: FAIL

**Step 3: Write minimal implementation**

Replace hardcoded `'public'` with `current_schema()` in PostgreSQL queries:
- In analyzeSchemaColumnQuery function
- In describeSQLPostgresTableColumns function

**Step 4: Run test to verify it passes**

Run: `go test ./internal/mcp -run TestDescribeTableAutoDetectsSchema -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/mcp/server.go internal/mcp/server_test.go
git commit -m "feat(describe-table): add auto-schema detection using current_schema()"
```

### Task 3: Add schema parameter to describe-table tool

**Files:**
- Modify: `internal/mcp/server.go` (tool definition for describe-table)
- Modify: `internal/mcp/server.go` (handleDescribeTable function)
- Test: `internal/mcp/server_test.go`

**Step 1: Write the failing test**

```go
func TestDescribeTableWithSchemaParameter(t *testing.T) {
    // Setup: Create test table in specific schema
    // Call describe-table with schema parameter
    // Verify it uses the provided schema
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/mcp -run TestDescribeTableWithSchemaParameter -v`
Expected: FAIL

**Step 3: Write minimal implementation**

1. Add optional `schema` string parameter to describe-table tool definition
2. Modify handleDescribeTable to use provided schema when available
3. Fall back to auto-detection when schema is empty

**Step 4: Run test to verify it passes**

Run: `go test ./internal/mcp -run TestDescribeTableWithSchemaParameter -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/mcp/server.go internal/mcp/server_test.go
git commit -m "feat(describe-table): add optional schema parameter"
```

### Task 4: Implement list-schemas discovery tool

**Files:**
- Create: (none) - add to existing server.go
- Modify: `internal/mcp/server.go` (add list-schemas tool definition and handler)
- Test: `internal/mcp/server_test.go`

**Step 1: Write the failing test**

```go
func TestListSchemasTool(t *testing.T) {
    // Setup: Create test database with multiple schemas
    // Call list-schemas tool
    // Verify response contains schema names
    // Expect: {"schemas":["public","bitnami_redmine"],"default_schema":"bitnami_redmine"}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/mcp -run TestListSchemasTool -v`
Expected: FAIL

**Step 3: Write minimal implementation**

1. Add list-schemas tool definition to toolsRegistry
2. Implement handleListSchemas function:
   ```sql
   SELECT schema_name FROM information_schema.schemata 
   WHERE schema_name NOT IN ('pg_catalog', 'information_schema')
   ORDER BY schema_name
   ```
3. Get current schema via `SELECT current_schema()`

**Step 4: Run test to verify it passes**

Run: `go test ./internal/mcp -run TestListSchemasTool -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/mcp/server.go internal/mcp/server_test.go
git commit -m "feat(list-schemas): add schema discovery tool"
```

### Task 5: Implement get-search-path and set-search-path tools

**Files:**
- Modify: `internal/mcp/server.go` (add two tool definitions and handlers)
- Test: `internal/mcp/server_test.go`

**Step 1: Write the failing test**

```go
func TestGetAndSetSearchPathTools(t *testing.T) {
    // Setup: 
    // Call set-search-path to set schema
    // Call get-search-path to verify it was set
    // Verify response contains the set schema
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/mcp -run TestGetAndSetSearchPathTools -v`
Expected: FAIL

**Step 3: Write minimal implementation**

1. Add get-search-path and set-search-path tool definitions
2. Implement handlers:
   - get-search-path: `SHOW search_path; SELECT current_schema()`
   - set-search-path: Execute `SET search_path TO $1` (with prepend option)

**Step 4: Run test to verify it passes**

Run: `go test ./internal/mcp -run TestGetAndSetSearchPathTools -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/mcp/server.go internal/mcp/server_test.go
git commit -m "feat(search-path): add get-search-path and set-search-path tools"
```

### Task 6: Enhance error messages with schema context

**Files:**
- Modify: `internal/mcp/server.go` (error handling in table-related functions)
- Modify: `internal/mcp/server.go` (create enhanced error response structure)
- Test: `internal/mcp/server_test.go`

**Step 1: Write the failing test**

```go
func TestDescribeTableErrorIncludesSchemaSuggestions(t *string) {
    // Setup: Try to describe non-existent table
    // Call describe-table
    // Verify error includes:
    // - Clear message about missing table
    // - List of available schemas
    // - Suggestions for similar table names
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/mcp -run TestDescribeTableErrorIncludesSchemaSuggestions -v`
Expected: FAIL

**Step 3: Write minimal implementation**

1. Create enhanced error response structure with:
   - Code, Message, Details, Suggestions, Context fields
   - Context containing available schemas, tables, etc.
2. Modify table-related functions to:
   - Catch "table not found" errors
   - Query information_schema for available schemas/tables
   - Generate helpful suggestions
   - Return enhanced error response

**Step 4: Run test to verify it passes**

Run: `go test ./internal/mcp -run TestDescribeTableErrorIncludesSchemaSuggestions -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/mcp/server.go internal/mcp/server_test.go
git commit -m "feat(error-handling): enhance table-related errors with schema context"
```

### Task 7: Enhance smart-query-builder for multi-schema support

**Files:**
- Modify: `internal/mcp/server.go` (smart-query-builder tool definition)
- Modify: `internal/mcp/server.go` (handleSmartQueryBuilder function)
- Test: `internal/mcp/server_test.go`

**Step 1: Write the failing test**

```go
func TestSmartQueryBuilderSearchesAllSchemas(t *testing.T) {
    // Setup: Create tables with matching names in different schemas
    // Call smart-query-builder with intent matching tables
    // Verify it searches across all schemas and ranks by relevance
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/mcp -run TestSmartQueryBuilderSearchesAllSchemas -v`
Expected: FAIL

**Step 3: Write minimal implementation**

1. Add optional `schema` parameter to smart-query-builder tool
2. Modify handler to:
   - When schema provided: search only that schema
   - When schema empty: search `information_schema.tables` across all user schemas
   - Score tables by relevance to intent keywords
   - Return best match with schema qualification

**Step 4: Run test to verify it passes**

Run: `go test ./internal/mcp -run TestSmartQueryBuilderSearchesAllSchemas -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/mcp/server.go internal/mcp/server_test.go
git commit -m "feat(smart-query-builder): enhance for multi-schema support"
```

### Task 8: Add schema parameter to sample-data and analyze-schema tools

**Files:**
- Modify: `internal/mcp/server.go` (tool definitions and handlers)
- Test: `internal/mcp/server_test.go`

**Step 1: Write the failing test**

```go
func TestSampleDataAndAnalyzeSchemaWithSchemaParameter(t *testing.T) {
    // Setup: Create test data in specific schema
    // Call sample-data and analyze-schema with schema parameter
    // Verify they use the provided schema
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/mcp -run TestSampleDataAndAnalyzeSchemaWithSchemaParameter -v`
Expected: FAIL

**Step 3: Write minimal implementation**

1. Add optional `schema` parameter to both tools
2. Modify handlers to use provided schema when available
3. Fall back to auto-detection when schema is empty

**Step 4: Run test to verify it passes**

Run: `go test ./internal/mcp -run TestSampleDataAndAnalyzeSchemaWithSchemaParameter -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/mcp/server.go internal/mcp/server_test.go
git commit -m "feat(sample-data,analyze-schema): add optional schema parameter"
```

### Task 9: Run full test suite and verify no regressions

**Files:**
- Test: `./...`

**Step 1: Write the failing test**

(None - verification task)

**Step 2: Run test to verify it fails**

Run: `go test ./... -v`
Expected: May fail if we broke something

**Step 3: Write minimal implementation**

Fix any test failures from previous changes

**Step 4: Run test to verify it passes**

Run: `go test ./... -v`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add ./...
git commit -m "test: verify all tests pass after schema awareness changes"
```

### Task 10: Update documentation

**Files:**
- Modify: `docs/api-documentation.md` (add new tools and updated parameters)
- Modify: `docs/README.md` (update examples and features)
- Modify: `docs/prd.md` (already updated)
- Modify: `docs/implementation-status.md` (already updated)
- Modify: `docs/roadmap.md` (already updated)

**Step 1: Write the failing test**

(None - documentation task)

**Step 2: Run test to verify it fails**

Run: `echo "Documentation verification"`
Expected: Documentation doesn't match implementation

**Step 3: Write minimal implementation**

Update all documentation to reflect:
- New tools: list-schemas, get-search-path, set-search-path
- Updated tools: describe-table, sample-data, list-tables, analyze-schema, smart-query-builder (with schema parameter)
- Enhanced error messages
- Auto-schema detection behavior

**Step 4: Run test to verify it passes**

Run: `echo "Documentation verified"`
Expected: Documentation matches implementation

**Step 5: Commit**

```bash
git add docs/
git commit -m "docs: update for schema awareness enhancement"
```

---

## Execution Instructions

To implement this plan using the executing-plans skill in a separate session:

1. Open a new terminal session in the same workspace
2. Run: `skill superpowers executing-plans`
3. Load this plan: `read /media/eddy/hdd/Project/Database_MCP_Server/docs/plans/2026-03-23-schema-awareness-enhancement.md`
4. Follow the task-by-task execution with review checkpoints

**Total Estimated Effort:** 3-5 days for a single developer working full-time

**Key Benefits:**
- Each task is small (2-5 minutes) and verifiable
- TDD ensures quality and prevents regressions
- Frequent commits enable easy rollback if needed
- Clear separation of concerns between tasks