# Database MCP Server - API Documentation

## Overview

The Database MCP Server provides a comprehensive set of MCP (Model Context Protocol) actions for interacting with SQL databases. This document describes the complete API specification for all available tools.

## Protocol Specification

### Communication Protocol
- **Transport**: Stdio (standard input/output)
- **Protocol**: JSON-RPC 2.0 over MCP
- **Encoding**: UTF-8
- **Format**: JSON

### Request Format
```json
{
  "jsonrpc": "2.0",
  "method": "tool_name",
  "params": {
    "parameter1": "value1",
    "parameter2": "value2"
  },
  "id": "request_id"
}
```

### Response Format
```json
{
  "jsonrpc": "2.0",
  "result": {
    "status": "success",
    "data": {...}
  },
  "id": "request_id"
}
```

### Error Response Format
```json
{
  "jsonrpc": "2.0",
  "error": {
    "code": "ERROR_CODE",
    "message": "Human-readable error message",
    "details": {
      "additional": "context"
    },
    "suggestions": [
      "Suggestion 1",
      "Suggestion 2"
    ]
  },
  "id": "request_id"
}
```

## MCP Tools Reference

### 1. configure-profile

Create or update a database connection profile.

#### Parameters
```json
{
  "profile_name": "string (required)",
  "db_type": "string (required)",
  "host": "string (required)",
  "port": "integer (required)",
  "username": "string (required)",
  "password": "string (required)",
  "database_name": "string (required)",
  "readonly": "boolean (optional, default: false)"
}
```

#### Parameter Details
- **profile_name**: Unique identifier for the profile
- **db_type**: Database type - one of: `mysql`, `postgresql`, `sqlite`
- **host**: Database server hostname or IP address
- **port**: Database server port number
- **username**: Database username
- **password**: Database password (will be encrypted)
- **database_name**: Default database name
- **readonly**: Whether to restrict operations to read-only

#### Example Request
```json
{
  "jsonrpc": "2.0",
  "method": "configure-profile",
  "params": {
    "profile_name": "production_db",
    "db_type": "postgresql",
    "host": "localhost",
    "port": 5432,
    "username": "app_user",
    "password": "secure_password",
    "database_name": "myapp",
    "readonly": false
  },
  "id": "config_001"
}
```

#### Success Response
```json
{
  "jsonrpc": "2.0",
  "result": {
    "status": "success",
    "message": "Profile 'production_db' configured successfully"
  },
  "id": "config_001"
}
```

#### Error Responses
- `MISSING_PARAMETER`: Required parameter not provided
- `INVALID_DB_TYPE`: Unsupported database type
- `CONNECTION_FAILED`: Cannot connect to database with provided credentials

---

### 2. list-profiles

List all configured database profiles.

#### Parameters
None

#### Example Request
```json
{
  "jsonrpc": "2.0",
  "method": "list-profiles",
  "params": {},
  "id": "list_001"
}
```

#### Success Response
```json
{
  "jsonrpc": "2.0",
  "result": {
    "status": "success",
    "profiles": [
      {
        "profile_name": "production_db",
        "db_type": "postgresql"
      },
      {
        "profile_name": "analytics_db",
        "db_type": "mysql"
      }
    ]
  },
  "id": "list_001"
}
```

---

### 3. delete-profile

Delete a database connection profile.

#### Parameters
```json
{
  "profile_name": "string (required)"
}
```

Notes:
- `params` are positional values for prepared statements. BLOB/BINARY values must be base64-encoded strings.

#### Example Request
```json
{
  "jsonrpc": "2.0",
  "method": "delete-profile",
  "params": {
    "profile_name": "old_profile"
  },
  "id": "delete_001"
}
```

#### Success Response
```json
{
  "jsonrpc": "2.0",
  "result": {
    "status": "success",
    "message": "Profile 'old_profile' deleted successfully"
  },
  "id": "delete_001"
}
```

#### Error Responses
- `PROFILE_NOT_FOUND`: Specified profile does not exist

---

### 4. update-profile

Update an existing database profile.

#### Parameters
```json
{
  "profile_name": "string (required)",
  "host": "string (optional)",
  "port": "integer (optional)",
  "username": "string (optional)",
  "password": "string (optional)",
  "database_name": "string (optional)",
  "readonly": "boolean (optional)"
}
```

#### Example Request
```json
{
  "jsonrpc": "2.0",
  "method": "update-profile",
  "params": {
    "profile_name": "production_db",
    "readonly": true
  },
  "id": "update_001"
}
```

#### Success Response
```json
{
  "jsonrpc": "2.0",
  "result": {
    "status": "success",
    "message": "Profile 'production_db' updated successfully"
  },
  "id": "update_001"
}
```

---

### 5. execute-sql

Execute SQL queries on a database profile.

#### Parameters
```json
{
  "profile_name": "string (required)",
  "database_name": "string (required)",
  "sql": "string (required)",
  "params": ["string|number|boolean|null", "... optional"]
}
```

#### Parameter Details
- **profile_name**: Name of the profile to use
- **database_name**: Target database/schema (required)
- **sql**: SQL query to execute
- **params**: Optional positional parameters for prepared statements. BLOB/BINARY values must be base64-encoded strings.

#### Example Request
```json
{
  "jsonrpc": "2.0",
  "method": "execute-sql",
  "params": {
    "profile_name": "production_db",
    "database_name": "production_db",
    "sql": "SELECT id, name, email FROM users WHERE active = true LIMIT 10"
  },
  "id": "query_001"
}
```

#### Success Response (SELECT Query)
```json
{
  "jsonrpc": "2.0",
  "result": {
    "status": "success",
    "columns": ["id", "name", "email"],
    "rows": [
      [1, "John Doe", "john@example.com"],
      [2, "Jane Smith", "jane@example.com"]
    ],
    "row_count": 2,
    "execution_time_ms": 45
  },
  "id": "query_001"
}
```

#### Success Response (INSERT/UPDATE/DELETE)
```json
{
  "jsonrpc": "2.0",
  "result": {
    "status": "success",
    "rows_affected": 1,
    "execution_time_ms": 23
  },
  "id": "query_001"
}
```

#### Error Responses
- `PROFILE_NOT_FOUND`: Specified profile does not exist
- `SQL_SYNTAX_ERROR`: Invalid SQL syntax
- `PERMISSION_DENIED`: Insufficient database permissions
- `READONLY_VIOLATION`: Write operation on read-only profile
- `CONNECTION_FAILED`: Database connection failed

---

### 6. list-tables

List all tables and views in a database.

#### Parameters
```json
{
  "profile_name": "string (required)",
  "database_name": "string (optional)"
}
```

#### Example Request
```json
{
  "jsonrpc": "2.0",
  "method": "list-tables",
  "params": {
    "profile_name": "production_db"
  },
  "id": "tables_001"
}
```

#### Success Response
```json
{
  "jsonrpc": "2.0",
  "result": {
    "status": "success",
    "tables": [
      {
        "name": "users",
        "type": "TABLE",
        "row_count": 1250,
        "size_bytes": 2048576
      },
      {
        "name": "user_profiles",
        "type": "VIEW",
        "row_count": 1250,
        "size_bytes": 0
      }
    ],
    "total_count": 2
  },
  "id": "tables_001"
}
```

---

### 7. list-databases

List all databases/schemas available on the server.

#### Parameters
```json
{
  "profile_name": "string (required)"
}
```

#### Example Request
```json
{
  "jsonrpc": "2.0",
  "method": "list-databases",
  "params": {
    "profile_name": "production_db"
  },
  "id": "databases_001"
}
```

#### Success Response
```json
{
  "jsonrpc": "2.0",
  "result": {
    "status": "success",
    "databases": [
      {
        "name": "myapp",
        "owner": "app_user",
        "size_bytes": 104857600
      },
      {
        "name": "analytics",
        "owner": "analytics_user",
        "size_bytes": 52428800
      }
    ],
    "total_count": 2
  },
  "id": "databases_001"
}
```

---

### 8. describe-table

Get detailed schema information for a specific table.

#### Parameters
```json
{
  "profile_name": "string (required)",
  "table_name": "string (required)",
  "database_name": "string (optional)"
}
```

#### Example Request
```json
{
  "jsonrpc": "2.0",
  "method": "describe-table",
  "params": {
    "profile_name": "production_db",
    "table_name": "users"
  },
  "id": "describe_001"
}
```

#### Success Response
```json
{
  "jsonrpc": "2.0",
  "result": {
    "status": "success",
    "table_name": "users",
    "table_type": "TABLE",
    "columns": [
      {
        "name": "id",
        "type": "integer",
        "nullable": false,
        "key": "PRI",
        "default": null,
        "extra": "auto_increment",
        "max_length": null,
        "precision": 10,
        "scale": 0,
        "comment": "Primary key"
      },
      {
        "name": "email",
        "type": "varchar",
        "nullable": false,
        "key": "UNI",
        "default": null,
        "extra": "",
        "max_length": 255,
        "precision": null,
        "scale": 0,
        "comment": "User email address"
      }
    ],
    "indexes": [
      {
        "name": "PRIMARY",
        "columns": ["id"],
        "type": "PRIMARY",
        "unique": true
      },
      {
        "name": "idx_email",
        "columns": ["email"],
        "type": "UNIQUE",
        "unique": true
      }
    ]
  },
  "id": "describe_001"
}
```

---

### 9. analyze-schema

Perform comprehensive schema analysis with business context inference.

#### Parameters
```json
{
  "profile_name": "string (required)",
  "database_name": "string (optional)",
  "level": "string (optional, default: 'DETAILED')",
  "table_names": "array (optional)"
}
```

#### Parameter Details
- **level**: Analysis level - one of: `BASIC`, `DETAILED`, `COMPREHENSIVE`
- **table_names**: Specific tables to analyze (analyzes all if not provided)

#### Example Request
```json
{
  "jsonrpc": "2.0",
  "method": "analyze-schema",
  "params": {
    "profile_name": "production_db",
    "level": "COMPREHENSIVE",
    "table_names": ["users", "orders", "products"]
  },
  "id": "analyze_001"
}
```

#### Success Response
```json
{
  "jsonrpc": "2.0",
  "result": {
    "status": "success",
    "level": "COMPREHENSIVE",
    "database_name": "myapp",
    "analysis_timestamp": "2024-12-09T15:12:36Z",
    "tables_analyzed": 3,
    "business_context": {
      "domain": "ecommerce",
      "entity_types": ["users", "orders", "products"],
      "data_patterns": ["user_management", "order_processing", "inventory"]
    },
    "data_quality": {
      "completeness_score": 0.95,
      "consistency_score": 0.88,
      "validity_score": 0.92,
      "issues": [
        {
          "table": "orders",
          "column": "customer_email",
          "issue": "null_values",
          "percentage": 5.2
        }
      ]
    },
    "relationships": [
      {
        "from_table": "orders",
        "from_column": "user_id",
        "to_table": "users",
        "to_column": "id",
        "relationship_type": "foreign_key",
        "confidence": 0.95
      }
    ],
    "query_suggestions": [
      {
        "description": "Find customers with recent orders",
        "sql": "SELECT u.*, COUNT(o.id) as order_count FROM users u JOIN orders o ON u.id = o.user_id WHERE o.created_at > NOW() - INTERVAL '30 days' GROUP BY u.id",
        "complexity": "medium"
      }
    ]
  },
  "id": "analyze_001"
}
```

---

### 10. optimize-query

Run EXPLAIN for a statement, apply optimization rules, and return a performance estimate.

#### Parameters
```json
{
  "profile_name": "string (required)",
  "database_name": "string (required)",
  "sql": "string (required)",
  "params": ["string|number|boolean|null", "... optional"]
}
```

#### Example Request
```json
{
  "jsonrpc": "2.0",
  "method": "optimize-query",
  "params": {
    "profile_name": "analytics_db",
    "database_name": "analytics_db",
    "sql": "SELECT * FROM orders WHERE customer_id = ?",
    "params": [123]
  },
  "id": "optimize_001"
}
```

#### Success Response
```json
{
  "jsonrpc": "2.0",
  "result": {
    "plan": { "...": "normalized plan" },
    "findings": [
      {
        "rule": "missing_index",
        "severity": "warn",
        "message": "Full table scan with filter detected; an index may be missing.",
        "suggestion": "Add an index on the columns used in the filter for table orders"
      }
    ],
    "estimation": {
      "baseline_cost": 12.3,
      "baseline_rows": 1200,
      "confidence": 0.75,
      "improvement": {
        "lower_percent": 25,
        "upper_percent": 60,
        "confidence": 0.7,
        "explanation": "Indexing filter/join columns typically yields large gains."
      },
      "suggestions": [
        "Add an index on the columns used in the filter for table orders"
      ]
    },
    "summary": "Potential improvement: 25-60% (confidence 70%). Findings: 1."
  },
  "id": "optimize_001"
}
```

#### Error Responses
- Missing required parameters
- Profile not found
- Database connection/driver errors

---

### 11. sample-data

Fetch sample rows from a table for data inference.

#### Parameters
```json
{
  "profile_name": "string (required)",
  "table_name": "string (required)",
  "limit": "integer (optional, default: 10)",
  "database_name": "string (optional)"
}
```

#### Example Request
```json
{
  "jsonrpc": "2.0",
  "method": "sample-data",
  "params": {
    "profile_name": "production_db",
    "table_name": "users",
    "limit": 5
  },
  "id": "sample_001"
}
```

#### Success Response
```json
{
  "jsonrpc": "2.0",
  "result": {
    "status": "success",
    "table_name": "users",
    "columns": ["id", "name", "email", "created_at"],
    "rows": [
      [1, "John Doe", "john@example.com", "2024-01-15T10:30:00Z"],
      [2, "Jane Smith", "jane@example.com", "2024-01-16T14:22:00Z"],
      [3, "Bob Johnson", "bob@example.com", "2024-01-17T09:15:00Z"]
    ],
    "row_count": 3,
    "limit_used": 5,
    "total_rows": 1250,
    "data_inference": {
      "id": {"type": "integer", "pattern": "sequential"},
      "name": {"type": "string", "pattern": "full_name"},
      "email": {"type": "email", "pattern": "standard"},
      "created_at": {"type": "timestamp", "pattern": "iso8601"}
    }
  },
  "id": "sample_001"
}
```

---

### 12. discover-joins

Discover foreign key relationships and suggest JOIN operations.

#### Parameters
```json
{
  "profile_name": "string (required)",
  "tables": "array (optional)",
  "database_name": "string (optional)"
}
```

#### Example Request
```json
{
  "jsonrpc": "2.0",
  "method": "discover-joins",
  "params": {
    "profile_name": "production_db",
    "tables": ["users", "orders", "products"]
  },
  "id": "joins_001"
}
```

#### Success Response
```json
{
  "jsonrpc": "2.0",
  "result": {
    "status": "success",
    "relationships": [
      {
        "from_table": "orders",
        "from_column": "user_id",
        "to_table": "users",
        "to_column": "id",
        "relationship_type": "foreign_key",
        "confidence": 1.0,
        "join_suggestion": "JOIN users ON orders.user_id = users.id"
      },
      {
        "from_table": "orders",
        "from_column": "product_id",
        "to_table": "products",
        "to_column": "id",
        "relationship_type": "foreign_key",
        "confidence": 1.0,
        "join_suggestion": "JOIN products ON orders.product_id = products.id"
      }
    ],
    "complex_relationships": [
      {
        "description": "Many-to-many relationship between users and products through orders",
        "path": "users -> orders -> products",
        "suggested_query": "SELECT u.name, p.name, COUNT(o.id) as order_count FROM users u JOIN orders o ON u.id = o.user_id JOIN products p ON o.product_id = p.id GROUP BY u.id, p.id"
      }
    ],
    "tables_analyzed": 3
  },
  "id": "joins_001"
}
```

---

### 13. smart-query-builder

Generate SQL from natural language intent.

#### Parameters
```json
{
  "profile_name": "string (required)",
  "intent": "string (required)",
  "database_name": "string (optional)",
  "table_names": "array (optional)"
}
```

#### Example Request
```json
{
  "jsonrpc": "2.0",
  "method": "smart-query-builder",
  "params": {
    "profile_name": "production_db",
    "intent": "Find customers who placed orders in the last 30 days with total order value over $100"
  },
  "id": "smart_001"
}
```

#### Success Response
```json
{
  "jsonrpc": "2.0",
  "result": {
    "status": "success",
    "intent": "Find customers who placed orders in the last 30 days with total order value over $100",
    "generated_sql": "SELECT u.id, u.name, u.email, SUM(o.total_amount) as total_spent FROM users u JOIN orders o ON u.id = o.user_id WHERE o.created_at >= NOW() - INTERVAL '30 days' GROUP BY u.id, u.name, u.email HAVING SUM(o.total_amount) > 100 ORDER BY total_spent DESC",
    "explanation": "This query joins users with their orders, filters for recent orders within the last 30 days, calculates total spending per customer, and returns customers who have spent more than $100.",
    "tables_used": ["users", "orders"],
    "complexity": "medium",
    "estimated_rows": 150,
    "optimization_suggestions": [
      "Consider adding an index on orders.created_at for better performance",
      "Consider adding an index on orders.user_id for faster joins"
    ]
  },
  "id": "smart_001"
}
```

---

### 14. list-tools

List all available MCP tools with their specifications.

#### Parameters
None

#### Example Request
```json
{
  "jsonrpc": "2.0",
  "method": "list-tools",
  "params": {},
  "id": "tools_001"
}
```

#### Success Response
```json
{
  "jsonrpc": "2.0",
  "result": {
    "status": "success",
    "tools": [
      {
        "name": "configure-profile",
        "description": "Create or update a database connection profile",
        "parameters": {
          "type": "object",
          "properties": {
            "profile_name": {"type": "string"},
            "db_type": {"type": "string", "enum": ["mysql", "postgresql", "sqlite"]},
            "host": {"type": "string"},
            "port": {"type": "integer"},
            "username": {"type": "string"},
            "password": {"type": "string"},
            "database_name": {"type": "string"},
            "readonly": {"type": "boolean"}
          },
          "required": ["profile_name", "db_type", "host", "port", "username", "password", "database_name"]
        }
      },
      {
        "name": "execute-sql",
        "description": "Execute SQL queries on a database profile",
        "parameters": {
          "type": "object",
          "properties": {
            "profile_name": {"type": "string"},
            "sql_query": {"type": "string"},
            "database_name": {"type": "string"}
          },
          "required": ["profile_name", "sql_query"]
        }
      }
    ],
    "total_count": 15
  },
  "id": "tools_001"
}
```

---

### 15. mcp-info

Get MCP server information and capabilities.

#### Parameters
None

#### Example Request
```json
{
  "jsonrpc": "2.0",
  "method": "mcp-info",
  "params": {},
  "id": "info_001"
}
```

#### Success Response
```json
{
  "jsonrpc": "2.0",
  "result": {
    "status": "success",
    "server": {
      "name": "Database MCP Server",
      "version": "1.0.0",
      "description": "Unified database access via Model Context Protocol",
      "author": "guyinwonder"
    },
    "capabilities": {
      "supported_databases": ["mysql", "mariadb", "postgresql", "sqlite"],
      "features": [
        "connection_pooling",
        "encrypted_credentials",
        "read_only_mode",
        "schema_analysis",
        "query_building",
        "join_discovery"
      ],
      "protocol_version": "1.0.0",
      "transport": "stdio"
    },
    "configuration": {
      "max_pool_size": 25,
      "log_level": "info",
      "encryption_enabled": true
    }
  },
  "id": "info_001"
}
```

## Error Codes Reference

### Connection Errors
- `CONNECTION_FAILED`: Cannot establish database connection
- `TIMEOUT`: Query execution timeout
- `RESOURCE_EXHAUSTED`: Connection pool exhausted

### Configuration Errors
- `PROFILE_NOT_FOUND`: Specified profile does not exist
- `CONFIG_NOT_FOUND`: Configuration file missing
- `INVALID_CONFIGURATION`: Malformed configuration

### SQL Errors
- `SQL_SYNTAX_ERROR`: Invalid SQL syntax
- `TABLE_NOT_FOUND`: Table does not exist
- `COLUMN_NOT_FOUND`: Column does not exist
- `PERMISSION_DENIED`: Insufficient database permissions
- `READONLY_VIOLATION`: Write operation on read-only profile
- `CONSTRAINT_VIOLATION`: Database constraint violation
- `DATA_TYPE_MISMATCH`: Data type conflict

### Parameter Errors
- `MISSING_PARAMETER`: Required parameter not provided
- `INVALID_PARAMETER`: Parameter value invalid
- `UNSUPPORTED_DATABASE`: Database type not supported

### System Errors
- `INTERNAL_ERROR`: Unexpected internal error
- `ENCRYPTION_ERROR`: Credential encryption/decryption failed

## Usage Examples

### Complete Workflow Example

```bash
# 1. Configure a new profile
echo '{
  "jsonrpc": "2.0",
  "method": "configure-profile",
  "params": {
    "profile_name": "mydb",
    "db_type": "postgresql",
    "host": "localhost",
    "port": 5432,
    "username": "user",
    "password": "pass",
    "database_name": "app"
  },
  "id": "1"
}' | ./mcp-server

# 2. List tables
echo '{
  "jsonrpc": "2.0",
  "method": "list-tables",
  "params": {"profile_name": "mydb"},
  "id": "2"
}' | ./mcp-server

# 3. Execute query
echo '{
  "jsonrpc": "2.0",
  "method": "execute-sql",
  "params": {
    "profile_name": "mydb",
    "sql_query": "SELECT COUNT(*) FROM users"
  },
  "id": "3"
}' | ./mcp-server
```

### Integration with Kilocode AI

```yaml
mcp_providers:
  - name: database-mcp
    type: process
    command: /path/to/mcp-server
    working_dir: /path/to/config
    auto_start: true
```

## Performance Considerations

### Connection Pooling
- Default pool size: 25 connections
- Idle connections: 12 (half of max)
- Connection lifetime: 1 hour
- Idle timeout: 30 minutes

### Query Optimization
- Use LIMIT clauses for large result sets
- Consider using sample-data for data exploration
- Leverage analyze-schema for query optimization
- Use appropriate indexes for frequent queries

### Memory Usage
- Large result sets are streamed to reduce memory usage
- Connection pooling limits memory consumption
- Automatic garbage collection optimization

## Security Notes

### Credential Protection
- All passwords encrypted with AES-256-GCM
- 32-character encryption key required
- Keys stored in environment variables recommended
- No plaintext credentials in logs or memory

### Access Control
- Read-only profiles prevent write operations
- SQL injection prevention via parameterized queries
- Database-level permissions enforced
- Connection-level access control

## Troubleshooting

### Common Issues

#### Connection Failures
1. Verify database server is running
2. Check network connectivity
3. Validate credentials
4. Confirm database exists

#### Permission Errors
1. Check database user permissions
2. Verify read-only profile settings
3. Ensure database access rights

#### Performance Issues
1. Monitor connection pool usage
2. Check query execution times
3. Review database indexes
4. Consider query optimization

### Debug Information

Enable verbose logging for debugging:
```bash
./mcp-server --verbose
```

Check server information:
```bash
echo '{"method":"mcp-info"}' | ./mcp-server
```

## Future MCP Tools (Planned Enhancements)

### 16. validate-query

Validate SQL syntax, logic, and basic security patterns without execution.

#### Parameters
```json
{
  "profile_name": "string (required)",
  "database_name": "string (optional)",
  "sql": "string (required)",
  "params": ["string|number|boolean|null", "... optional"]
}
```

#### Parameter Details
- **profile_name**: Name of the profile to use for validation
- **database_name**: Override default database (optional)
- **sql**: SQL query to validate (not executed)
- **params**: Optional parameters for context (not executed). BLOB/BINARY values must be base64-encoded strings.

#### Example Request
```json
{
  "jsonrpc": "2.0",
  "method": "validate-query",
  "params": {
    "profile_name": "production_db",
    "sql": "SELECT * FROM users WHERE name = 'John' AND email = 'john@example.com'"
  },
  "id": "validate_001"
}
```

#### Success Response
```json
{
  "jsonrpc": "2.0",
  "result": {
    "is_valid": false,
    "issues": [
      {
        "rule": "syntax_error",
        "severity": "error",
        "message": "SQL syntax error: syntax error at position 63 near '\"'",
        "suggestion": "Review SQL syntax; ensure keywords and clauses are spelled correctly."
      },
      {
        "rule": "tautology",
        "severity": "warn",
        "message": "Potential tautology detected (OR 1=1).",
        "suggestion": "Use parameterized queries; avoid concatenating untrusted input."
      }
    ],
    "summary": "Validation failed.",
    "sql": "SELECT * FROM users WHERE name = 'John' AND email = 'john@example.com'",
    "profile_name": "production_db"
  },
  "id": "validate_001"
}
```

#### Error Responses
- Missing required parameters
- Profile not found

---

### 17. analyze-data-lineage

Trace data dependencies and impact relationships across the database ecosystem.

#### Parameters
```json
{
  "profile_name": "string (required)",
  "table_name": "string (required)",
  "analysis_scope": "string (optional, default: 'both')",
  "database_name": "string (optional)"
}
```

#### Parameter Details
- **profile_name**: Name of the profile to use for lineage analysis
- **table_name**: Target table for dependency analysis
- **analysis_scope**: Scope of analysis - one of: `upstream`, `downstream`, `both`
- **database_name**: Override default database

#### Example Request
```json
{
  "jsonrpc": "2.0",
  "method": "analyze-data-lineage",
  "params": {
    "profile_name": "production_db",
    "table_name": "orders",
    "analysis_scope": "both"
  },
  "id": "lineage_001"
}
```

#### Success Response
```json
{
  "jsonrpc": "2.0",
  "result": {
    "status": "success",
    "target_table": "orders",
    "analysis_scope": "both",
    "data_flow": {
      "upstream": [
        {
          "table": "customers",
          "relationship": "foreign_key",
          "columns": ["customer_id"],
          "impact_level": "High"
        }
      ],
      "downstream": [
        {
          "table": "order_items",
          "relationship": "foreign_key",
          "columns": ["order_id"],
          "impact_level": "Medium"
        }
      ]
    },
    "impact_analysis": {
      "change_impact": "High",
      "affected_tables": ["customers", "orders", "order_items", "inventory"],
      "critical_dependencies": ["customers.id", "products.id"],
      "change_propagation_time": "2-5 minutes"
    },
    "dependency_graph": {
      "nodes": ["customers", "orders", "order_items", "products", "inventory"],
      "edges": [
        {"from": "customers", "to": "orders", "type": "foreign_key"},
        {"from": "orders", "to": "order_items", "type": "foreign_key"},
        {"from": "products", "to": "order_items", "type": "foreign_key"}
      ]
    }
  },
  "id": "lineage_001"
}
```

#### Error Responses
- `TABLE_NOT_FOUND`: Specified table does not exist
- `LINEAGE_TOO_COMPLEX`: Dependency graph too complex to analyze

---

### 18. discover-insights

Automatically discover business insights, KPIs, trends, and anomalies from database data.

#### Parameters
```json
{
  "profile_name": "string (required)",
  "table_names": "array (optional)",
  "analysis_type": "string (required)",
  "database_name": "string (optional)"
}
```

#### Parameter Details
- **profile_name**: Name of the profile to use for insight discovery
- **table_names**: Specific tables to analyze (analyzes all if not provided)
- **analysis_type**: Type of analysis - one of: `kpi`, `trends`, `anomalies`, `correlations`
- **database_name**: Override default database

#### Example Request
```json
{
  "jsonrpc": "2.0",
  "method": "discover-insights",
  "params": {
    "profile_name": "production_db",
    "table_names": ["orders", "customers"],
    "analysis_type": "kpi"
  },
  "id": "insights_001"
}
```

#### Success Response
```json
{
  "jsonrpc": "2.0",
  "result": {
    "status": "success",
    "analysis_type": "kpi",
    "insights": [
      {
        "type": "kpi",
        "title": "Monthly Revenue Growth",
        "description": "Revenue has increased by 23% month-over-month with strong correlation to customer acquisition",
        "supporting_data": {
          "current_month": "$125,430",
          "previous_month": "$101,950",
          "growth_rate": "23.0%",
          "correlation_coefficient": 0.87
        },
        "visualization_suggestion": "line_chart",
        "business_impact": "High",
        "confidence": 0.92
      },
      {
        "type": "kpi",
        "title": "Customer Retention Rate",
        "description": "85% of customers make repeat purchases within 30 days, indicating strong loyalty",
        "supporting_data": {
          "retention_rate": "85%",
          "industry_average": "65%",
          "improvement": "+20 percentage points"
        },
        "visualization_suggestion": "gauge_chart",
        "business_impact": "Medium",
        "confidence": 0.88
      }
    ]
  },
  "id": "insights_001"
}
```

#### Error Responses
- `INSUFFICIENT_DATA`: Not enough data for reliable analysis
- `ANALYSIS_FAILED`: Insight discovery process failed

---

### 19. track-schema-changes

Track and analyze schema evolution over time with automated migration assistance.

#### Parameters
```json
{
  "profile_name": "string (required)",
  "operation": "string (optional, default: 'track')",
  "database_name": "string (optional)",
  "dialect": "string (optional)",
  "from_snapshot_id": "string (optional, required with to_snapshot_id for migration)",
  "to_snapshot_id": "string (optional, required with from_snapshot_id for migration)",
  "snapshot_id": "string (optional, baseline for drift)",
  "limit": "integer (optional, history only)",
  "retention_days": "integer (optional, track only)"
}
```

#### Parameter Details
- **profile_name**: Name of the profile to use for schema tracking
- **operation**: One of `track`, `history`, `generate_migration`, `detect_drift`
- **database_name**: Override default database
- **dialect**: SQL dialect for migration generation (`mysql`, `postgresql`, `sqlite`)
- **from_snapshot_id / to_snapshot_id**: Explicit snapshot pair for migration generation
- **snapshot_id**: Baseline snapshot ID for drift detection (defaults to latest)
- **limit**: Number of history snapshots to return
- **retention_days**: Snapshot retention window in days (default 30)

#### Example Request
```json
{
  "jsonrpc": "2.0",
  "method": "track-schema-changes",
  "params": {
    "profile_name": "production_db",
    "operation": "track",
    "retention_days": 30
  },
  "id": "schema_001"
}
```

#### Success Response
```json
{
  "jsonrpc": "2.0",
  "result": {
    "operation": "track",
    "profile_name": "production_db",
    "snapshot_id": "snap-1738863700123456789",
    "previous_snapshot_id": "snap-1738863600123456789",
    "changes": [
      {
        "type": "add_column",
        "table": "users",
        "column": "email",
        "impact": "compatible"
      }
    ],
    "migration": {
      "from_version": "snap-1738863600123456789",
      "to_version": "snap-1738863700123456789",
      "dialect": "sqlite",
      "statements": [
        "ALTER TABLE \"users\" ADD COLUMN \"email\" TEXT;"
      ],
      "estimated_time": "about 2 minute(s)",
      "is_reversible": true
    },
    "summary": "Schema changes tracked: 1 change(s) detected"
  },
  "id": "schema_001"
}
```

#### Error Responses
- `MISSING_PARAMETER`: `profile_name` is required
- `INVALID_INPUT`: Invalid operation or invalid snapshot arguments
- `PROFILE_NOT_FOUND`: Profile does not exist

---

### 20. federated-query

Execute queries across multiple database profiles with intelligent distributed execution.

#### Parameters
```json
{
  "profile_names": "array (required)",
  "query": "string (required)",
  "join_strategy": "string (optional, default: 'auto')",
  "result_limit": "integer (optional)"
}
```

#### Parameter Details
- **profile_names**: Array of profile names to include in federation
- **query**: SQL query to execute across databases
- **join_strategy**: Join execution strategy - one of: `local`, `remote`, `hybrid`, `auto`
- **result_limit**: Maximum number of rows to return

#### Example Request
```json
{
  "jsonrpc": "2.0",
  "method": "federated-query",
  "params": {
    "profile_names": ["analytics_db", "crm_db"],
    "query": "SELECT c.name, COUNT(o.id) as order_count FROM customers c LEFT JOIN analytics_db.orders o ON c.id = o.customer_id WHERE c.created_at >= '2024-01-01' GROUP BY c.id ORDER BY order_count DESC LIMIT 10",
    "join_strategy": "hybrid",
    "result_limit": 100
  },
  "id": "federated_001"
}
```

#### Success Response
```json
{
  "jsonrpc": "2.0",
  "result": {
    "status": "success",
    "query": "SELECT c.name, COUNT(o.id) as order_count FROM customers c LEFT JOIN analytics_db.orders o ON c.id = o.customer_id WHERE c.created_at >= '2024-01-01' GROUP BY c.id ORDER BY order_count DESC LIMIT 10",
    "join_strategy": "hybrid",
    "execution_plan": {
      "strategy": "Hybrid execution with local aggregation",
      "databases_involved": ["analytics_db", "crm_db"],
      "data_transfer_volume": "2.3MB"
    },
    "results": [
      ["Global Corp", 15],
      ["Tech Solutions", 12],
      ["Innovation Labs", 8]
    ],
    "performance_metrics": {
      "total_execution_time": "3.2s",
      "database_breakdown": {
        "crm_db": "0.8s",
        "analytics_db": "1.7s",
        "data_transfer": "0.7s"
      },
      "rows_returned": 3,
      "result_limit_reached": false
    }
  },
  "id": "federated_001"
}
```

#### Error Responses
- `FEDERATION_FAILED`: Cross-database query execution failed
- `PROFILE_UNAVAILABLE`: One or more specified profiles are not accessible
- `QUERY_TOO_COMPLEX`: Query too complex for federation

---
## Version History

### v1.0.0 (Current)
- Initial release with all 15 MCP tools (optimize-query included)
- Support for MySQL, MariaDB, PostgreSQL, SQLite
- AES-256-GCM credential encryption
- Connection pooling and performance optimization
- Comprehensive error handling and logging

### v1.1.0 (Planned - AI Enhancement Phase 1)
- **Query Validation**: `validate-query` tool for syntax and logic checking
- **Enhanced NLP**: Context-aware `smart-query-builder` with multi-turn support
- **Performance Improvements**: Advanced execution plan analysis

### v1.2.0 (Planned - AI Enhancement Phase 2)
- **Data Lineage**: `analyze-data-lineage` tool for dependency tracking
- **Business Intelligence**: `discover-insights` tool for KPI and trend analysis
- **Advanced Analytics**: Statistical analysis and anomaly detection
- **Impact Assessment**: Change impact prediction and analysis

### v1.3.0 (Planned - AI Enhancement Phase 3)
- **Schema Evolution**: `track-schema-changes` tool for change management
- **Advanced Profiling**: Enhanced `analyze-schema` with statistical analysis
- **Multi-DB Federation**: `federated-query` tool for cross-database operations
- **Enterprise Features**: Advanced data quality and governance capabilities

### Future Versions
- Additional database support (SQL Server, Oracle)
- GraphQL support
- Advanced monitoring and metrics
