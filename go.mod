module database-mcp-provider

go 1.25

toolchain go1.25.5

require (
	github.com/blastrain/vitess-sqlparser v0.0.0-20201030050434-a139afbb1aba
	github.com/go-sql-driver/mysql v1.9.3
	github.com/lestrrat-go/file-rotatelogs v2.4.0+incompatible
	github.com/lib/pq v1.10.9
	github.com/mattn/go-sqlite3 v1.14.30
	github.com/modelcontextprotocol/go-sdk v1.1.0
	gopkg.in/yaml.v3 v3.0.1
// Add MCP SDK import here when available
)

require (
	filippo.io/edwards25519 v1.1.0 // indirect
	github.com/google/jsonschema-go v0.3.0 // indirect
	github.com/jonboulle/clockwork v0.5.0 // indirect
	github.com/juju/errors v0.0.0-20170703010042-c7d06af17c68 // indirect
	github.com/lestrrat-go/strftime v1.1.1 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/yosida95/uritemplate/v3 v3.0.2 // indirect
	golang.org/x/oauth2 v0.30.0 // indirect
	golang.org/x/text v0.3.8 // indirect
)
