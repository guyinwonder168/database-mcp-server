# Database MCP Server - Technology Stack

## Core Technologies

### Programming Language
- **Go (Golang)** - Version 1.25.5
  - Chosen for performance, strong typing, and excellent concurrency support
  - Toolchain: go1.25.5 (gvm default; 1.25.4 removed)

### MCP Framework
- **Official Go MCP SDK** (github.com/modelcontextprotocol/go-sdk v1.2.0)
  - Model Context Protocol implementation
  - Stdio transport for local communication
  - JSON-RPC protocol support
  - Upgraded to v1.2.0 for latest features and stability

### Database Drivers
- **MySQL/MariaDB**: github.com/go-sql-driver/mysql v1.9.3
- **PostgreSQL**: github.com/lib/pq v1.10.9
- **SQLite**: github.com/mattn/go-sqlite3 v1.14.30
- All use standard `database/sql` interface

### Supporting Libraries
- **YAML Configuration**: gopkg.in/yaml.v3 v3.0.1
- **Log Rotation**: github.com/lestrrat-go/file-rotatelogs v2.4.0+incompatible
- **Encryption**: Standard library crypto/aes for AES-GCM
- **Additional Dependencies**:
  - filippo.io/edwards25519 v1.1.0 // indirect
  - github.com/jonboulle/clockwork v0.5.0 // indirect
  - github.com/lestrrat-go/strftime v1.1.1 // indirect
  - github.com/pkg/errors v0.9.1 // indirect
  - github.com/yosida95/uritemplate/v3 v3.0.2 // indirect
  - github.com/blastrain/vitess-sqlparser v0.0.0-20201030050434-a139afbb1aba (SQL validation)

## Development Setup

### Prerequisites
```bash
# Go 1.25.5
go version

# Git for version control
git --version
```

### Build Instructions
```bash
# Clone repository
git clone [repository-url]
cd database-mcp-provider

# Download dependencies
go mod download

# Build the server
go build -o mcp-server ./cmd/server/main.go

# Run tests
go test ./...
```

### Configuration
1. **First Run**: Interactive CLI wizard guides through setup
2. **Manual Configuration**: Edit `config.yaml`
   ```yaml
   max_pool_size: 10
   aes_key: "<32-character-string>"
   profiles:
     - profile_name: "mydb"
       db_type: "postgres"
       host: "localhost"
       port: 5432
       username: "user"
       password: "<encrypted>"
       database_name: "database"
       readonly: false
   ```
   - Configuration cleanup: obsolete user_key/user_secret fields have been removed for security and clarity.
   - All database profile examples (MySQL, MariaDB, PostgreSQL, SQLite) are included in the documentation.

### Environment Variables
- **DB_MCP_AES_KEY**: Optional, overrides config file AES key
- **MCP_LOG_TO_STDOUT**: Set to `true` to mirror logs to stdout (defaults to file-only to keep MCP stdio clean)
- Live DB smoke tests:
  - Postgres: `DB_MCP_IT_PG_HOST`, `DB_MCP_IT_PG_PORT`, `DB_MCP_IT_PG_USER`, `DB_MCP_IT_PG_PASS`, `DB_MCP_IT_PG_DB`, `DB_MCP_IT_PG_SSLMODE`
  - MySQL/MariaDB: `DB_MCP_IT_MYSQL_HOST`, `DB_MCP_IT_MYSQL_PORT`, `DB_MCP_IT_MYSQL_USER`, `DB_MCP_IT_MYSQL_PASS`, `DB_MCP_IT_MYSQL_DB`
- Standard database environment variables respected by drivers
- **MCP_SSE_ADDR**: Optional; set to host:port (e.g., `:8080`) to serve MCP over HTTP/SSE alongside stdio. Example: `MCP_SSE_ADDR=":8080" ./mcp-server`

## Technical Constraints

### Security
- AES-256-GCM encryption mandatory for passwords
- No plaintext credentials in logs or config
- SQL injection prevention via parameterized queries

### Performance
- Connection pooling with configurable limits
- Stateless design for scalability
- JSON structured logging for efficient parsing

### Compatibility
- Works with MCP-compatible clients (Kilocode AI, etc.)
- Database version requirements:
  - MySQL 5.7+
  - MariaDB 10.2+
  - PostgreSQL 10+
  - SQLite 3.8.0+

## Development Patterns

### Code Organization
- **cmd/**: Entry points
- **internal/**: Private packages
- **config/**: Configuration management
- **db/**: Database abstraction
- **log/**: Logging infrastructure
- **mcp/**: MCP server implementation

### Testing Strategy
- Unit tests for individual components
- Integration tests with test databases
- Mock MCP client for end-to-end testing

### Error Handling
- Structured error responses
- Detailed logging for debugging
- User-friendly error messages

## Deployment

### Local Development
```bash
./mcp-server
```

### Production Deployment
1. Set secure AES key via environment variable
2. Configure appropriate pool sizes
3. Set up log rotation and monitoring
4. Use systemd or similar for process management
5. Optional SSE transport: set `MCP_SSE_ADDR` (e.g., `:8080`) to serve MCP over HTTP/SSE alongside stdio.

### Integration with Kilocode AI
```yaml
mcp_providers:
  - name: database-mcp
    type: process
    command: /path/to/mcp-server
    working_dir: /path/to/config
    auto_start: true
```

## Monitoring & Debugging

### Logs
- Location: `mcp-provider.log`
- Format: Structured JSON
- Rotation: 500KB file size limit
- Retention: 7 days
- Stdout logging disabled by default; enable via `MCP_LOG_TO_STDOUT=true` to mirror logs (kept off to avoid MCP stdio contamination)

### Debug Commands
```bash
# Test configuration
./mcp-server --test-config

# Verbose logging
./mcp-server --verbose

# Check MCP info
echo '{"method":"mcp-info"}' | ./mcp-server
```

## Dependencies Management

### Update Dependencies
```bash
go get -u ./...
go mod tidy
```

### Security Scanning
```bash
go list -m -json all | nancy sleuth
```

## Known Technical Limitations
- No GUI interface (CLI and MCP only)
- Single transaction per action (no multi-statement transactions)
- Connection pool per process (not shared across instances)

## Documentation

- README.md provides comprehensive documentation for all 12 MCP tools, configuration scenarios, and usage examples for all supported databases.
