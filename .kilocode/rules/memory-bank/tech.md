# Database MCP Server - Technology Stack

## Core Stack

### Language and Toolchain
- Go module baseline: `go 1.26`
- Toolchain: `go1.26.2`

### MCP SDK
- `github.com/modelcontextprotocol/go-sdk v1.5.0`

### Database Drivers
- MySQL/MariaDB: `github.com/go-sql-driver/mysql v1.9.3`
- PostgreSQL: `github.com/lib/pq v1.12.3`
- SQLite: `github.com/mattn/go-sqlite3 v1.14.42`

### Other Key Dependencies
- YAML config: `gopkg.in/yaml.v3`
- JSON schema helper: `github.com/google/jsonschema-go v0.4.2`
- SQL parser support: `github.com/blastrain/vitess-sqlparser`
- Log rotation: `github.com/lestrrat-go/file-rotatelogs`
- SQL mocking (tests): `github.com/DATA-DOG/go-sqlmock`

## Runtime Model

- Default transport: stdio
- Optional transport: HTTP/SSE via `MCP_SSE_ADDR`
- Configuration: `config.yaml` (auto-created if missing)
- Logs: structured JSON with rotation

## Security and Policy

- AES-GCM password encryption
- Per-profile read-only mode
- SQL injection prevention via `sanitizeIdentifier()` + `quoteForDB()` helpers
- Parameterized queries throughout (including SQLite TVF for PRAGMAs)
- No `fmt.Sprintf` in SQL construction paths

## Build and Test Commands

```bash
go build -o ./tmp/mcp-server ./cmd/server/main.go
go test ./...
go test -cover ./...
go vet ./...
```

Optional live integration tests:

```bash
DB_MCP_IT_PG_HOST=... DB_MCP_IT_PG_DB=... go test ./internal/mcp -run TestLive -count=1
```

## Packaging

- GitHub Releases for binaries
- GHCR package: `ghcr.io/guyinwonder168/database-mcp-server`

## Version Sync

- Source of truth: `MCPVersion` constant in `internal/mcp/server.go`
- Sync script: `scripts/sync-version-from-server.sh` updates README.md and `docs/mcp-openapi.yaml`
- Must be run after any version bump

## Current Capability Surface

- 21 MCP tools implemented
- Full coverage of profile management, schema discovery, SQL execution, intelligence tools, schema tracking, and federation
- Dedicated `internal/mcp/analyze/` package for schema analysis logic
- Dedicated `internal/mcp/lineage/` for data lineage
- Dedicated `internal/mcp/nlp/` for natural language processing
- Dedicated `internal/mcp/context/` for conversation management

## Source of Truth

- Runtime contracts: `internal/mcp/server.go`
- Analyze-schema logic: `internal/mcp/analyze/`
- API schema: `docs/mcp-openapi.yaml`
