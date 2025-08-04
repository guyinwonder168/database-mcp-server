<!--
Author: guyinwonder
Project created using OpenAI GPT-4.1 via VSCode Kilo code AI code assistant extension.
Version v1.0.0
-->

# Database MCP Server

A production-ready Model Context Protocol (MCP) provider for SQL databases, written in Go by guyinwonder. Supports MySQL, MariaDB, PostgreSQL, and SQLite. Features robust connection pooling, secure AES-GCM credential storage, structured JSON logging, comprehensive schema introspection, and a full suite of 11 MCP tools.

---

## Features

- **Interactive Setup:** Auto-creates `config.yaml` if missing; all configuration is managed via MCP actions.
- **Profile Management:** Add, update, and list database profiles via MCP.
- **SQL Execution:** Run arbitrary SQL queries (with read-only enforcement).
- **Schema Introspection:** List tables/views, describe table schemas, list databases, and discover joins.
- **Sample Data Fetching:** Fetch sample rows to infer data formats and value ranges.
- **Automated Join Discovery:** Suggest JOIN SQL for building complex queries.
- **Smart Query Builder:** Generate SQL queries programmatically.
- **Read-only Profiles:** Prevent write operations on selected profiles.
- **Secure Credentials:** Passwords are encrypted at rest using AES-GCM (256-bit).
- **Connection Pooling:** Efficient, configurable pooling with max pool size.
- **Structured Logging & Error Handling:** All actions and errors are logged as structured JSON; actionable error responses.
- **Tool Discovery:** `list-tools` MCP action returns a machine-readable list of all available tools/actions.
- **Official MCP Protocol:** Communication via stdio (not HTTP/JSON-RPC).

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
- `discover-joins`
- `sample-data`
- `list-tools`

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
- You can send any MCP action in this way; see [`docs/mcp-openapi.yaml`](docs/mcp-openapi.yaml) for schemas.

The server will respond with a JSON object containing the results.

**C. Best Practices**

- Always verify the MCP server is running before sending actions.
- Use the `list-tools` action to discover available tools and their parameters.
- Refer to [`docs/mcp-openapi.yaml`](docs/mcp-openapi.yaml) for detailed schemas and examples.

---

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
- For SQLite, only `database_name` and `readonly` are required.

---

## Usage Examples

### Adding a Profile via MCP

Use the `configure-profile` MCP tool with parameters matching your database type. Example (JSON-RPC):

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
> The server will encrypt the password and update `config.yaml` automatically.

### Executing SQL

```json
{
  "method": "execute-sql",
  "params": {
    "profile_name": "pg-main",
    "sql": "SELECT * FROM users LIMIT 10"
  }
}
```

### Fetching Sample Data

```json
{
  "method": "sample-data",
  "params": {
    "profile_name": "mysql-dev",
    "database_name": "devdb",
    "table_name": "orders",
    "limit": 5
  }
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

- Passwords are encrypted with AES-GCM (256-bit).
- No plaintext credentials are stored or logged.
- All sensitive operations are logged in structured JSON format.
- Read-only profiles enforce access control for sensitive actions.

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

- All MCP actions and usage examples are documented in [`docs/mcp-openapi.yaml`](docs/mcp-openapi.yaml).
- The `list-tools` MCP action provides a machine-readable list of all available tools/actions.
- No HTTP endpoints or web server are provided; all communication is via stdio/JSON-RPC MCP protocol.

---

## Testing

```sh
go test ./...
```

---

## Project Status

- **Version:** v1.0.0
- **Author:** guyinwonder
- All 11 MCP tools are fully implemented and OpenAPI-aligned.
- Enhanced schema introspection and sample data features.
- AES-GCM encryption, connection pooling, and structured error handling are enforced.
- Comprehensive unit and integration tests included.
- Ready for production use.

---

## License

MIT