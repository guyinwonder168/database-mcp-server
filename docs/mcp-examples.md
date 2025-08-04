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
      "example": "{\"tool\": \"list-tables\", \"profile_name\": \"mydb\"}"
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
      "sql": "SELECT * FROM users"
    }
    ```
2. **Server returns structured error** (as above).
3. **Agent reads suggestions** and issues a `list-tables` request:
    ```json
    {
      "profile_name": "mydb"
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