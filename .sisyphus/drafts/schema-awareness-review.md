# Draft: Schema Awareness Enhancement Plan Review

## Interview Summary

**User Intent**: Review and improve the implementation plan for multi-schema PostgreSQL support

**Plan Reviewed**: `docs/schema-awareness-enhancement-2026-03-23.md`

---

## Research Findings

### 1. Current Implementation (Explore Agent)

**Key Files**:
- `internal/mcp/server.go:67` - Hardcoded `queryPostgresPublicInformationTable = "SELECT table_name FROM information_schema.tables WHERE table_schema='public'"`
- `internal/mcp/server.go:2871-2905` - `handleListTables` function
- `internal/mcp/server.go:2993-3021` - `handleDescribeTable` function
- `internal/mcp/server.go:495-530` - Tool registration pattern

**Current Behavior**:
- PostgreSQL queries assume 'public' schema
- No schema parameter in any tool
- No schema discovery tools exist
- MySQL/MariaDB use database-as-schema (no multi-schema within database)
- SQLite has no schema concept

**Error Handling**:
- `handleTableNotFound` in `errors.go:204-247` provides suggestions but lacks schema context

### 2. Connection Pool Implications (Explore Agent + Librarian)

**CRITICAL FINDING**:
- Connections are pooled per profile using `sql.DB` with `SetMaxOpenConns` and `SetMaxIdleConns`
- No transactions used in query execution - all operations use direct `db.ExecContext()` or `db.QueryContext()`
- `SET search_path` on a pooled connection persists beyond individual queries
- **Risk**: Schema contamination across different client requests using the same pooled connection

**Files**:
- `internal/db/driver.go` - Connection pooling setup

### 3. PostgreSQL Multi-Schema Best Practices (Librarian)

**Key Findings from PostgreSQL Docs and OSS Implementations**:

| Concern | Best Practice | Source |
|---------|--------------|--------|
| Schema Discovery | Use `information_schema.schemata` for portability, `pg_catalog` for performance | PostgreSQL 18 Docs |
| Tenant Isolation | Always qualify: `schema.table` not relying on `search_path` | Supabase, passwall-server |
| Connection Pooling | Use `SET LOCAL search_path` at transaction start with pgBouncer | DZone 2026, node-postgres |
| Performance | `pg_catalog` is 10-200x faster than `information_schema` | DBA StackExchange |
| Case Sensitivity | Normalize to lowercase, use `quote_ident()` for dynamic identifiers | PostgreSQL 18 Lexical |
| search_path Management | Reset to `public` after tenant operations | passwall-server |

**Critical: `current_schema()` Limitation**:
- Returns FIRST schema in search_path, NOT a persistent "current schema"
- If search_path is empty or contains nonexistent schemas, returns `NULL`
- Not reliable for determining "active" schema in multi-tenant scenarios

---

## Gap Analysis

### Critical Gaps Identified

#### Gap 1: `SET search_path` Unsafe with Pooled Connections

**Issue**: Plan proposes `set-search-path` tool that would contaminate pooled connections.

**Impact**: HIGH - Could cause wrong schema access across different clients.

**Solution Options**:
1. **Preferred**: Use schema-qualified queries (`schema.table`) instead of `SET search_path`
2. **Alternative**: Wrap `SET search_path` in transactions with `SET LOCAL` (requires transaction support)
3. **Alternative**: Implement connection reset after `SET search_path` operations

**Recommended Fix**:
- Remove `set-search-path` tool from plan, OR
- Implement with explicit warning about connection pooling, OR
- Use `SET LOCAL search_path` within transaction boundaries

#### Gap 2: `current_schema()` Not Reliable for Schema Detection

**Issue**: Plan proposes using `current_schema()` for auto-detection, but it'ssearch_path-dependent.

**Impact**: MEDIUM - Could return NULL or wrong schema.

**Solution**: 
- Use explicit schema parameter when provided
- Fall back to 'public' as default (current behavior)
- Document that `current_schema()` reflects search_path, not a persistent setting

#### Gap 3: Case Sensitivity Not Addressed

**Issue**: Schema names in PostgreSQL are case-sensitive when quoted. Plan doesn't handle mixed-case schema names.

**Impact**: MEDIUM - Could cause "schema not found" errors for schemas like `bitnami_redmine`.

**Solution**:
- Normalize schema names to lowercase on input
- Use `quote_ident()` for all dynamic schema/table names
- Document lowercase-only schema name requirement

#### Gap 4: MySQL/MariaDB Schema Differences

**Issue**: Plan focuses on PostgreSQL. MySQL uses database-as-schema model differently.

**Impact**: LOW - Need to clarify behavior for non-PostgreSQL databases.

**Solution**: Document that schema parameter is PostgreSQL-only feature, or implement equivalent for MySQL (database selection).

#### Gap 5: Performance Not Considered

**Issue**: Plan uses `information_schema` for all queries. High-frequency operations should use `pg_catalog`.

**Impact**: LOW but measurable - Slower metadata queries.

**Solution**: 
- Use `pg_catalog` for internal/high-frequency operations
- Use `information_schema` for user-facing API responses

---

## Task-by-Task Review

### Task 1: Update list-tables to show schema information
**Status**: GOOD - Basic approach is correct
**Improvement**: Add case sensitivity handling (quote_ident)

### Task 2: Implement auto-schema detection for describe-table
**Status**: NEEDS REVISION - `current_schema()` is unreliable
**Improvement**: Use explicit 'public' default or configuration, not `current_schema()`

### Task 3: Add schema parameter to describe-table
**Status**: GOOD

### Task 4: Implement list-schemas discovery tool
**Status**: GOOD - SQL is correct
**Improvement**: Consider using `pg_catalog.pg_namespace` for performance

### Task 5: Implement get-search-path and set-search-path tools
**Status**: CRITICAL ISSUE - `SET search_path` contaminates pooled connections
**Recommendation**: Remove `set-search-path` OR implement connection isolation

### Task 6: Enhance error messages with schema context
**Status**: GOOD

### Task 7: Enhance smart-query-builder for multi-schema support
**Status**: GOOD

### Task 8: Add schema parameter to sample-data and analyze-schema
**Status**: GOOD

### Task 9: Run full test suite
**Status**: GOOD

### Task 10: Update documentation
**Status**: GOOD - Needs to document connection pooling implications

---

## Final Recommendations

### Task 5 Decision: Split into Two Behaviors

| Tool | Implementation | Notes |
|------|---------------|-------|
| `get-search-path` | Implement as read-only query | Safe - no state changes |
| `set-search-path` | **Remove** or implement with explicit warning | Connection pooling makes session-scoped search_path unreliable |

**Recommendation**: Remove `set-search-path` from initial implementation. Document that schema should be explicitly qualified in queries. Add `get-search-path` as a useful read-only diagnostic tool.

### Default Schema Behavior (User Approved)

1. Try `current_schema()` first
2. If NULL, query `information_schema.schemata` for first accessible schema
3. Final fallback: 'public'

### MUST FIX Before Implementation

1. **Remove `set-search-path` tool** - Connection pooling makes it unsafe without architectural changes
2. **Add quote_ident() everywhere** - Prevent SQL injection and case sensitivity issues

### SHOULD FIX (Quality Improvements)

3. **Consider `pg_catalog` for performance** - Use for high-frequency internal operations
4. **Add Connection Pool Documentation** - Explain implications in docs

### NICE TO HAVE (Future Enhancements)

5. **MySQL Schema Support** - Document as PostgreSQL-only or implement equivalent
6. **Schema Caching** - Cache schema discovery results for performance