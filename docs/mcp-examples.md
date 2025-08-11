### Requirements Analysis: Automated Join Discovery

...

---

## Error Handling & Feedback Loop Examples

### Structured Error Response Example

When a query fails (e.g., referencing a missing table), the MCP server returns a structured error with actionable suggestions:

```json
{
  "status": "error",
  "error_code": "TABLE_NOT_FOUND",
  "message": "Table 'users' not found",
  "details": "The specified table does not exist in the database",
  "suggestions": [
    {
      "action": "List available tables",
      "description": "Use the list-tables tool to see all tables in the database",
      "example": "{\"tool\": \"list-tables\", \"profile_name\": \"mydb\", \"database_name\": \"sampledb\"}"
    },
    {
      "action": "Check table name spelling",
      "description": "Verify the table name is spelled correctly and matches the case sensitivity requirements of your database"
    }
  ],
  "context": {
    "profile_name": "mydb",
    "table_name": "users",
    "database_name": "sampledb"
  }
}
```

### Feedback Loop Workflow

1. **Agent attempts a query** with `execute-sql`:
    ```json
    {
      "profile_name": "mydb",
      "database_name": "sampledb",
      "sql": "SELECT * FROM users"
    }
    ```
2. **Server returns structured error** (as above).
3. **Agent reads suggestions** and issues a `list-tables` request:
    ```json
    {
      "profile_name": "mydb",
      "database_name": "sampledb"
    }
    ```
4. **Agent corrects the query** using the actual table name and retries.

### More Error Examples

#### Column Not Found

```json
{
  "status": "error",
  "error_code": "COLUMN_NOT_FOUND",
  "message": "Column 'email' not found in table 'users'",
  "details": "The specified column does not exist in the table",
  "suggestions": [
    {
      "action": "Describe table schema",
      "description": "Use the describe-table tool to see all columns in the table",
      "example": "{\"tool\": \"describe-table\", \"profile_name\": \"mydb\", \"table_name\": \"users\"}"
    },
    {
      "action": "Check column name spelling",
      "description": "Verify the column name is spelled correctly and matches the case sensitivity requirements"
    }
  ],
  "context": {
    "profile_name": "mydb",
    "table_name": "users",
    "column_name": "email"
  }
}
```

#### SQL Syntax Error

```json
{
  "status": "error",
  "error_code": "SQL_SYNTAX_ERROR",
  "message": "SQL syntax error",
  "details": "syntax error at or near \"FROMM\"",
  "suggestions": [
    {
      "action": "Check SQL syntax",
      "description": "Verify your SQL syntax is valid for postgres"
    },
    {
      "action": "Add semicolon",
      "description": "Some databases require a semicolon at the end of SQL statements"
    }
  ],
  "context": {
    "sql": "SELECT * FROMM users",
    "db_type": "postgres"
  }
}
```

---

These structured error responses enable AI agents and clients to self-correct, retry, and automate database interactions safely and efficiently.
---

## List Tools

Lists all available MCP tools/actions for programmatic discovery.

### Request
```json
{
  "method": "list-tools",
  "params": {}
}
```

### Response
```json
{
  "tools": [
    { "name": "configure-profile", "description": "Create or update a database connection profile. Required for all database actions.\nExample:\n{\"profile_name\":\"some-profile-name\",\"db_type\":\"mariadb\",\"host\":\"localhost\",\"port\":3306,\"username\":\"app\",\"password\":\"secret\",\"database_name\":\"mysql\",\"readonly\":false}" },
    { "name": "list-profiles", "description": "List all configured database profiles.\nExample:\n{}" },
    { "name": "execute-sql", "description": "Execute an arbitrary SQL query or statement. Use the 'database_name' parameter to select a database if needed.\nNote: For cross-database queries or describing tables in another database, use fully qualified table names (e.g., db.table).\nExample:\n{\"profile_name\":\"some-profile-name\",\"database_name\":\"some-database-name\",\"sql\":\"SELECT * FROM some-table-name WHERE some-field-name=34;\"}\n{\"profile_name\":\"some-profile-name\",\"sql\":\"DESCRIBE some-database-name.some-table-name\"}" },
    { "name": "list-tables", "description": "List all tables in the selected database. Use 'database_name' to override the profile's default database.\nExample:\n{\"profile_name\":\"some-profile-name\",\"database_name\":\"some-database-name\"}" },
    { "name": "describe-table", "description": "Describe the comprehensive schema of a table including columns, types, constraints, comments, and metadata. Returns detailed information to enable AI/agents to understand table structure and build intelligent queries.\nReturns: column names, data types, nullable status, key constraints, default values, column comments, character sets, collation, auto-increment status, max length, precision, and scale.\nExample:\n{\"profile_name\":\"some-profile-name\",\"database_name\":\"some-database-name\",\"table_name\":\"some-table-name\"}" },
    { "name": "list-databases", "description": "List all databases/schemas available to the profile.\nExample:\n{\"profile_name\":\"some-profile-name\"}" },
    { "name": "analyze-schema", "description": "Perform schema analysis for a database, including table/column metadata, relationships, and sample data integration.\n\nRequired parameters:\n\t - profile_name: Database profile to analyze\n\t - analysis_level: REQUIRED. Must be one of \"basic\", \"detailed\", \"comprehensive\".\n\t   - BASIC: Quick overview for initial exploration\n\t   - DETAILED: Comprehensive schema for query construction\n\t   - COMPREHENSIVE: Deep business context with AI insights\n\nOptional parameters:\n\t - database_name: Specific database (uses profile default if empty)\n\t - include_tables: Specific tables to analyze (all if empty)\n\t - exclude_tables: Tables to exclude from analysis\n\t - sample_size: Rows to sample per table (default: 10)\n\t - include_queries: Generate query suggestions (default: true)\n\nAI agents MUST specify analysis_level. Example:\n{\"profile_name\":\"analytics_db\",\"analysis_level\":\"detailed\",\"database_name\":\"analytics_db\"}" },
    { "name": "mcp-info", "description": "Show MCP provider version and author.\nExample:\n{}" },
    { "name": "smart-query-builder", "description": "Generate optimized SQL from high-level intent and schema analysis.\nInput: profile_name, intent (natural language), optional database_name/table_name(s).\nReturns: generated SQL, explanation, and any errors.\nExample:\n{\"profile_name\":\"some-profile-name\",\"intent\":\"attendance dashboard\"}" },
    { "name": "discover-joins", "description": "Discover joinable relationships (foreign keys) between tables and suggest JOIN SQL.\nInput: profile_name (required), tables (optional).\nReturns: list of join suggestions and summary.\nExample:\n{\"profile_name\":\"analytics_db\",\"tables\":[\"orders\",\"customers\"]}" },
    { "name": "sample-data", "description": "Fetch sample rows from a table to help AI/agents infer data types, formats, and value ranges.\nInput: profile_name (required), table_name (required), database_name (optional), sample_size (optional, default: 3).\nReturns: sample rows with column names and values.\nExample:\n{\"profile_name\":\"analytics_db\",\"table_name\":\"users\",\"sample_size\":5}" },
    { "name": "list-tools", "description": "List all available MCP tools and their descriptions.\nExample:\n{}" }
  ]
}
```

### Use Case
This action enables AI agents and MCP clients to programmatically discover all available tools without needing prior knowledge of the server's capabilities.