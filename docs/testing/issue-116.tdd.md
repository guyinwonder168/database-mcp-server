# Issue #116 TDD Evidence

## Source

Journeys and acceptance criteria were derived from GitHub issue #116.

## User journey

As an MCP client diagnosing a failed SQL query, I want the database driver's error details and a stable error code so that I can correct the query without switching to a database CLI.

## Task report

| Stage | Evidence |
|---|---|
| RED | `go test ./internal/mcp -run TestQueryRowsForSQLPreservesMySQLUnknownColumnError -count=1` failed because `queryRowsForSQL` returned a nil MCP error result after MySQL error 1054. |
| GREEN | The same focused command passed after direct query errors were analyzed and returned as structured MCP errors. |
| Review RED | The typed-driver and row-stream regression command failed because errno/SQLSTATE metadata was absent and `rows.Err()` was ignored. |
| Review GREEN | `go test ./internal/mcp -run 'Test(QueryRowsForSQLPreservesMySQLUnknownColumnError\|TryExecuteSQLQueryPreservesRowIterationError)$' -count=1` passed after metadata extraction and row-stream error handling were added. |
| Regression | `go test ./internal/mcp -count=1` passed after the implementation and response-message refinement. |

## Test specification

| # | Guarantee | Test | Type | Result |
|---|---|---|---|---|
| 1 | A direct MySQL query failure returns an MCP error result rather than falling through to statement execution. | `TestQueryRowsForSQLPreservesMySQLUnknownColumnError` | Integration-style unit test with `sqlmock` | PASS |
| 2 | MySQL error 1054 maps to `COLUMN_NOT_FOUND`. | `TestQueryRowsForSQLPreservesMySQLUnknownColumnError` | Error contract | PASS |
| 3 | Driver errno, SQLSTATE, and message are preserved in `details`; `"<nil>"` is never emitted. | `TestQueryRowsForSQLPreservesMySQLUnknownColumnError` | Regression | PASS |
| 4 | A delayed row iteration failure returns `SQL_EXECUTION_ERROR` instead of partial rows. | `TestTryExecuteSQLQueryPreservesRowIterationError` | Regression | PASS |
| 5 | Non-SQL connection diagnostics do not expose raw connection details. | `TestAnalyzeErrorDoesNotExposeNonSQLConnectionDetails` | Security regression | PASS |
| 6 | MySQL duplicate-key diagnostics preserve error metadata while redacting the conflicting value. | `TestAnalyzeErrorRedactsMySQLDuplicateEntryValue` | Security regression | PASS |

## Security boundary

The structured response includes the database driver's error text only for SQL execution operations. MySQL duplicate-entry values are redacted, including values containing apostrophes. The server does not append bound parameters, credentials, or connection strings to the response.

## Coverage and known gaps

The regression uses a deterministic MySQL-compatible driver error through `sqlmock`. Live MariaDB verification remains covered by the repository's optional integration-test workflow rather than this unit test.

## Final verification

- `go test ./...` — PASS
- `go vet ./...` — PASS
- `go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.9.0 run ./... --timeout=5m` — PASS, 0 issues
- `go test -cover ./...` — PASS; `internal/mcp` coverage 86.0%
- `go build -o ./tmp/mcp-server ./cmd/server/main.go` — PASS
- `go run golang.org/x/vuln/cmd/govulncheck@latest ./...` — PASS, no called vulnerabilities

## Checkpoints

- RED: `97b958f test: reproduce masked execute-sql database error`
- GREEN: `ef1e2b0 fix: preserve execute-sql database errors`
- Refactor: `568e330 refactor: refine execute-sql error response`
