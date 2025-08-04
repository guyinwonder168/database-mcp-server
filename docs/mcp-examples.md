### Requirements Analysis: Automated Join Discovery

**Objective:**  
Enable AI/agents to programmatically discover joinable relationships between tables in a database, suggest optimal join paths, and generate example JOIN SQL.

**Expected Output:**  
- List of joinable tables and columns (foreign key relationships)
- Join paths between specified tables (direct and multi-hop)
- Example JOIN SQL statements for discovered relationships
- Human-readable summary of join logic

**Supported DBMS Features:**  
- Extraction of foreign key metadata from `information_schema` or DBMS-specific catalogs
- Support for MySQL, MariaDB, PostgreSQL, SQLite (where foreign keys are defined)
- Handling of composite and multi-table foreign keys

**Usage Scenarios:**  
- AI/agent wants to join two or more tables but does not know the schema
- Suggesting the most efficient join path for multi-table queries
- Validating if a join is possible between given tables

**Constraints:**  
- Read-only operation (no schema modification)
- Only relationships defined by foreign keys are considered (no heuristic joins)
- Output must be structured for both programmatic and human consumption
# MCP Tool Usage Examples

This document provides example workflows for using the Database MCP Provider tools programmatically.

## 1. List All Databases

**Request:**
```json
{
  "profile_name": "Some-profile-name"
}
```
**Tool:** `list-databases`

---

## 2. List All Tables in a Database

**Request:**
```json
{
  "profile_name": "some-profile-name",
  "database_name": "sampledb"
}
```
**Tool:** `list-tables`

---

## 3. Describe a Table

**Request:**
```json
{
  "profile_name": "some-profile-name",
  "table_name": "example_table"
}
```
**Tool:** `describe-table`

---

## 4. Execute a SQL Query

**Request:**
```json
{
  "profile_name": "some-profile-name",
  "sql": "SELECT * FROM sampledb.example_table WHERE some-field=34 AND DATE(somefield)='2025-06-08';"
}
```
**Tool:** `execute-sql`

---

## 5. Workflow: Discover Schema and Query Attendance

1. Call `list-databases` to find available databases.
2. Call `list-tables` with the target database to list tables.
3. Call `describe-table` for the attendance table to get columns and types.
4. Call `execute-sql` with a constructed query using the discovered schema.

---

## 6. Smart Query Builder

Generate optimized SQL from high-level intent and schema analysis.

**Request:**
```json
{
  "profile_name": "some-profile-name",
  "intent": "attendance dashboard",
  "database_name": "hrms_db",
  "table_names": ["attendance"]
}
```
**Tool:** `smart-query-builder`

**Example Response:**
```json
{
  "sql": "SELECT id, employee_id, date, time_in, time_out FROM attendance;",
  "explanation": "Selected table 'attendance' and columns [id, employee_id, date, time_in, time_out] based on keywords from intent 'attendance dashboard'."
}
```

**Use Cases:**
- Generate SQL from natural language descriptions
- Automatically select appropriate tables based on intent keywords
- Reduce manual SQL authoring for common queries

**Limitations:**
- Basic keyword matching (no advanced NLP)
- Generates simple SELECT statements only
- No JOIN support in current version
- Requires existing database schema

**Error Handling:**
If no table matches the intent, returns a structured error:
```json
{
  "status": "error",
  "error_code": "NO_TABLE_MATCH",
  "message": "No table found matching the intent for query generation. Available tables: users, reports, logs."
}
```

---

These examples enable LLMs and clients to discover schema and construct valid queries automatically.
## [2025-08-02] Example: Describe Table (Explicit Database Required)

To describe a table, you must now provide both the database name and the table name:

```json
{
  "profile_name": "local-mariadb",
  "database_name": "orangehrm_mysql",
  "table_name": "ohrm_attendance_record"
}
```

The profile's default database is ignored for this operation.
## MCP Action: execute-sql

### Description
Executes an arbitrary SQL query or statement on the selected database profile. Supports parameterized queries and enforces read-only mode for profiles as configured.

### Parameters

| Name          | Type          | Required | Description                                 |
|---------------|---------------|----------|---------------------------------------------|
| profile_name  | string        | Yes      | Name of the database profile                |
| sql           | string        | Yes      | SQL query or statement                      |
| database_name | string        | No       | Override the default database (optional)    |
| params        | array         | No       | Parameters for parameterized queries        |

### Example: Parameterized SELECT

```json
{
  "profile_name": "testsqlite",
  "sql": "SELECT id, name FROM test WHERE name = ?",
  "database_name": ":memory:",
  "params": ["Alice"]
}
```

### Example: INSERT (will fail if profile is read-only)

```json
{
  "profile_name": "testsqlite",
  "sql": "INSERT INTO test (name) VALUES (?)",
  "database_name": ":memory:",
  "params": ["Bob"]
}
```

### Read-Only Enforcement

If the profile is marked as read-only, only SELECT, SHOW, DESCRIBE, EXPLAIN, and PRAGMA statements are allowed. Mutating statements (INSERT, UPDATE, DELETE, etc.) will be rejected.

### Response

- For SELECT:  
  ```json
  {
    "columns": ["id", "name"],
    "rows": [[1, "Alice"]]
  }
  ```
- For mutating statements:  
  ```json
  {
    "affected": 1
  }
  ```
- On error:  
  Returns a structured error message and logs the error in JSON format.
## 7. Automated Join Discovery

Discover joinable relationships (foreign keys) between tables and suggest JOIN SQL for building complex queries.

**Tool:** `discover-joins`

### Overview
The join discovery feature enables AI agents and developers to programmatically discover foreign key relationships between database tables and receive suggested JOIN SQL statements. This is particularly useful when working with unfamiliar database schemas or when building complex multi-table queries.

### Features
- **Multi-Database Support**: Works with MySQL, MariaDB, PostgreSQL, and SQLite
- **Foreign Key Detection**: Automatically extracts foreign key metadata from database catalogs
- **Smart Filtering**: Focus discovery on specific tables or scan entire database
- **JOIN SQL Generation**: Provides ready-to-use JOIN statements
- **Human-Readable Summaries**: Clear explanations of discovered relationships

### Input Schema
```json
{
  "profile_name": "string",         // Required: Database profile to use
  "tables": ["string"]              // Optional: List of tables to focus join discovery (if omitted, discover all)
}
```

### Output Schema
```json
{
  "joins": [
    {
      "from_table": "string",       // Source table containing foreign key
      "from_column": "string",      // Foreign key column name
      "to_table": "string",         // Referenced table
      "to_column": "string",        // Referenced primary key column
      "relationship": "foreign_key", // Always "foreign_key" in current version
      "suggested_join_sql": "string" // Ready-to-use JOIN SQL statement
    }
  ],
  "summary": "string"               // Human-readable summary of discovered joins
}
```

### Usage Examples

#### Example 1: Discover All Joins in Database
**Request:**
```json
{
  "profile_name": "ecommerce_db"
}
```

**Response:**
```json
{
  "joins": [
    {
      "from_table": "orders",
      "from_column": "customer_id",
      "to_table": "customers",
      "to_column": "id",
      "relationship": "foreign_key",
      "suggested_join_sql": "SELECT * FROM orders JOIN customers ON orders.customer_id = customers.id"
    },
    {
      "from_table": "order_items",
      "from_column": "order_id",
      "to_table": "orders",
      "to_column": "id",
      "relationship": "foreign_key",
      "suggested_join_sql": "SELECT * FROM order_items JOIN orders ON order_items.order_id = orders.id"
    },
    {
      "from_table": "order_items",
      "from_column": "product_id",
      "to_table": "products",
      "to_column": "id",
      "relationship": "foreign_key",
      "suggested_join_sql": "SELECT * FROM order_items JOIN products ON order_items.product_id = products.id"
    }
  ],
  "summary": "Discovered 3 join(s) based on foreign key relationships."
}
```

#### Example 2: Focus on Specific Tables
**Request:**
```json
{
  "profile_name": "analytics_db",
  "tables": ["orders", "customers", "payments"]
}
```

**Response:**
```json
{
  "joins": [
    {
      "from_table": "orders",
      "from_column": "customer_id",
      "to_table": "customers",
      "to_column": "id",
      "relationship": "foreign_key",
      "suggested_join_sql": "SELECT * FROM orders JOIN customers ON orders.customer_id = customers.id"
    },
    {
      "from_table": "payments",
      "from_column": "order_id",
      "to_table": "orders",
      "to_column": "id",
      "relationship": "foreign_key",
      "suggested_join_sql": "SELECT * FROM payments JOIN orders ON payments.order_id = orders.id"
    }
  ],
  "summary": "Discovered 2 join(s) based on foreign key relationships."
}
```

#### Example 3: No Joins Found
**Request:**
```json
{
  "profile_name": "simple_db",
  "tables": ["logs", "cache"]
}
```

**Response:**
```json
{
  "joins": [],
  "summary": "Discovered 0 join(s) based on foreign key relationships."
}
```

### Practical Workflows

#### Workflow 1: Building a Customer Order Report
1. **Discover joins**: Use `discover-joins` to find relationships between customer and order tables
2. **Extract schema**: Use `describe-table` to understand column structure
3. **Build query**: Construct JOIN SQL using discovered relationships
4. **Execute**: Use `execute-sql` to run the final query

```json
// Step 1: Discover joins
{
  "profile_name": "sales_db",
  "tables": ["customers", "orders", "order_items", "products"]
}

// Step 2: Build multi-table query using discovered relationships
{
  "profile_name": "sales_db",
  "sql": "SELECT c.name, o.order_date, oi.quantity, p.product_name FROM customers c JOIN orders o ON c.id = o.customer_id JOIN order_items oi ON o.id = oi.order_id JOIN products p ON oi.product_id = p.id WHERE o.order_date >= '2024-01-01'"
}
```

#### Workflow 2: Schema Exploration for AI Agents
When an AI agent encounters an unfamiliar database:

1. **List databases**: `list-databases` to see available databases
2. **List tables**: `list-tables` to see all tables in target database
3. **Discover joins**: `discover-joins` to understand table relationships
4. **Describe key tables**: `describe-table` for detailed column information
5. **Smart querying**: Use discovered relationships to build intelligent queries

### Database-Specific Behavior

#### MySQL/MariaDB
- Queries `INFORMATION_SCHEMA.KEY_COLUMN_USAGE` for foreign key metadata
- Supports complex foreign key constraints
- Handles multi-column foreign keys

#### PostgreSQL
- Uses `information_schema` tables with PostgreSQL-specific joins
- Extracts foreign key constraints from system catalogs
- Supports advanced constraint types

#### SQLite
- Uses `PRAGMA foreign_key_list()` for each table
- Requires foreign keys to be explicitly defined
- May need `PRAGMA foreign_keys = ON` for constraint enforcement

### Error Scenarios

#### Profile Not Found
```json
{
  "error": "profile not found"
}
```

#### Unsupported Database Type
```json
{
  "error": "unsupported db_type for join discovery"
}
```

#### Connection Issues
```json
{
  "error": "dial tcp 127.0.0.1:5432: connect: connection refused"
}
```

### Best Practices

1. **Use table filtering**: When working with large databases, specify the `tables` parameter to focus on relevant tables
2. **Combine with schema discovery**: Use `describe-table` alongside join discovery for complete understanding
3. **Validate relationships**: Test generated JOIN SQL with small datasets before running on production data
4. **Consider performance**: Large databases may have many relationships; filter appropriately
5. **Handle empty results**: Not all tables have foreign key relationships; always check for empty joins array

### Limitations

- Only detects formally defined foreign key constraints
- Does not infer relationships based on naming conventions or data patterns
- Multi-hop join paths require manual construction using multiple discovered relationships
- Performance depends on database size and number of foreign key constraints

---

## 8. Sample Data Fetching

Fetch sample rows from a table to help AI/agents infer data types, formats, and value ranges without needing to query the entire table.

**Tool:** `sample-data`

### Overview
The sample data tool provides a quick way to inspect the actual data in a table. This is essential for AI agents to understand data formats (e.g., date strings, ID formats), value ranges, and typical content before constructing complex queries.

### Features
- **Quick Data Snapshot**: Fetches a small, configurable number of rows.
- **Multi-Database Support**: Works with MySQL, MariaDB, PostgreSQL, and SQLite.
- **Performance-Focused**: Limits the number of rows to prevent large data transfers and ensure fast responses.
- **Clear Output**: Returns column names and rows with data in a structured format.

### Input Schema
```json
{
  "profile_name": "string",         // Required: Database profile to use
  "table_name": "string",           // Required: The table to sample data from
  "database_name": "string",        // Optional: Override the profile's default database
  "sample_size": "integer"          // Optional: Number of rows to fetch (default: 3, max: 100)
}
```

### Output Schema
```json
{
  "table_name": "string",           // The name of the table sampled
  "sample_size": "integer",         // The actual number of rows returned
  "columns": ["string"],            // List of column names
  "sample_rows": [[]],              // An array of rows, where each row is an array of values
  "summary": "string"               // Human-readable summary of the result
}
```

### Usage Examples

#### Example 1: Default Sample Size
**Request:**
```json
{
  "profile_name": "inventory_db",
  "table_name": "products"
}
```

**Response (default sample size of 3):**
```json
{
  "table_name": "products",
  "sample_size": 3,
  "columns": ["id", "product_name", "price", "stock_quantity", "last_updated"],
  "sample_rows": [
    [101, "SuperWidget", 19.99, 500, "2025-08-04T10:00:00Z"],
    [102, "MegaWidget", 29.99, 350, "2025-08-03T15:30:00Z"],
    [103, "GigaWidget", 49.99, 200, "2025-08-04T11:20:00Z"]
  ],
  "summary": "Retrieved 3 sample row(s) from table 'products' with 5 column(s)."
}
```

#### Example 2: Custom Sample Size
**Request:**
```json
{
  "profile_name": "logs_db",
  "table_name": "api_requests",
  "sample_size": 5
}
```

**Response:**
```json
{
  "table_name": "api_requests",
  "sample_size": 5,
  "columns": ["request_id", "endpoint", "status_code", "response_time_ms", "timestamp"],
  "sample_rows": [
    ["a1b2c3d4", "/api/v1/users", 200, 55, "2025-08-04T09:20:15Z"],
    ["e5f6g7h8", "/api/v1/products", 200, 120, "2025-08-04T09:20:18Z"],
    ["i9j0k1l2", "/api/v1/users/123", 404, 30, "2025-08-04T09:20:21Z"],
    ["m3n4o5p6", "/api/v1/orders", 201, 250, "2025-08-04T09:20:25Z"],
    ["q7r8s9t0", "/api/v1/products/456", 200, 110, "2025-08-04T09:20:30Z"]
  ],
  "summary": "Retrieved 5 sample row(s) from table 'api_requests' with 5 column(s)."
}
```

### Practical Workflows

#### Workflow: AI-Powered Query Generation
1.  **List Tables**: Use `list-tables` to discover available tables.
2.  **Describe Table**: Use `describe-table` on a promising table to understand its schema (columns, types).
3.  **Fetch Sample Data**: Use `sample-data` to see what the data *actually* looks like. This helps in formatting values for WHERE clauses (e.g., date formats).
4.  **Construct Query**: Build an accurate `execute-sql` query using the schema and data format knowledge.

### Error Scenarios
- **Table Not Found**: Returns an error if the specified table does not exist.
- **Profile Not Found**: Returns an error if the `profile_name` is invalid.
- **Connection Issues**: Returns a standard connection error if the database is unreachable.

### Best Practices
- Use a small `sample_size` (1-5) for quick inspection.
- Combine with `describe-table` for a complete picture of a table's structure and content.
- Do not rely on sample data to be representative of the entire dataset's distribution; it is for format and type inference.

---