module database-mcp-provider

go 1.26

toolchain go1.26.4

require (
	github.com/DATA-DOG/go-sqlmock v1.5.2
	github.com/blastrain/vitess-sqlparser v0.0.0-20201030050434-a139afbb1aba
	github.com/go-sql-driver/mysql v1.10.0
	github.com/google/jsonschema-go v0.4.3
	github.com/lestrrat-go/file-rotatelogs v2.4.0+incompatible
	github.com/lib/pq v1.12.3
	github.com/mattn/go-sqlite3 v1.14.47
	github.com/modelcontextprotocol/go-sdk v1.6.1
	gopkg.in/yaml.v3 v3.0.1
// Add MCP SDK import here when available
)

require (
	filippo.io/edwards25519 v1.2.0 // indirect
	github.com/jonboulle/clockwork v0.5.0 // indirect
	github.com/juju/errors v1.0.0 // indirect
	github.com/lestrrat-go/strftime v1.1.1 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/segmentio/asm v1.1.3 // indirect
	github.com/segmentio/encoding v0.5.4 // indirect
	github.com/yosida95/uritemplate/v3 v3.0.2 // indirect
	golang.org/x/mod v0.37.0 // indirect
	golang.org/x/oauth2 v0.35.0 // indirect
	golang.org/x/sync v0.21.0 // indirect
	golang.org/x/sys v0.46.0 // indirect
	golang.org/x/telemetry v0.0.0-20260611141451-d61e87d5f4a3 // indirect
	golang.org/x/text v0.33.0 // indirect
	golang.org/x/tools v0.46.0 // indirect
	golang.org/x/vuln v1.4.0 // indirect
)

tool golang.org/x/vuln/cmd/govulncheck
