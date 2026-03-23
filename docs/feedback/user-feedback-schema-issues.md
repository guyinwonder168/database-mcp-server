# Database MCP Server - User Feedback

## Date: 2026-03-13
## Context: Testing with Redmine PostgreSQL database (bitnami_redmine)

---

## Executive Summary

The Database MCP Server has strong core functionality but encounters friction when working with non-standard schemas (schemas other than `public`). The main issue is **lack of schema awareness** - the tools assume tables are in the default schema, making it difficult to work with multi-schema databases like PostgreSQL where each database can have multiple schemas.

---

## Issues Encountered

### Issue 1: Schema Not Automatically Detected

**Symptom:** Query execution fails with misleading "Column not found" errors when querying tables in non-public schemas.

**Example:**
```sql
-- This fails with "Column not found"
SELECT id, login, firstname, lastname FROM users WHERE login = 'eddy.wijaya';
```

**Root Cause:** The Redmine tables are in `bitnami_redmine` schema, not `public`. The tool attempts to find the table but fails silently or returns misleading errors.

**Workaround Used:**
```sql
SET search_path TO bitnami_redmine;
SELECT id, login, firstname, lastname FROM users WHERE login = 'eddy.wijaya';
```

**Impact:** High - Forces user to know internal implementation details.

---

### Issue 2: Table Discovery Tools Fail

**Symptom:** `describe-table`, `sample-data`, and `analyze-schema` return empty results or "column not found" errors.

**Examples:**
```
mcp_database_describe_table(table_name="users")
→ Returns: {"columns":[]}

mcp_database_sample_data(table_name="users", sample_size=5)
→ Returns: Column not found error
```

**Root Cause:** These tools don't accept a `schema` parameter and default to `public` schema, which doesn't contain the Redmine tables.

**Impact:** High - Cannot discover table structure for tables in non-public schemas.

---

### Issue 3: Smart Query Builder Lacks Context

**Symptom:** The `smart-query-builder` tool suggests irrelevant tables and doesn't learn from successful queries.

**Example:**
```
Intent: "Find user by login name eddy.wijaya"
Result: SELECT key, value FROM ar_internal_metadata
→ Wrong table suggested (should be users table)
```

**Root Cause:** 
1. Tool searches default schema only
2. No memory of previous successful queries
3. No schema context awareness

**Impact:** Medium - Reduces trust in the tool's suggestions.

---

### Issue 4: Missing Schema Discovery Tools

**Symptom:** No easy way to discover available schemas in a database.

**Needed:** A way to quickly find:
- What schemas exist in a database
- Which schema contains the tables I need
- Current search path settings

**Impact:** Medium - Forces manual SQL queries to discover schemas.

---

### Issue 5: Error Messages Are Misleading

**Symptom:** Errors say "Column not found" when the real issue is "Table not found in schema".

**Example:**
```
Error: "Column '' not found in table ''"
Actual Issue: Table 'users' not found in 'public' schema (it's in 'bitnami_redmine')
```

**Impact:** High - Wastes debugging time and causes confusion.

---

## Recommendations

### Recommendation 1: Add Schema Parameter to All Tools

Add an optional `schema` parameter to table-related tools:

```yaml
# Proposed new signature for table tools
describe-table:
  params:
    profile_name: string (required)
    database_name: string (required)
    table_name: string (required)
    schema: string (optional)  # NEW - defaults to search_path or 'public'
```

```python
# Example usage
mcp_database_describe_table(
    profile_name="bitnami-pg",
    database_name="bitnami_redmine",
    table_name="users",
    schema="bitnami_redmine"  # NEW
)
```

---

### Recommendation 2: Auto-Detect Schema from Search Path

Before executing queries, the server should:

1. Check if a search path is already set in the session
2. Detect which schema contains the target table
3. Automatically route to the correct schema
4. If ambiguous, prompt the user

```python
# Pseudocode implementation
def execute_sql(profile_name, database_name, sql):
    # Step 1: Get current search_path for the connection
    search_path = get_search_path(profile_name, database_name)
    
    # Step 2: Extract table names from SQL
    tables = extract_tables(sql)
    
    # Step 3: Find which schema contains these tables
    for table in tables:
        schema = find_schema_for_table(table, search_path)
        if schema:
            # Rewrite query with schema prefix or set search_path
            sql = rewrite_with_schema(sql, schema)
    
    # Step 4: Execute
    return actual_execute(sql)
```

---

### Recommendation 3: Add Schema Discovery Tools

Add new tools:

```yaml
# Tool: list-schemas
# Returns all schemas in a database
mcp_database_list_schemas:
  params:
    profile_name: string (required)
    database_name: string (required)

# Tool: get-search-path
# Returns current search path for the connection
mcp_database_get_search_path:
  params:
    profile_name: string (required)
    database_name: string (required)

# Tool: set-search-path
# Sets default schema for subsequent queries
mcp_database_set_search_path:
  params:
    profile_name: string (required)
    database_name: string (required)
    schema: string (required)
```

---

### Recommendation 4: Improve Error Messages

Instead of generic errors, provide actionable messages:

```python
# Current (bad)
Error: "Column not found"

# Improved (good)
Error: "Table 'users' not found in schema 'public'. 
Available schemas: public, bitnami_redmine, information_schema.
Did you mean: bitnami_redmine.users?"
```

---

### Recommendation 5: Enhance Smart Query Builder

1. **Accept schema parameter:**
```yaml
mcp_database_smart_query_builder:
  params:
    profile_name: string (required)
    database_name: string (required)
    intent: string (required)
    schema: string (optional)  # NEW
```

2. **Remember successful queries:**
- Track which tables/schemas worked for similar intents
- Use this for future suggestions

3. **Search multiple schemas:**
- Query `information_schema.tables` across all schemas
- Find tables matching the intent keywords

---

## Priority Ranking

| Priority | Recommendation | Impact |
|----------|----------------|--------|
| P0 | Schema parameter for table tools | High |
| P0 | Auto-detect schema | High |
| P1 | Improved error messages | High |
| P1 | Schema discovery tools | Medium |
| P2 | Smart query builder enhancements | Medium |

---

## Test Case for Validation

After implementing fixes, these queries should work:

```python
# Test 1: Describe table with schema
result = mcp_database_describe_table(
    profile_name="bitnami-pg",
    database_name="bitnami_redmine",
    table_name="users",
    schema="bitnami_redmine"
)
# Expected: Returns column metadata

# Test 2: Auto-schema detection
result = mcp_database_execute_sql(
    profile_name="bitnami-pg",
    database_name="bitnami_redmine",
    sql="SELECT id, login FROM users WHERE login = 'eddy.wijaya'"
)
# Expected: Automatically detects bitnami_redmine schema

# Test 3: List schemas
result = mcp_database_list_schemas(
    profile_name="bitnami-pg",
    database_name="bitnami_redmine"
)
# Expected: ["public", "bitnami_redmine", "information_schema", "pg_catalog"]

# Test 4: Smart query builder with schema
result = mcp_database_smart_query_builder(
    profile_name="bitnami-pg",
    database_name="bitnami_redmine",
    intent="Find user by login name",
    schema="bitnami_redmine"
)
# Expected: SELECT id, login, firstname, lastname FROM users WHERE login = ?
```

---

## Additional Notes

- The core functionality (execute-sql, validate-query, etc.) works well once the schema issue is resolved
- Profile management and security features work as expected
- The issues primarily affect PostgreSQL databases with multiple schemas
- MySQL databases (which don't have schema concept) work correctly

---

## Contact

For clarification on this feedback, please refer to the test session:
- Date: 2026-03-13
- Database: bitnami_redmine (PostgreSQL)
- Use case: Redmine user administration
