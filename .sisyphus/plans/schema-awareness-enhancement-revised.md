# Schema Awareness Enhancement Implementation Plan (Revised)

> **For Claude/Gemini**: REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement multi-schema support for PostgreSQL using schema-qualified queries (no search_path manipulation), with automatic schema detection, schema discovery tools, and improved error handling.

**Architecture Decision**: 
- ✅ Use schema-qualified queries (`schema.table`) instead of `SET search_path`
- ✅ Schema parameter explicit in all table-related tools
- ✅ Auto-detection falls back gracefully: current_schema() → first accessible schema → 'public'
- ✅ No session state changes - safe for connection pooling

**Tech Stack:**
- Go 1.26 with go1.26.0 toolchain
- Official Go MCP SDK
- PostgreSQL information_schema for schema discovery
- pq.QuoteIdentifier for safe schema quoting

---

## Phase 1: Foundation & Utilities (v1.3.1)

### Task 1: Add schema utility functions

**Files:**
- Create: `internal/mcp/schema_utils.go`
- Test: `internal/mcp/schema_utils_test.go`

**Step 1: Write the failing test**

```go
func TestQuoteSchemaName(t *testing.T) {
    tests := []struct {
        input    string
        expected string
    }{
        {"public", `"public"`},
        {"bitnami_redmine", `"bitnami_redmine"`},
        {"MySchema", `"MySchema"`}, // Case preserved when quoted
        {"schema\"name", `"schema""name"`}, // Special chars escaped
    }
    for _, tt := range tests {
        got := QuoteSchemaName(tt.input)
        if got != tt.expected {
            t.Errorf("QuoteSchemaName(%q) = %q, want %q", tt.input, got, tt.expected)
        }
    }
}

func TestGetDefaultSchema(t *testing.T) {
    // Mock or use test database
    // Test: current_schema() returns value → use it
    // Test: current_schema() returns NULL → query first schema
    // Test: No schemas → fallback to 'public'
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/mcp -run TestQuoteSchemaName -v`
Expected: FAIL (function doesn't exist)

**Step 3: Write minimal implementation**

```go
package mcp

import (
    "database/sql"
    "github.com/lib/pq"
)

// QuoteSchemaName safely quotes a schema name for use in SQL queries.
// Uses pq.QuoteIdentifier to handle special characters and case sensitivity.
func QuoteSchemaName(schema string) string {
    return pq.QuoteIdentifier(schema)
}

// GetDefaultSchema determines the default schema to use when none is specified.
// Priority: current_schema() → first accessible schema → 'public'
func GetDefaultSchema(ctx context.Context, conn *sql.Conn) (string, error) {
    // Try current_schema() first
    var schema sql.NullString
    err := conn.QueryRowContext(ctx, "SELECT current_schema()").Scan(&schema)
    if err != nil {
        return "", fmt.Errorf("failed to get current_schema: %w", err)
    }

    if schema.Valid && schema.String != "" && schema.String != "pg_catalog" {
        return schema.String, nil
    }

    // Fallback: query first accessible schema
    query := `
        SELECT schema_name 
        FROM information_schema.schemata 
        WHERE schema_name NOT IN ('pg_catalog', 'information_schema')
          AND schema_name NOT LIKE 'pg_%'
        ORDER BY schema_name
        LIMIT 1`
    
    var fallbackSchema string
    err = conn.QueryRowContext(ctx, query).Scan(&fallbackSchema)
    if err != nil {
        return "public", nil // Final fallback
    }
    
    return fallbackSchema, nil
}

// ResolveSchema resolves the effective schema name.
// If schema is provided, returns it (quoted).
// If schema is empty, uses GetDefaultSchema.
func ResolveSchema(ctx context.Context, conn *sql.Conn, schema string) (string, error) {
    if schema != "" {
        return QuoteSchemaName(schema), nil
    }
    defaultSchema, err := GetDefaultSchema(ctx, conn)
    if err != nil {
        return "", err
    }
    return QuoteSchemaName(defaultSchema), nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/mcp -run "TestQuoteSchemaName|TestGetDefaultSchema" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/mcp/schema_utils.go internal/mcp/schema_utils_test.go
git commit -m "feat(utils): add schema quoting and default schema detection utilities"
```

---

### Task 2: Update list-tables to show schema information

**Files:**
- Modify: `internal/mcp/server.go:52` (query definition)
- Modify: `internal/mcp/server.go:2960-2991` (queryTableNames function)
- Modify: `internal/mcp/server.go:1107-1112` (ListTablesParams/ListTablesResult)
- Test: `internal/mcp/server_test.go`

**Step 1: Write the failing test**

```go
func TestListTablesShowsSchemaInfo(t *testing.T) {
    // Setup: Create tables in multiple schemas
    // Call list-tables tool
    // Verify response includes schema names alongside table names
    // Expected format: [{"schema":"public","table":"users"},{"schema":"bitnami_redmine","table":"issues"}]
}

func TestListTablesFiltersSystemSchemas(t *testing.T) {
    // Verify pg_catalog and information_schema are not included
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/mcp -run "TestListTables" -v`
Expected: FAIL

**Step 3: Write minimal implementation**

1. Update query constant to include schema:
```go
const queryPostgresAllSchemasTables = `
    SELECT table_schema, table_name 
    FROM information_schema.tables 
    WHERE table_schema NOT IN ('pg_catalog', 'information_schema')
      AND table_schema NOT LIKE 'pg_%'
    ORDER BY table_schema, table_name`
```

2. Update result struct:
```go
type ListTablesResult struct {
    Tables []TableInfo `json:"tables"`
}

type TableInfo struct {
    Schema string `json:"schema"`
    Name   string `json:"table"`
}
```

3. Update queryTableNames to return schema+name

**Step 4: Run test to verify it passes**

Run: `go test ./internal/mcp -run "TestListTables" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/mcp/server.go internal/mcp/server_test.go
git commit -m "feat(list-tables): include schema information in response"
```

---

### Task 3: Add schema parameter to describe-table tool

**Files:**
- Modify: `internal/mcp/server.go` (DescribeTableParams struct)
- Modify: `internal/mcp/server.go` (handleDescribeTable function)
- Modify: `internal/mcp/server.go:67` (query constant)
- Test: `internal/mcp/server_test.go`

**Step 1: Write the failing test**

```go
func TestDescribeTableWithSchemaParameter(t *testing.T) {
    // Setup: Create test table in specific schema (e.g., bitnami_redmine.users)
    // Call describe-table with schema parameter
    // Verify it uses the provided schema correctly
}

func TestDescribeTableAutoDetectsSchema(t *testing.T) {
    // Setup: Set search_path to test schema
    // Call describe-table without schema parameter
    // Verify it falls back to detected schema (not hardcoded 'public')
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/mcp -run "TestDescribeTable" -v`
Expected: FAIL

**Step 3: Write minimal implementation**

1. Update DescribeTableParams:
```go
type DescribeTableParams struct {
    ProfileName  string `json:"profile_name"`
    DatabaseName string `json:"database_name"`
    TableName    string `json:"table_name"`
    Schema       string `json:"schema,omitempty"` // Optional schema parameter
}
```

2. Update query to use schema parameter:
```go
func (s *MCPServer) queryDescribeTableColumns(...) ([]ColumnInfo, *mcp.CallToolResult, error) {
    schema, err := ResolveSchema(ctx, conn, p.Schema)
    if err != nil {
        return nil, nil, err
    }
    
    query := fmt.Sprintf(`
        SELECT column_name, data_type, is_nullable, column_default, ...
        FROM information_schema.columns 
        WHERE table_schema = %s AND table_name = $1
        ORDER BY ordinal_position`,
        schema) // Use safe quoting
    
    // Execute query...
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/mcp -run "TestDescribeTable" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/mcp/server.go internal/mcp/server_test.go
git commit -m "feat(describe-table): add optional schema parameter with auto-detection"
```

---

### Task 4: Implement list-schemas discovery tool

**Files:**
- Modify: `internal/mcp/server.go` (add tool definition and handler)
- Test: `internal/mcp/server_test.go`

**Step 1: Write the failing test**

```go
func TestListSchemasTool(t *testing.T) {
    // Setup: Create database with multiple schemas
    // Call list-schemas tool
    // Verify response contains schema names
    // Expect: {"schemas":["public","bitnami_redmine"],"default_schema":"public"}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/mcp -run TestListSchemasTool -v`
Expected: FAIL

**Step 3: Write minimal implementation**

```go
// Tool definition
{
    Name: "list-schemas",
    Description: "List all accessible database schemas for a profile",
    InputSchema: inputSchemaFor[ListSchemasParams](),
}

type ListSchemasParams struct {
    ProfileName  string `json:"profile_name"`
    DatabaseName string `json:"database_name"`
}

type ListSchemasResult struct {
    Schemas       []string `json:"schemas"`
    DefaultSchema string   `json:"default_schema"`
}

func (s *MCPServer) handleListSchemas(ctx context.Context, req *mcp.CallToolRequest, input ListSchemasParams) (*mcp.CallToolResult, any, error) {
    // Query information_schema.schemata
    // Get current_schema() for default
    // Return ListSchemasResult
}
```

Query:
```sql
SELECT schema_name 
FROM information_schema.schemata 
WHERE schema_name NOT IN ('pg_catalog', 'information_schema')
  AND schema_name NOT LIKE 'pg_%'
ORDER BY schema_name
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/mcp -run TestListSchemasTool -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/mcp/server.go internal/mcp/server_test.go
git commit -m "feat(list-schemas): add schema discovery tool"
```

---

### Task 5: Implement get-search-path tool (READ-ONLY)

**Files:**
- Modify: `internal/mcp/server.go` (add tool definition and handler)
- Test: `internal/mcp/server_test.go`

**IMPORTANT**: This tool is read-only. `set-search-path` is NOT implemented due to connection pooling constraints.

**Step 1: Write the failing test**

```go
func TestGetSearchPathTool(t *testing.T) {
    // Call get-search-path tool
    // Verify response contains current search_path and current_schema
    // Expect: {"search_path":"public","current_schema":"public"}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/mcp -run TestGetSearchPathTool -v`
Expected: FAIL

**Step 3: Write minimal implementation**

```go
// Tool definition
{
    Name: "get-search-path",
    Description: "Get the current search_path and effective schema (read-only diagnostic)",
    InputSchema: inputSchemaFor[GetSearchPathParams](),
}

type GetSearchPathParams struct {
    ProfileName  string `json:"profile_name"`
    DatabaseName string `json:"database_name"`
}

type GetSearchPathResult struct {
    SearchPath               string `json:"search_path"`
    CurrentSchema            string `json:"current_schema"`
    ConnectionPoolingWarning string `json:"connection_pooling_warning,omitempty"`
}

func (s *MCPServer) handleGetSearchPath(ctx context.Context, req *mcp.CallToolRequest, input GetSearchPathParams) (*mcp.CallToolResult, any, error) {
    // Execute: SHOW search_path
    // Execute: SELECT current_schema()
    // Include warning about connection pooling
    // Return GetSearchPathResult
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/mcp -run TestGetSearchPathTool -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/mcp/server.go internal/mcp/server_test.go
git commit -m "feat(get-search-path): add read-only search_path diagnostic tool"
```

---

### Task 6: Enhance error messages with schema context

**Files:**
- Modify: `internal/mcp/server.go` (error handling in table-related functions)
- Modify: `internal/mcp/errors.go` (handleTableNotFound)
- Test: `internal/mcp/server_test.go`

**Step 1: Write the failing test**

```go
func TestDescribeTableErrorIncludesSchemaSuggestions(t *testing.T) {
    // Setup: Try to describe non-existent table
    // Call describe-table
    // Verify error includes:
    // - Clear message about missing table
    // - List of available schemas
    // - Schema-specific table suggestions
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/mcp -run TestDescribeTableErrorIncludesSchemaSuggestions -v`
Expected: FAIL

**Step 3: Write minimal implementation**

1. Create enhanced error structure:
```go
type SchemaAwareError struct {
    Code          string   `json:"code"`
    Message       string   `json:"message"`
    Schema        string   `json:"schema,omitempty"`
    Table         string   `json:"table"`
    AvailableSchemas []string `json:"available_schemas,omitempty"`
    Suggestions   []ErrorSuggestion `json:"suggestions"`
}
```

2. Update handleTableNotFound to query available schemas

**Step 4: Run test to verify it passes**

Run: `go test ./internal/mcp -run TestDescribeTableErrorIncludesSchemaSuggestions -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/mcp/server.go internal/mcp/errors.go internal/mcp/server_test.go
git commit -m "feat(error-handling): enhance table-related errors with schema context"
```

---

### Task 7: Enhance smart-query-builder for multi-schema support

**Files:**
- Modify: `internal/mcp/server.go` (smart-query-builder tool definition and handler)
- Test: `internal/mcp/server_test.go`

**Step 1: Write the failing test**

```go
func TestSmartQueryBuilderSearchesAllSchemas(t *testing.T) {
    // Setup: Create tables with matching names in different schemas
    // Call smart-query-builder with intent matching tables
    // Verify it searches across all schemas and returns schema-qualified results
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/mcp -run TestSmartQueryBuilderSearchesAllSchemas -v`
Expected: FAIL

**Step 3: Write minimal implementation**

1. Add optional `schema` parameter
2. When schema provided: search only that schema
3. When schema empty: search information_schema.tables across all user schemas
4. Score tables by relevance to intent keywords
5. Return best match with schema qualification

**Step 4: Run test to verify it passes**

Run: `go test ./internal/mcp -run TestSmartQueryBuilderSearchesAllSchemas -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/mcp/server.go internal/mcp/server_test.go
git commit -m "feat(smart-query-builder): enhance for multi-schema support"
```

---

### Task 8: Add schema parameter to sample-data and analyze-schema tools

**Files:**
- Modify: `internal/mcp/server.go` (tool definitions and handlers)
- Test: `internal/mcp/server_test.go`

**Step 1: Write the failing test**

```go
func TestSampleDataWithSchemaParameter(t *testing.T) {
    // Setup: Create test data in specific schema
    // Call sample-data with schema parameter
    // Verify it uses the provided schema
}

func TestAnalyzeSchemaWithSchemaParameter(t *testing.T) {
    // Setup: Create test tables in specific schema
    // Call analyze-schema with schema parameter
    // Verify analysis is scoped to that schema
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/mcp -run "TestSampleData|TestAnalyzeSchema" -v`
Expected: FAIL

**Step 3: Write minimal implementation**

1. Add optional `schema` parameter to both tools
2. Modify handlers to use ResolveSchema()
3. Fall back to auto-detection when schema is empty

**Step 4: Run test to verify it passes**

Run: `go test ./internal/mcp -run "TestSampleData|TestAnalyzeSchema" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/mcp/server.go internal/mcp/server_test.go
git commit -m "feat(sample-data,analyze-schema): add optional schema parameter"
```

---

### Task 9: Run full test suite and verify no regressions

**Files:**
- All test files

**Step 1: Run full test suite**

Run: `go test ./... -v`
Expected: ALL PASS (fix any failures from previous changes)

**Step 2: Run coverage**

Run: `go test -cover ./...`
Expected: Coverage >= previous level

**Step 3: Run linting**

Run: `golangci-lint run ./...`
Expected: 0 issues

**Step 4: Commit**

```bash
git add ./...
git commit -m "test: verify all tests pass after schema awareness changes"
```

---

### Task 10: Update documentation

**Files:**
- Modify: `README.md` (update features list)
- Modify: `docs/api-documentation.md` (add new tools and parameters)
- Modify: `docs/schema-awareness-enhancement-2026-03-23.md` (mark as implemented)

**Step 1: Update README.md**

Add to features:
```markdown
- 🔍 **Multi-Schema Support** - Work with tables in any PostgreSQL schema with automatic detection
```

Update tools table:
```markdown
| `list-schemas` | List all accessible database schemas |
| `get-search-path` | Get current search_path (read-only diagnostic) |
```

**Step 2: Update API documentation**

Document all new parameters:
- `schema` parameter for: describe-table, sample-data, analyze-schema, smart-query-builder
- New tools: list-schemas, get-search-path

**Step 3: Add connection pooling note**

```markdown
## Connection Pooling Notes

This server uses connection pooling per database profile. The `get-search-path` tool is read-only because `SET search_path` on pooled connections would contaminate other clients using the same connection. 

Always use the `schema` parameter to specify the target schema explicitly, or rely on auto-detection which falls back gracefully through: current_schema() → first accessible schema → 'public'.
```

**Step 4: Commit**

```bash
git add README.md docs/
git commit -m "docs: update for schema awareness enhancement"
```

---

## Architecture Decision Record

### Why NOT implement `set-search-path`?

**Context**: Connection pooling with `sql.DB`

**Decision**: Remove `set-search-path` from implementation

**Rationale**:
1. Connections are pooled per profile (`SetMaxOpenConns`, `SetMaxIdleConns`)
2. `SET search_path` is session-scoped - persists after query completes
3. Pooled connections are reused across different client requests
4. A client setting search_path would contaminate subsequent requests

**Consequences**:
- Users must explicitly specify schema in queries or use schema parameter
- Auto-detection provides sensible defaults without session state changes
- Safer for multi-tenant scenarios

**Alternatives Considered**:
- `SET LOCAL search_path` in transactions - would require adding transaction support
- Connection reset after search_path - performance overhead
- Connection pinning per schema - complex refactoring

---

## Testing Considerations

### Integration Tests Required

For each task, integration tests should:
1. Create schemas dynamically: `CREATE SCHEMA test_schema_1`
2. Create tables in different schemas
3. Test schema parameter passing
4. Test auto-detection fallback
5. Test case sensitivity (mixed-case schema names)
6. Clean up: `DROP SCHEMA test_schema_1 CASCADE`

### Edge Cases to Test

1. Empty schema parameter → auto-detection
2. Non-existent schema → error with suggestions
3. Mixed-case schema name → proper quoting
4. Schema with special characters → escaping
5. Multiple tables with same name in different schemas → disambiguation

---

## Total Estimated Effort

**2-3 days** for a single developer working full-time:
- Task 1-2: Foundation (4 hours)
- Task 3-8: Tool updates (6 hours)
- Task 9: Testing (2 hours)
- Task 10: Documentation (1 hour)

**Review checkpoints after each task recommended.**