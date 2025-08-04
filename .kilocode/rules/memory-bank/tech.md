# Database MCP Server - Technology Stack

## Core Technologies

### Programming Language
- **Go (Golang)** - Version 1.23.0+
  - Chosen for performance, strong typing, and excellent concurrency support
  - Toolchain: go1.24.0

### MCP Framework
- **Official Go MCP SDK** (github.com/modelcontextprotocol/go-sdk v0.2.0)
  - Model Context Protocol implementation
  - Stdio transport for local communication
  - JSON-RPC protocol support

### Database Drivers
- **MySQL/MariaDB**: github.com/go-sql-driver/mysql v1.9.3
- **PostgreSQL**: github.com/lib/pq v1.10.9
- **SQLite**: github.com/mattn/go-sqlite3 v1.14.30
- All use standard `database/sql` interface

### Supporting Libraries
- **YAML Configuration**: gopkg.in/yaml.v3
- **Log Rotation**: github.com/lestrrat-go/file-rotatelogs v2.4.0
- **Encryption**: Standard library crypto/aes for AES-GCM

## Development Setup

### Prerequisites
```bash
# Go 1.23.0 or higher
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
- Standard database environment variables respected by drivers

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

- README.md provides comprehensive documentation for all 11 MCP tools, configuration scenarios, and usage examples for all supported databases.