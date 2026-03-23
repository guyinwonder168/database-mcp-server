# Database MCP Server - Enhanced Design Proposal

## Version: 2.0 (Design Draft)
## Date: 2026-03-13
## Status: Design Proposal

---

## 1. Executive Summary

This document proposes enhancements to the Database MCP Server to address schema awareness limitations and improve usability for multi-schema databases (particularly PostgreSQL). The proposed changes maintain backward compatibility while adding powerful new capabilities.

### Key Problems Being Solved

| Problem | Current Impact | Proposed Solution |
|---------|----------------|-------------------|
| Schema not detected | Tables in non-public schemas fail | Auto-detect + explicit schema param |
| Discovery tools broken | Can't discover table structure | Add schema parameter to all tools |
| Error messages misleading | "Column not found" for table issues | Context-aware error messages |
| No schema discovery | Can't find available schemas | New `list-schemas` tool |
| Smart query builder weak | Wrong table suggestions | Schema-aware with context memory |

---

## 2. Architecture Changes

### 2.1 Current Architecture

```
MCP Client
  -> MCP Server Layer (internal/mcp)
     -> Config Layer (internal/config)
     -> DB Layer (internal/db)
     -> Logging Layer (internal/log)
```

### 2.2 Proposed Architecture

```
MCP Client
  -> MCP Server Layer (internal/mcp)
     -> Config Layer (internal/config)
     -> DB Layer (internal/db)
        -> Connection Pool Manager
        -> Schema Resolver (NEW)
        -> Query Rewriter (NEW)
     -> Session Context Manager (NEW)
     -> Logging Layer (internal/log)
```

### 2.3 New Components

#### 2.3.1 Schema Resolver

```go
// internal/db/schema_resolver.go

type SchemaResolver struct {
    // Caches schema information per profile/database
    schemaCache map[string]*SchemaCacheEntry
    
    // Current session context
    sessionContexts map[string]*SessionContext
}

type SchemaCacheEntry struct {
    Schemas       []string
    TableSchemas  map[string]string  // table -> schema mapping
    LastUpdated   time.Time
}

type SessionContext struct {
    ProfileName   string
    DatabaseName  string
    SearchPath    []string  // PostgreSQL search_path
    LastQueryTime time.Time
}

func (r *SchemaResolver) ResolveTable(tableName, databaseName string) (string, error) {
    // 1. Check session context for current search_path
    // 2. Look up table in all accessible schemas
    // 3. Return fully qualified table reference
}
```

#### 2.3.2 Query Rewriter

```go
// internal/db/query_rewriter.go

type QueryRewriter struct {
    resolver *SchemaResolver
}

func (r *QueryRewriter) Rewrite(sql string, profileName, databaseName string) (string, error) {
    // 1. Parse SQL to extract table names
    // 2. For each table, resolve the correct schema
    // 3. Rewrite query with schema prefixes or SET search_path
    // 4. Return rewritten SQL
}
```

#### 2.3.3 Session Context Manager

```go
// internal/db/session_manager.go

type SessionManager struct {
    sessions map[string]*DBSession
    mu       sync.RWMutex
}

type DBSession struct {
    ProfileName    string
    DatabaseName   string
    SearchPath     []string
    ActiveSchema   string
    QueryHistory   []QueryHistoryEntry
    LastActivity   time.Time
}

func (m *SessionManager) GetOrCreateSession(profileName, databaseName string) *DBSession {
    // Create session if not exists
    // Initialize with database default search_path
}
```

---

## 3. New Tool Definitions

### 3.1 Schema Discovery Tools (NEW)

#### 3.1.1 list-schemas

```yaml
name: list-schemas
description: List all schemas in a database
params:
  profile_name:
    type: string
    required: true
    description: Name of the connection profile
  database_name:
    type: string
    required: true
    description: Database to list schemas from
  include_system:
    type: boolean
    required: false
    default: false
    description: Include system schemas (information_schema, pg_catalog)
```

**Example:**
```python
mcp_database_list_schemas(
    profile_name="bitnami-pg",
    database_name="bitnami_redmine",
    include_system=False
)
```

**Response:**
```json
{
  "schemas": ["bitnami_redmine", "public"],
  "default_schema": "bitnami_redmine"
}
```

#### 3.1.2 get-search-path

```yaml
name: get-search-path
description: Get current search path for PostgreSQL connection
params:
  profile_name:
    type: string
    required: true
  database_name:
    type: string
    required: true
```

**Response:**
```json
{
  "search_path": ["bitnami_redmine", "public"],
  "current_schema": "bitnami_redmine"
}
```

#### 3.1.3 set-search-path

```yaml
name: set-search-path
description: Set default schema search path for session
params:
  profile_name:
    type: string
    required: true
  database_name:
    type: string
    required: true
  schema:
    type: string
    required: true
    description: Schema name to set as default
  prepend:
    type: boolean
    required: false
    default: true
    description: Prepend to search path (true) or replace entirely (false)
```

### 3.2 Enhanced Existing Tools

#### 3.2.1 Updated describe-table

```yaml
# NEW PARAMETER ADDED
name: describe-table
params:
  profile_name:
    type: string
    required: true
  database_name:
    type: string
    required: true
  table_name:
    type: string
    required: true
  schema:           # NEW
    type: string
    required: false
    description: Schema containing the table (auto-detected if omitted)
```

**Behavior Change:**
- If `schema` omitted: auto-detect using search_path
- If `schema` provided: use explicitly provided schema
- Error message includes suggestions if table not found

#### 3.2.2 Updated sample-data

```yaml
name: sample-data
params:
  profile_name:
    type: string
    required: true
  database_name:
    type: string
    required: true
  table_name:
    type: string
    required: true
  sample_size:
    type: integer
    required: false
    default: 5
  schema:           # NEW
    type: string
    required: false
    description: Schema containing the table
```

#### 3.2.3 Updated list-tables

```yaml
name: list-tables
params:
  profile_name:
    type: string
    required: true
  database_name:
    type: string
    required: true
  schema:           # NEW
    type: string
    required: false
    description: Filter to specific schema
  include_views:
    type: boolean
    required: false
    default: true
```

#### 3.2.4 Updated execute-sql

```yaml
name: execute-sql
params:
  profile_name:
    type: string
    required: true
  database_name:
    type: string
    required: true
  sql:
    type: string
    required: true
  params:
    type: array
    required: false
    description: Positional parameters for prepared statements
  auto_schema:      # NEW
    type: boolean
    required: false
    default: true
    description: Automatically resolve schema for unqualified table names
```

**Behavior Change:**
- `auto_schema=true`: Automatically detect and rewrite schema references
- `auto_schema=false`: Use current search_path (backward compatible)

#### 3.2.5 Updated smart-query-builder

```yaml
name: smart-query-builder
params:
  profile_name:
    type: string
    required: true
  database_name:
    type: string
    required: true
  intent:
    type: string
    required: true
    description: Natural language query intent
  schema:           # NEW
    type: string
    required: false
    description: Target schema for query
  table_names:      # ENHANCED
    type: array
    required: false
    description: Hint tables (now searches across all schemas)
```

---

## 4. Error Handling Improvements

### 4.1 Current vs Improved Error Messages

| Scenario | Current Error | Improved Error |
|----------|--------------|----------------|
| Table not found in public | "Column not found in table ''" | "Table 'users' not found in schema 'public'. Available schemas: public, bitnami_redmine. Did you mean: bitnami_redmine.users?" |
| Invalid SQL | Generic parse error | "SQL syntax error at position 19: expected WHERE clause. Suggestions: Add WHERE condition, check table name spelling" |
| Connection failed | "Connection failed" | "Connection failed to host:5432. Check: 1) Host is reachable, 2) Port 5432 is open, 3) Credentials are valid. Network error: timeout after 30s" |

### 4.2 Error Response Structure

```go
type MCPError struct {
    Code          string                 `json:"code"`
    Message       string                 `json:"message"`
    Details       map[string]interface{} `json:"details,omitempty"`
    Suggestions   []string              `json:"suggestions,omitempty"`
    Context       *ErrorContext         `json:"context,omitempty"`
}

type ErrorContext struct {
    Query         string   `json:"query,omitempty"`
    TableName     string   `json:"table_name,omitempty"`
    SchemaName    string   `json:"schema_name,omitempty"`
    AvailableSchemas []string `json:"available_schemas,omitempty"`
    AvailableTables map[string][]string `json:"available_tables,omitempty"` // schema -> tables
}
```

---

## 5. Smart Query Builder Enhancements

### 5.1 Schema-Aware Query Generation

```python
# Current behavior (wrong)
Intent: "Find user by login name eddy.wijaya"
Result: SELECT key, value FROM ar_internal_metadata  # Wrong table!

# Proposed behavior (correct)
Intent: "Find user by login name eddy.wijaya"
1. Search ALL schemas for tables matching "user"
   - bitnami_redmine.users (matches: user, login)
   - public.user_sessions (matches: user)
2. Select best match based on column relevance
3. Generate: SELECT id, login, firstname, lastname FROM bitnami_redmine.users WHERE login = 'eddy.wijaya'
```

### 5.2 Query History & Learning

```go
type QueryHistory struct {
    ProfileName   string
    DatabaseName  string
    Intent       string
    GeneratedSQL string
    TablesUsed   []string
    SchemaUsed   string
    Success      bool
    ExecutionTime time.Duration
}

func (b *SmartQueryBuilder) LearnFromExecution(history QueryHistory) {
    // Store successful mappings: intent -> (schema, table, columns)
    // Use for future suggestions
}
```

### 5.3 Multi-Schema Search

```go
func (b *SmartQueryBuilder) findMatchingTables(intent string, databaseName string) []TableMatch {
    matches := []TableMatch{}
    
    // Search across ALL schemas
    schemas := b.getAllSchemas(databaseName)
    
    for _, schema := range schemas {
        tables := b.listTablesInSchema(schema)
        for _, table := range tables {
            score := b.calculateRelevanceScore(intent, table)
            if score > threshold {
                matches = append(matches, TableMatch{
                    Schema: schema,
                    Table:  table.Name,
                    Score:  score,
                    Columns: table.Columns,
                })
            }
        }
    }
    
    // Sort by score descending
    sort.Slice(matches, func(i, j int) bool {
        return matches[i].Score > matches[j].Score
    })
    
    return matches
}
```

---

## 6. Implementation Roadmap

### Phase 1: Schema Foundation (v1.4.0)

| Task | Effort | Priority |
|------|--------|----------|
| Add schema parameter to all table tools | Medium | P0 |
| Implement list-schemas tool | Low | P0 |
| Implement schema resolver | Medium | P0 |
| Auto-detect schema in execute-sql | Medium | P0 |

### Phase 2: Error Improvements (v1.4.1)

| Task | Effort | Priority |
|------|--------|----------|
| Context-aware error messages | Low | P1 |
| Suggestion engine | Medium | P1 |
| Error context in responses | Low | P1 |

### Phase 3: Smart Query Enhancements (v1.5.0)

| Task | Effort | Priority |
|------|--------|----------|
| Multi-schema search | Medium | P2 |
| Query history learning | Medium | P2 |
| Relevance scoring | Medium | P2 |

### Phase 4: Advanced Features (v1.6.0)

| Task | Effort | Priority |
|------|--------|----------|
| Session context persistence | Low | P3 |
| Query caching | Medium | P3 |
| Federated schema discovery | Medium | P3 |

---

## 7. Backward Compatibility

### 7.1 Compatibility Matrix

| Change | Backward Compatible | Migration Required |
|--------|--------------------|-------------------|
| New schema param (optional) | ✅ Yes | None |
| auto_schema default=true | ✅ Yes | Set auto_schema=false for old behavior |
| New error fields | ✅ Yes | None |
| New tools | ✅ Yes (additive) | None |

### 7.2 Deprecation Path

```go
// If we ever need to remove auto_schema:
// 1. v1.x: auto_schema=true by default, show deprecation warning
// 2. v1.x+1: auto_schema=false by default
// 3. v1.x+2: Remove parameter, always auto-resolve
```

---

## 8. Testing Strategy

### 8.1 Schema Resolution Tests

```go
// Test cases for schema resolution
testCases := []struct {
    name           string
    searchPath     []string
    tableName      string
    expectedSchema string
}{
    {
        name:           "Single schema in search_path",
        searchPath:     []string{"bitnami_redmine"},
        tableName:      "users",
        expectedSchema: "bitnami_redmine",
    },
    {
        name:           "Multiple schemas, table in first",
        searchPath:     []string{"bitnami_redmine", "public"},
        tableName:      "users",
        expectedSchema: "bitnami_redmine",
    },
    {
        name:           "Table only in second schema",
        searchPath:     []string{"bitnami_redmine", "public"},
        tableName:      "pg_stat_activity",
        expectedSchema: "public",
    },
}
```

### 8.2 Integration Test Database

Create test PostgreSQL database with multiple schemas:

```sql
-- Test setup
CREATE SCHEMA test_schema_1;
CREATE SCHEMA test_schema_2;

CREATE TABLE test_schema_1.users (id INT, name TEXT);
CREATE TABLE test_schema_2.orders (id INT, user_id INT, total DECIMAL);

-- Test queries
-- Should auto-resolve to test_schema_1.users
SELECT * FROM users;

-- Should use explicit schema
SELECT * FROM test_schema_2.orders;
```

---

## 9. Configuration Enhancements

### 9.1 Profile-Level Schema Defaults

```yaml
profiles:
  - name: bitnami-pg
    db_type: postgres
    # NEW: Default schema for this profile
    default_schema: bitnami_redmine
    # NEW: Auto-resolve schema for queries
    auto_schema: true
```

### 9.2 Global Settings

```yaml
settings:
  # Default for all profiles
  default_auto_schema: true
  
  # Include system schemas in discovery
  include_system_schemas: false
  
  # Schema cache TTL
  schema_cache_ttl: 300  # seconds
```

---

## 10. Success Metrics

### 10.1 Target Improvements

| Metric | Current | Target | Improvement |
|--------|---------|--------|-------------|
| First successful query time | ~5 min | <1 min | 80% faster |
| Schema-related errors | ~60% | <5% | 90% reduction |
| Smart query accuracy | ~30% | >80% | 2.5x improvement |
| Error resolution time | ~10 min | <1 min | 90% faster |

### 10.2 Acceptance Criteria

- [ ] All existing tests pass
- [ ] New schema tools work for PostgreSQL, MySQL, SQLite
- [ ] Auto-schema detection works for 90% of common queries
- [ ] Error messages include actionable suggestions
- [ ] Smart query builder finds correct table >80% of time
- [ ] Documentation updated
- [ ] Backward compatibility maintained

---

## 11. Appendix: Full Tool Signature Changes

### 11.1 Tool Comparison Table

| Tool | Current Params | New Params | Notes |
|------|---------------|------------|-------|
| list-schemas | - | profile_name, database_name, include_system | **NEW** |
| get-search-path | - | profile_name, database_name | **NEW** |
| set-search-path | - | profile_name, database_name, schema, prepend | **NEW** |
| list-tables | profile_name, database_name | + schema | Modified |
| describe-table | profile_name, database_name, table_name | + schema | Modified |
| sample-data | profile_name, database_name, table_name, sample_size | + schema | Modified |
| execute-sql | profile_name, database_name, sql, params | + auto_schema | Modified |
| smart-query-builder | profile_name, database_name, intent, table_names | + schema | Modified |
| list-databases | profile_name | (unchanged) | - |
| analyze-schema | profile_name, analysis_level | (unchanged) | - |

---

## 12. References

- Current PRD: `docs/prd.md`
- Current API: `docs/api-documentation.md`
- Technical Specs: `docs/technical-specifications.md`
- User Feedback: `docs/user-feedback-schema-issues.md`
