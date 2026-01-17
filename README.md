# Database MCP Server

[![Go](https://img.shields.io/badge/Go-1.25.5%2B-00ADD8?logo=Go&logoColor=white)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](https://opensource.org/licenses/MIT)
[![Version](https://img.shields.io/badge/Version-v1.0.1-blue.svg)](https://github.com/guyinwonder168/database-mcp-server/releases/tag/v1.0.1)

A production-ready Model Context Protocol (MCP) provider for SQL databases, written in Go by guyinwonder. Supports MySQL, MariaDB, PostgreSQL, and SQLite. Features robust connection pooling, secure AES-GCM credential storage, structured JSON logging, comprehensive schema introspection, and a full suite of 12 MCP tools. Built and tested with Go 1.25.5.

## 🚀 Quick Start

```bash
# Clone the repository
git clone https://github.com/guyinwonder168/database-mcp-server.git
cd database-mcp-server

# Build the server
go build -o mcp-server ./cmd/server/main.go

# Run the server
./mcp-server
```

## 📋 Features

- 🔧 **Interactive Setup** - Auto-creates `config.yaml` if missing; all configuration is managed via MCP actions
- 👥 **Profile Management** - Add, update, and list database profiles via MCP
- ⚡ **SQL Execution** - Run arbitrary SQL queries (with read-only enforcement)
- 🔍 **Schema Introspection** - List tables/views, describe table schemas, list databases, and discover joins
- 📊 **Sample Data Fetching** - Fetch sample rows to infer data formats and value ranges
- 🔗 **Automated Join Discovery** - Suggest JOIN SQL for building complex queries
- 🚦 **Query Optimization** - EXPLAIN-based analysis with findings and performance estimates
- 🛡️ **Query Validation** - Syntax, logic, and security checks before execution
- 🤖 **Smart Query Builder** - Generate SQL queries programmatically (integrated into analyze-schema AIQuerySuggestions)
- 🔒 **Read-only Profiles** - Prevent write operations on selected profiles
- 🔐 **Secure Credentials** - Passwords are encrypted at rest using AES-GCM (256-bit)
- 🏊 **Connection Pooling** - Efficient, configurable pooling with max pool size
- 📝 **Structured Logging & Error Handling** - All actions and errors are logged as structured JSON; actionable error responses
- 🛠️ **Tool Discovery** - `list-tools` MCP action returns a machine-readable list of all available tools/actions
- 🔌 **Official MCP Protocol** - Communication via stdio (not HTTP server; JSON is exchanged over stdio via official Go MCP SDK)
- 🧭 **Data Lineage** - Analyze upstream/downstream dependencies via `analyze-data-lineage`

## 🛠️ Supported MCP Tools

| Tool | Description |
|-------|-------------|
| `configure-profile` | Create or update database connection profiles |
| `list-profiles` | List all configured database profiles |
| `execute-sql` | Execute arbitrary SQL queries with read-only enforcement |
| `list-tables` | List tables in selected database |
| `describe-table` | Describe comprehensive table schema with metadata |
| `list-databases` | List accessible databases for profile |
| `smart-query-builder` | Generate SQL from high-level intent |
| `optimize-query` | Run EXPLAIN, return plan, findings, and performance estimate |
| `validate-query` | Validate SQL syntax and flag risky patterns before execution |
| `analyze-data-lineage` | Trace FK-based upstream/downstream table dependencies |
| `discover-joins` | Discover foreign key relationships and suggest JOINs |
| `sample-data` | Fetch sample rows to infer data formats |
| `list-tools` | List all available MCP tools and descriptions |
| `analyze-schema` | Comprehensive schema analysis with AI query suggestions |
| `mcp-info` | Show provider version and author |

## 📖 Documentation

### Core Documentation
- 📋 [**API Documentation**](docs/api-documentation.md) - Detailed API specifications and examples
- 📊 [**Implementation Status**](docs/implementation-status.md) - Current implementation tracking
- 🏗️ [**Technical Specifications**](docs/technical-specifications.md) - Architecture and design details
- 📝 [**PRD Analysis**](docs/prd.md) - Product requirements analysis with AI perspective
- 🔍 [**Schema Introspection Queries**](docs/schema-introspection-queries.md) - Database-specific queries
- 🧪 [**Test Enhanced Schema**](docs/test-enhanced-schema.md) - Test schema documentation
- 🗺️ [**Implementation Roadmap**](docs/implementation-roadmap.md) - Strategic development planning

### Memory Bank Documentation
The project includes a comprehensive memory bank system for AI assistants, located in `.kilocode/rules/memory-bank/`:

- 🏗️ [**Architecture**](.kilocode/rules/memory-bank/architecture.md) - System architecture and component relationships
- 📋 [**Brief**](.kilocode/rules/memory-bank/brief.md) - Project overview and requirements
- 📊 [**Context**](.kilocode/rules/memory-bank/context.md) - Current state and recent changes
- 🎯 [**Product**](.kilocode/rules/memory-bank/product.md) - Problem statement and solution overview
- 💻 [**Tech**](.kilocode/rules/memory-bank/tech.md) - Technology stack and development setup

### Project Planning
- 🗺️ [**Roadmap**](project-plan/roadmap.md) - Comprehensive implementation strategy
- 📊 [**Vertical Slices**](project-plan/vertical-slices.md) - Phase-by-phase development breakdown
- 🔍 [**Architecture Validation**](project-plan/architecture-validation.md) - Technical compatibility analysis
- 📝 [**Implementation Tasks**](project-plan/implementation-tasks.md) - Detailed task tracking
- 🐛 [**MCP Tool Detection Fix**](project-plan/mcp-tool-detection-fix.md) - Critical bug fix documentation

### Version History
- 📋 [**CHANGELOG**](CHANGELOG.md) - Detailed release notes and version history

## 🤝 Contributing

We welcome contributions! Please see our [**Contributing Guidelines**](CONTRIBUTING.md) for development setup and workflow.

## 📄 License

This project is licensed under the [**MIT License**](LICENSE).

## 🔐 Security

For security policies and vulnerability reporting, please see our [**Security Policy**](SECURITY.md).

## 📜 Code of Conduct

Please read our [**Code of Conduct**](CODE_OF_CONDUCT.md) for community guidelines.

## 🧪 Testing

```bash
go test ./...
```

## 📊 Project Status

- **Version:** v1.0.1
- **Author:** guyinwonder
- **Status:** Production Ready ✅
- **All 12 MCP tools** are fully implemented and OpenAPI-aligned
- **Enhanced schema introspection** and sample data features
- **AES-GCM encryption**, connection pooling, and structured error handling
- **Comprehensive unit and integration tests** included

### Recent Enhancements (v1.0.1)

- 🧠 **Memory Bank System** - Added comprehensive AI assistant memory bank for project context preservation
- 📚 **Documentation Suite** - Complete documentation overhaul with API specs, implementation status, and technical specifications
- 📋 **Project Planning** - Detailed roadmap and implementation tracking documents
- 🧪 **Enhanced Testing** - Added integration tests and MCP tool discovery regression tests
- 📝 **Improved Logging** - Better credential redaction and structured error handling
- 🔌 **MCP Resources** - Added `tools://list` and `profile://{profile}` resources for read-only inspection
- 🌐 **SSE Transport** - Optional HTTP/SSE transport support for additional client compatibility
- 🔧 **Git Configuration** - Improved .gitignore to exclude logs and build artifacts

---

## 🤝 Enhancement Planning

**Current Development Status**: The Database MCP Server is production-ready with a comprehensive enhancement roadmap in progress.

### Implementation Phases
- **Phase 1** (Next 60 Days): Query optimization, validation, and enhanced NLP
- **Phase 2** (60-90 Days): Data lineage and business intelligence
- **Phase 3** (90+ Days): Schema evolution, advanced profiling, and multi-database federation

---

<div align="center">

**[⭐ Star this repository](https://github.com/guyinwonder168/database-mcp-server) if you find it useful!**

Made with ❤️ by [guyinwonder](https://github.com/guyinwonder168)

</div>

---

## Features

- **Interactive Setup:** Auto-creates `config.yaml` if missing; all configuration is managed via MCP actions.
- **Profile Management:** Add, update, and list database profiles via MCP.
- **SQL Execution:** Run arbitrary SQL queries (with read-only enforcement).
- **Schema Introspection:** List tables/views, describe table schemas, list databases, and discover joins.
- **Sample Data Fetching:** Fetch sample rows to infer data formats and value ranges.
- **Automated Join Discovery:** Suggest JOIN SQL for building complex queries.
- **Smart Query Builder:** Generate SQL queries programmatically (integrated into analyze-schema AIQuerySuggestions).
- **Read-only Profiles:** Prevent write operations on selected profiles.
- **Secure Credentials:** Passwords are encrypted at rest using AES-GCM (256-bit).
- **Connection Pooling:** Efficient, configurable pooling with max pool size.
- **Structured Logging & Error Handling:** All actions and errors are logged as structured JSON; actionable error responses.
- **Tool Discovery:** `list-tools` MCP action returns a machine-readable list of all available tools/actions.
- **Official MCP Protocol:** Communication via stdio (not HTTP server; JSON is exchanged over stdio via the official Go MCP SDK).

---

## Supported MCP Tools

- `configure-profile`
- `list-profiles`
- `execute-sql`
- `list-tables`
- `describe-table`
- `list-databases`
- `mcp-info`
- `smart-query-builder`
- `optimize-query`
- `validate-query`
- `discover-joins`
- `sample-data`
- `list-tools`
- `analyze-schema` (provides AIQuerySuggestions integrated with Smart Query Builder)

---

---

## Quick Start

1. **Set up encryption key (required for password encryption):**
   - On first run, `config.yaml` is auto-created with a secure random 32-character `aes_key`.
   - To customize, edit `config.yaml` and set `aes_key` to your own secure, random 32-character string.

2. **Build and run the server:**
   ```sh
   go build -o mcp-server ./cmd/server/main.go
   ./mcp-server
   ```

3. **Add database profiles:**
   - Use the `configure-profile` MCP tool (see examples below).

4. **Invoke MCP actions:**
   - Use any MCP-compatible client (e.g., Kilocode AI) to interact with the server.

---

### Step-by-Step: Invoking MCP Actions

**A. Using Kilocode AI**

1. Open Kilocode AI and ensure the Database MCP Server is running.
2. In Kilocode AI, open the MCP Tools panel.
3. Select the "database-mcp" provider from the list.
4. Choose a tool (e.g., `execute-sql`, `list-tables`, `configure-profile`).
5. Fill in the required parameters (such as `profile_name`, `sql`, etc.).
6. Click "Run" or "Execute" to send the action to the MCP server.
7. View the results directly in the Kilocode AI interface.

**B. Using JSON-RPC via Stdio (Script Example)**

The Database MCP Server communicates over stdio (process input/output), not HTTP. To invoke actions programmatically, use a script that launches the server and communicates via stdin/stdout.

Example (Python):

```python
import subprocess
import json

# Start the MCP server process
proc = subprocess.Popen(
    ['./mcp-server'],
    stdin=subprocess.PIPE,
    stdout=subprocess.PIPE,
    stderr=subprocess.PIPE,
    text=True
)

# Prepare a JSON-RPC request (e.g., list-profiles)
request = json.dumps({
    "method": "list-profiles",
    "params": {}
}) + '\n'

# Send request and read response
proc.stdin.write(request)
proc.stdin.flush()
response = proc.stdout.readline()
print("Response:", response)
```

- Replace `./mcp-server` with the path to your built binary if needed.
- You can send any MCP action in this way; parameter and response schemas are documented below and in [`docs/mcp-openapi.yaml`](docs/mcp-openapi.yaml).

The server will respond with a JSON object containing the results.

**C. Best Practices**

- Always verify the MCP server is running before sending actions.
- Use the `list-tools` action to discover available tools and their parameters.
- Refer to [`docs/mcp-openapi.yaml`](docs/mcp-openapi.yaml) for detailed schemas and examples.

---

## Development (Go 1.25.5)

- Run all tests: `go test ./...`
- MCP tool discovery regression note: The project uses `github.com/modelcontextprotocol/go-sdk v1.1.0`, which fixes the earlier `tools/list` rejection during session initialization (v0.2.0 regression). The guard test `internal/mcp/tools_list_integration_test.go` ensures all registered tools are returned by `ListTools`; downgrades will fail this test.
- Optional live DB integration tests: set env vars and run `go test ./internal/mcp -run TestLive -count=1`
  - Postgres: `DB_MCP_IT_PG_HOST`, `DB_MCP_IT_PG_PORT`, `DB_MCP_IT_PG_USER`, `DB_MCP_IT_PG_PASS`, `DB_MCP_IT_PG_DB`, `DB_MCP_IT_PG_SSLMODE` (e.g., disable for local containers)
  - MySQL/MariaDB: `DB_MCP_IT_MYSQL_HOST`, `DB_MCP_IT_MYSQL_PORT`, `DB_MCP_IT_MYSQL_USER`, `DB_MCP_IT_MYSQL_PASS`, `DB_MCP_IT_MYSQL_DB`
- Logging to stdout is disabled by default to avoid corrupting MCP stdio. To mirror logs to stdout, set `MCP_LOG_TO_STDOUT=true` when starting the server.
- Optional HTTP/SSE transport: set `MCP_SSE_ADDR` (e.g., `:8080`) to expose the MCP server over SSE in addition to stdio. Example:
  ```bash
  MCP_SSE_ADDR=":8080" ./mcp-server
  ```
  - Claude Desktop: add an MCP provider pointing to `http://localhost:8080` with transport `sse` (Claude auto-detects SSE when endpoint responds with SSE).
  - Codex CLI: set `MCP_SSE_ADDR` as above and add/update the provider entry to point at the HTTP endpoint if Codex supports SSE; otherwise keep stdio (default).
  - Kilocode: prefers stdio; leave `MCP_SSE_ADDR` unset unless testing SSE—Kilocode auto-uses the process command.

## Troubleshooting & Recovery

- **Invalid credentials**  
  - Symptoms: MCP calls such as `execute-sql` or `list-databases` return a structured error whose `message` includes the driver detail (e.g., MySQL “access denied for user”, Postgres “password authentication failed”).  
  - Fix: inspect the profile via `list-profiles` or `profile://{profile_name}` resource; rerun `configure-profile` with the corrected username/password/sslmode. For CI or local containers, ensure `DB_MCP_IT_*` env vars match the running DB.
  - Example structured error payload (MySQL bad password):
    ```json
    {
      "status": "error",
      "error_code": "CONNECTION_FAILED",
      "message": "Database connection failed",
      "details": "Access denied for user 'root'@'127.0.0.1' (using password: YES)",
      "suggestions": [
        {"action": "Check database server", "description": "Verify the database server is running and accessible"},
        {"action": "Verify connection details", "description": "Check host, port, username, and password in the profile configuration"},
        {"action": "Check network connectivity", "description": "Ensure there are no firewall or network issues blocking the connection"}
      ],
      "context": {
        "profile_name": "mysql_live",
        "db_type": "mysql",
        "operation": "connect",
        "database": "project"
      }
    }
    ```

- **Network drop / host unreachable**  
  - Symptoms: errors like “connection refused”, “EOF”, or “context deadline exceeded” when the DB container is stopped, the port is blocked, or DNS/host is wrong.  
  - Fix: confirm the DB process is up and listening (e.g., `podman ps`, `nc -z host port`), then rerun the MCP action; the server will open a fresh connection on each request. Set `MCP_LOG_TO_STDOUT=true` to surface the original driver error while debugging.
  - Example structured error payload (Postgres host down):
    ```json
    {
      "status": "error",
      "error_code": "CONNECTION_FAILED",
      "message": "Database connection failed",
      "details": "dial tcp 127.0.0.1:54320: connect: connection refused",
      "suggestions": [
        {"action": "Check database server", "description": "Verify the database server is running and accessible"},
        {"action": "Verify connection details", "description": "Check host, port, username, and password in the profile configuration"},
        {"action": "Check network connectivity", "description": "Ensure there are no firewall or network issues blocking the connection"}
      ],
      "context": {
        "profile_name": "pg_live",
        "db_type": "postgres",
        "operation": "connect",
        "database": "project"
      }
    }
    ```

- **Read-only enforcement**  
  - Symptoms: any write (`INSERT/UPDATE/DELETE/DDL`) on a profile with `readonly: true` returns the MCP error “profile is read-only” before the statement is sent to the database.  
  - Fix: perform writes via a non-readonly profile or flip `readonly: false` using `configure-profile`; verify with `list-profiles` or `profile://{profile_name}`. The guard path is covered by `TestHandleExecuteSQL_ParamAndReadonly`.
  - Example structured error payload (INSERT on readonly profile):
    ```json
    {
      "status": "error",
      "error_code": "SQL_EXECUTION_ERROR",
      "message": "Read-only profile restriction",
      "details": "Detected disallowed verb 'insert' in query for readonly profile",
      "suggestions": [
        {"action": "Use read-only queries", "description": "Only single SELECT/SHOW/DESCRIBE/EXPLAIN/PRAGMA queries are allowed", "example": "SELECT * FROM users LIMIT 50"},
        {"action": "Use a different profile", "description": "Switch to a profile that allows write operations"}
      ],
      "context": {
        "profile_name": "readonly_profile",
        "query": "INSERT INTO users(name) VALUES('alice')"
      }
    }
    ```

### MCP Resources

The server now exposes MCP resources (in addition to tools):

- `tools://list` — JSON array of all registered tools and descriptions.
- `profile://{profile_name}` — JSON metadata for a profile with secrets redacted.

Access these via your MCP client’s Resources panel (Codex/Kilocode). Tools remain the primary interaction surface; resources provide quick, read-only inspection.

## Release & Packaging

- Build release binary: `go build -o mcp-server ./cmd/server`
- Smoke run (stdout logging enabled): `MCP_LOG_TO_STDOUT=true ./mcp-server`
- Live DB smoke tests (podman/local): `DB_MCP_IT_PG_HOST=127.0.0.1 DB_MCP_IT_PG_PORT=54320 DB_MCP_IT_PG_USER=root DB_MCP_IT_PG_PASS=12345678 DB_MCP_IT_PG_DB=project DB_MCP_IT_MYSQL_HOST=127.0.0.1 DB_MCP_IT_MYSQL_PORT=33006 DB_MCP_IT_MYSQL_USER=root DB_MCP_IT_MYSQL_PASS='#12345678' DB_MCP_IT_MYSQL_DB=project go test ./internal/mcp -run Live -count=1`
- Update changelog (`CHANGELOG.md`) for the new version before tagging.
- Suggested tag for this release: `v1.0.1` (fixes MCP tool detection, adds structured error payload examples, and verifies live MySQL/Postgres connectivity).

## Comprehensive Configuration Examples

### config.yaml Structure

```yaml
max_pool_size: 10
aes_key: "<your-32-char-aes-key>"
profiles:
  - profile_name: "mariadb-profile"
    db_type: "mariadb"
    host: "localhost"
    port: 3306
    username: "mariauser"
    password: "<encrypted>"
    database_name: "mydb"
    readonly: false
  - profile_name: "mysql-profile"
    db_type: "mysql"
    host: "localhost"
    port: 3306
    username: "mysqluser"
    password: "<encrypted>"
    database_name: "mydb"
    readonly: true
  - profile_name: "postgres-profile"
    db_type: "postgres"
    host: "localhost"
    port: 5432
    username: "pguser"
    password: "<encrypted>"
    database_name: "mydb"
    sslmode: "require"   # Set to verify-full in production with proper certificates
    readonly: false
  - profile_name: "sqlite-profile"
    db_type: "sqlite"
    database_name: "./data/mydb.sqlite"
    readonly: true
```

#### MariaDB Example

```yaml
- profile_name: "mariadb-prod"
  db_type: "mariadb"
  host: "db.example.com"
  port: 3306
  username: "admin"
  password: "<encrypted>"
  database_name: "production"
  readonly: false
```

#### MySQL Example

```yaml
- profile_name: "mysql-dev"
  db_type: "mysql"
  host: "localhost"
  port: 3306
  username: "devuser"
  password: "<encrypted>"
  database_name: "devdb"
  readonly: true
```

#### PostgreSQL Example

```yaml
- profile_name: "pg-main"
  db_type: "postgres"
  host: "localhost"
  port: 5432
  username: "pguser"
  password: "<encrypted>"
  database_name: "main"
  sslmode: "require"   # Options: disable | require | verify-ca | verify-full (recommended: verify-full in production)
  readonly: false
```

#### SQLite Example

```yaml
- profile_name: "sqlite-local"
  db_type: "sqlite"
  database_file: "./data/local.sqlite"
  readonly: true
```

**Notes:**
- `password` fields must be encrypted using the server's AES key (handled automatically by MCP actions).
- For SQLite, only `database_name` (file path or :memory:) and `readonly` are required.

---

## Usage Examples

### configure-profile

Create or update a database connection profile.

Parameters:
- profile_name (string, required)
- db_type (string, required): "mysql" | "mariadb" | "postgres" | "sqlite"
- host (string, optional; required for non-sqlite)
- port (number, optional; required for non-sqlite)
- username (string, optional; required for non-sqlite)
- password (string, optional; required for non-sqlite)
- database_name (string, required; for sqlite this is a file path like ./data.db or :memory:)
- readonly (boolean, required)
- sslmode (string, optional; Postgres only: disable | require | verify-ca | verify-full; defaults to require; use verify-full in production with proper CA/cert validation)

Request example:
```json
{
  "method": "configure-profile",
  "params": {
    "profile_name": "pg-main",
    "db_type": "postgres",
    "host": "localhost",
    "port": 5432,
    "username": "pguser",
    "password": "mypassword",
    "database_name": "main",
    "readonly": false
  }
}
```

Success response:
```text
Profile configured successfully.
```

Error response: Structured JSON with error_code and suggestions (see Error Handling section).

### execute-sql

Execute an arbitrary SQL query or statement.

Parameters:
- profile_name (string, required)
- database_name (string, required)
- sql (string, required)
- params (array, optional): positional parameters for prepared queries (use ? placeholders)

Readonly enforcement:
- When the selected profile is readonly, only SELECT, SHOW, DESCRIBE, EXPLAIN, PRAGMA are allowed. Others are blocked with a structured error.

Database context:
- MySQL/MariaDB: issues USE database_name when the provided database differs from the profile default.
- Postgres/SQLite: DSN is built for the provided database.

Request example (query with params):
```json
{
  "method": "execute-sql",
  "params": {
    "profile_name": "pg-main",
    "database_name": "main",
    "sql": "SELECT id, name, age FROM users WHERE age > ? LIMIT 10",
    "params": [21]
  }
}
```

Success response (query):
```json
{
  "columns": ["id", "name", "age"],
  "rows": [
    [1, "Alice", 25],
    [2, "Bob", 32]
  ]
}
```

Success response (non-query):
```json
{
  "affected": 3
}
```

Error response: Structured JSON with error_code and suggestions.
### analyze-schema

```json
{
  "method": "analyze-schema",
  "params": {
    "profile_name": "pg-main",
    "database_name": "main",
    "analysis_level": "detailed",
    "include_tables": ["users", "orders"],
    "sample_size": 10,
    "include_queries": true
  }
}
```

Notes:
- analysis_level: "basic" | "detailed" | "comprehensive"
- include_tables/exclude_tables: filter scope of analysis
- include_queries: when true, response includes AIQuerySuggestions powered by Smart Query Builder

**Response (abridged):**
```json
{
  "analysis_metadata": {
    "analysis_level": "detailed",
    "database_type": "postgres",
    "analysis_timestamp": "2025-08-06T10:00:00Z"
  },
  "database_overview": {
    "total_tables": 2,
    "total_columns": 7
  },
  "table_schemas": {
    "users": {
      "column_count": 2,
      "key_columns": {
        "primary_key": "id",
        "foreign_keys": []
      },
      "columns": [
        {"column_name": "id", "data_type": "INT", "is_nullable": false, "is_primary_key": true},
        {"column_name": "name", "data_type": "VARCHAR(255)", "is_nullable": false}
      ]
    },
    "orders": {
      "column_count": 3,
      "key_columns": {
        "primary_key": "order_id",
        "foreign_keys": ["user_id"]
      },
      "columns": [
        {"column_name": "order_id", "data_type": "INT", "is_nullable": false, "is_primary_key": true},
        {"column_name": "user_id", "data_type": "INT", "is_nullable": false, "is_foreign_key": true, "foreign_key_ref": {"ref_table": "users", "ref_column": "id"}},
        {"column_name": "total", "data_type": "DECIMAL(10,2)", "is_nullable": false}
      ]
    }
  },
  "relationship_graph": {
    "foreign_keys": [
      {
        "from_table": "orders",
        "from_column": "user_id",
        "to_table": "users",
        "to_column": "id",
        "relationship_type": "many_to_one",
        "suggested_join": "JOIN users u ON orders.user_id = u.id"
      }
    ]
  },
  "ai_query_suggestions": {
    "data_exploration": [
      {
        "question": "Top 10 recent orders with customer name",
        "sql": "SELECT o.order_id, u.name, o.total FROM orders o JOIN users u ON o.user_id = u.id ORDER BY o.order_id DESC LIMIT 10"
      }
    ],
    "business_intelligence": [
      {
        "question": "Monthly revenue trend",
        "sql": "SELECT date_trunc('month', created_at) AS month, SUM(total) AS revenue FROM orders GROUP BY 1 ORDER BY 1"
      }
    ]
  }
}
```
```

### list-profiles

List configured profiles.

Request:
```json
{
  "method": "list-profiles",
  "params": {}
}
```

Response:
```json
{
  "profiles": [
    { "profile_name": "pg-main", "db_type": "postgres" },
    { "profile_name": "sqlite-local", "db_type": "sqlite" }
  ]
}
```

### list-tables

List tables in a database/schema.

Parameters:
- profile_name (string, required)
- database_name (string, required)

Request:
```json
{
  "method": "list-tables",
  "params": {
    "profile_name": "pg-main",
    "database_name": "main"
  }
}
```

Response:
```json
{
  "tables": ["users", "orders", "products"]
}
```

### describe-table

Describe table columns and metadata.

Parameters:
- profile_name (string, required)
- database_name (string, required)
- table_name (string, required)

Request:
```json
{
  "method": "describe-table",
  "params": {
    "profile_name": "pg-main",
    "database_name": "main",
    "table_name": "users"
  }
}
```

Response (example shape):
```json
{
  "columns": [
    {
      "name": "id",
      "type": "integer",
      "nullable": false,
      "key": "PRI",
      "default": null,
      "comment": "",
      "extra": "",
      "character_set": "",
      "collation": "",
      "auto_increment": true,
      "max_length": null,
      "precision": 32,
      "scale": 0
    }
  ]
}
```

### list-databases

List databases accessible to the profile.

Parameters:
- profile_name (string, required)

Request:
```json
{
  "method": "list-databases",
  "params": {
    "profile_name": "pg-main"
  }
}
```

Response:
```json
{
  "databases": ["main", "analytics", "test"]
}
```
Note: For sqlite, the response will include only the configured database_name.

### sample-data

Fetch sample rows from a table.

Parameters:
- profile_name (string, required)
- database_name (string, required)
- table_name (string, required)
- sample_size (number, optional; default 3; max 100)

Request:
```json
{
  "method": "sample-data",
  "params": {
    "profile_name": "mysql-dev",
    "database_name": "devdb",
    "table_name": "orders",
    "sample_size": 5
  }
}
```

Response:
```json
{
  "table_name": "orders",
  "sample_size": 5,
  "columns": ["id", "user_id", "total"],
  "sample_rows": [
    [1, 10, "99.50"],
    [2, 11, "150.00"]
  ],
  "summary": "Retrieved 2 sample row(s) from table 'orders' with 3 column(s)."
}
```

### discover-joins

Discover foreign key relationships and suggest JOIN SQL.

Parameters:
- profile_name (string, required)
- tables (array of string, optional): restrict discovery to these tables

Request:
```json
{
  "method": "discover-joins",
  "params": {
    "profile_name": "analytics_db",
    "tables": ["orders", "customers"]
  }
}
```

Response:
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
    }
  ],
  "summary": "Discovered 1 join(s) based on foreign key relationships."
}
```

### smart-query-builder

Generate SQL from high-level intent and optional table hints.

Parameters:
- profile_name (string, required)
- intent (string, required)
- database_name (string, optional)
- table_names (array of string, optional)

Request:
```json
{
  "method": "smart-query-builder",
  "params": {
    "profile_name": "pg-main",
    "intent": "attendance dashboard",
    "database_name": "main",
    "table_names": ["attendance"]
  }
}
```

Response:
```json
{
  "sql": "SELECT id, employee_id, status FROM attendance;",
  "explanation": "Selected table 'attendance' and columns [id, employee_id, status] based on keywords from intent 'attendance dashboard'."
}
```

### mcp-info

Show provider version and author.

Request:
```json
{
  "method": "mcp-info",
  "params": {}
}
```

Response (text):
```
Database MCP Provider
Author: guyinwonder
Version: v1.0.0
Created using OpenAI GPT-4.1 via VSCode Kilocode AI code assistant extension.
```

### list-tools

List all available tools and descriptions.

Request:
```json
{
  "method": "list-tools",
  "params": {}
}
```

Response:
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

### Listing Tools

```json
{
  "method": "list-tools"
}
```

---

## Installation & Setup

### Prerequisites

- Go 1.23+ installed (`go version`)
- Supported databases: MySQL, MariaDB, PostgreSQL, SQLite

### Build & Run

```sh
git clone <repo-url>
cd database-mcp-provider
go mod download
go build -o mcp-server ./cmd/server/main.go
./mcp-server
```

- The server runs as a stdio-based MCP provider (no HTTP server or port).
- On first run, if `config.yaml` is missing, a minimal config with a secure random AES key is auto-created.
- All configuration is managed via MCP actions (no interactive CLI prompts).
- To add database profiles, use the `configure-profile` MCP action.

---

## Best Practices

- Use strong, random `aes_key` for encryption.
- Set `readonly: true` for profiles used by AI/agents to prevent accidental writes.
- Monitor `mcp-provider.log` for errors and audit events.
- Use connection pooling (`max_pool_size`) appropriate for your workload.
- Never store plaintext passwords; always use MCP actions to manage credentials.
- Regularly update dependencies and run tests (`go test ./...`).

---

## Security

- Passwords are encrypted at rest with AES-GCM (256-bit) using a 32-character key (auto-generated if absent).
- Logger now performs automatic credential redaction (passwords, tokens, keys, DSN inline secrets) to prevent accidental leakage in logs.
- No plaintext credentials are stored or logged; only redacted placeholders (***REDACTED***) appear for sensitive fields.
- All operations use structured JSON logging for auditability; error analysis produces actionable suggestions.
- Read-only profiles enforce access control (write / DDL / unsafe multi-statement queries blocked; WITH + SELECT allowed).
- PostgreSQL profiles default to `sslmode=require` (encryption in transit). For production, use:
  - `sslmode=verify-full` (recommended): validates certificate chain AND hostname
  - `sslmode=verify-ca`: validates CA chain only (use only when hostname match cannot be satisfied)
  - Avoid `disable` except in isolated, disposable dev environments
- Certificate Guidance (PostgreSQL):
  - Server certificate & key managed on the database host (standard PostgreSQL configuration).
  - Client validation paths (lib/pq) typically honor environment variables:
    - `SSLROOTCERT` (root CA), `SSLCERT` (client cert), `SSLKEY` (client key)
  - Alternatively place certificates in a secure directory (e.g. `/etc/db_certs/`) with 600 permissions for private keys.
  - Rotate certificates periodically; monitor expiration with CI checks.
- Operational Hardening:
  - Restrict filesystem permissions on `config.yaml` (e.g. `chmod 600`).
  - Do not commit `config.yaml` to version control; add to `.gitignore`.
  - Rotate AES key if compromise suspected (requires re-encrypting stored passwords via re-saving profiles).
  - Prefer separate least-privilege database users for readonly vs write profiles.
- Threat Mitigations:
  - SQL injection minimized by prepared statements path (`params` usage).
  - Multi-statement execution blocked for readonly profiles.
  - Excessive quality issue lists capped to reduce potential data leakage surface.
- Future (optional) Enhancements (not yet implemented):
  - Support for explicit client cert/key fields in profile for managed rotation workflows.
  - Pluggable secret manager integration (e.g., environment / vault) instead of storing AES key in config.

---

## Kilocode AI Integration

This provider is fully compatible with Kilocode AI's MCP integration.

**Example Kilocode AI config snippet:**
```yaml
mcp_providers:
  - name: database-mcp
    command: "/[path to ]/mcp-server"
    workingDirectory: "/[path to ]"
    args: []
    disabled: false
    alwaysAllow:
      - "list-profiles"
```
- `command` is the full path to your built mcp-server binary.
- `workingDirectory` is the path where the binary is located.

---

## Documentation

### Core Documentation
- All MCP actions and usage examples are documented in [`docs/mcp-openapi.yaml`](docs/mcp-openapi.yaml).
- The `list-tools` MCP action provides a machine-readable list of all available tools/actions.
- No HTTP endpoints or web server are provided; all communication is via stdio MCP protocol.

### Comprehensive Documentation Suite
- **API Documentation**: [`docs/api-documentation.md`](docs/api-documentation.md) - Detailed API specifications and examples
- **Implementation Status**: [`docs/implementation-status.md`](docs/implementation-status.md) - Current implementation tracking
- **Technical Specifications**: [`docs/technical-specifications.md`](docs/technical-specifications.md) - Architecture and design details
- **PRD Analysis**: [`docs/prd.md`](docs/prd.md) - Product requirements analysis with AI perspective
- **Schema Introspection Queries**: [`docs/schema-introspection-queries.md`](docs/schema-introspection-queries.md) - Database-specific queries
- **Test Enhanced Schema**: [`docs/test-enhanced-schema.md`](docs/test-enhanced-schema.md) - Test schema documentation
- **Implementation Roadmap**: [`docs/implementation-roadmap.md`](docs/implementation-roadmap.md) - Strategic development planning

### GitHub Project Documentation
- **Contributing Guidelines**: [`CONTRIBUTING.md`](CONTRIBUTING.md) - Development setup and contribution workflow
- **Code of Conduct**: [`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md) - Community guidelines and behavior standards
- **Security Policy**: [`SECURITY.md`](SECURITY.md) - Security practices and vulnerability reporting
- **Issue Templates**: Bug reports and feature request templates in [`.github/ISSUE_TEMPLATE/`](.github/ISSUE_TEMPLATE/)
- **Pull Request Template**: [`.github/PULL_REQUEST_TEMPLATE.md`](.github/PULL_REQUEST_TEMPLATE.md) - Comprehensive PR checklist
- **License**: [`LICENSE`](LICENSE) - MIT license for open source use

### Memory Bank Documentation
The project includes a comprehensive memory bank system for AI assistants, located in `.kilocode/rules/memory-bank/`:
- **Architecture**: [`memory-bank/architecture.md`](.kilocode/rules/memory-bank/architecture.md) - System architecture and component relationships
- **Brief**: [`memory-bank/brief.md`](.kilocode/rules/memory-bank/brief.md) - Project overview and requirements
- **Context**: [`memory-bank/context.md`](.kilocode/rules/memory-bank/context.md) - Current state and recent changes
- **Product**: [`memory-bank/product.md`](.kilocode/rules/memory-bank/product.md) - Problem statement and solution overview
- **Tech**: [`memory-bank/tech.md`](.kilocode/rules/memory-bank/tech.md) - Technology stack and development setup

### Project Planning
- **Roadmap**: [`project-plan/roadmap.md`](project-plan/roadmap.md) - Comprehensive implementation strategy
- **Vertical Slices**: [`project-plan/vertical-slices.md`](project-plan/vertical-slices.md) - Phase-by-phase development breakdown
- **Architecture Validation**: [`project-plan/architecture-validation.md`](project-plan/architecture-validation.md) - Technical compatibility analysis
- **Implementation Tasks**: [`project-plan/implementation-tasks.md`](project-plan/implementation-tasks.md) - Detailed task tracking
- **MCP Tool Detection Fix**: [`project-plan/mcp-tool-detection-fix.md`](project-plan/mcp-tool-detection-fix.md) - Critical bug fix documentation

### Version History
- **CHANGELOG**: [`CHANGELOG.md`](CHANGELOG.md) - Detailed release notes and version history

---

## Testing

```sh
go test ./...
```

---

## Project Status

- **Version:** v1.0.1
- **Author:** guyinwonder
- All 12 MCP tools are fully implemented and OpenAPI-aligned.
- Enhanced schema introspection and sample data features.
- AES-GCM encryption, connection pooling, and structured error handling are enforced.
- Comprehensive unit and integration tests included.
- Ready for production use.

### Recent Enhancements (v1.0.1)
- **Memory Bank System**: Added comprehensive AI assistant memory bank for project context preservation
- **Documentation Suite**: Complete documentation overhaul with API specs, implementation status, and technical specifications
- **Project Planning**: Detailed roadmap and implementation tracking documents
- **Enhanced Testing**: Added integration tests and MCP tool discovery regression tests
- **Improved Logging**: Better credential redaction and structured error handling
- **MCP Resources**: Added `tools://list` and `profile://{profile}` resources for read-only inspection
- **SSE Transport**: Optional HTTP/SSE transport support for additional client compatibility
- **Git Configuration**: Improved .gitignore to exclude logs and build artifacts

---

## License

MIT

## Enhancement Planning

**Current Development Status**: The Database MCP Server is production-ready with a comprehensive enhancement roadmap in progress.

**Implementation Phases**:
- **Phase 1** (Next 60 Days): Query optimization, validation, and enhanced NLP
- **Phase 2** (60-90 Days): Data lineage and business intelligence
- **Phase 3** (90+ Days): Schema evolution, advanced profiling, and multi-database federation

**Planning Documents**:
- [Implementation Roadmap](docs/implementation-roadmap.md) - Strategic overview
- [Project Plan](../project-plan/roadmap.md) - Comprehensive implementation strategy
- [Vertical Slices](../project-plan/vertical-slices.md) - Detailed phase breakdowns
- [Architecture Validation](../project-plan/architecture-validation.md) - Technical compatibility analysis

**Ready for immediate enhancement development while maintaining production stability.**
