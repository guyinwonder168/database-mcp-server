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
| Regression | `go test ./internal/mcp -count=1` passed after the implementation and response-message refinement. |

## Test specification

| # | Guarantee | Test | Type | Result |
|---|---|---|---|---|
| 1 | A direct MySQL query failure returns an MCP error result rather than falling through to statement execution. | `TestQueryRowsForSQLPreservesMySQLUnknownColumnError` | Integration-style unit test with `sqlmock` | PASS |
| 2 | MySQL error 1054 maps to `COLUMN_NOT_FOUND`. | `TestQueryRowsForSQLPreservesMySQLUnknownColumnError` | Error contract | PASS |
| 3 | Driver errno, SQLSTATE, and message are preserved in `details`; `"<nil>"` is never emitted. | `TestQueryRowsForSQLPreservesMySQLUnknownColumnError` | Regression | PASS |

## Security boundary

The structured response includes the database driver's error text and query metadata already used by the error analyzer. It does not add bound parameter values, credentials, or connection strings to the response.

## Coverage and known gaps

The regression uses a deterministic MySQL-compatible driver error through `sqlmock`. Live MariaDB verification remains covered by the repository's optional integration-test workflow rather than this unit test.

## Checkpoints

- RED: `97b958f test: reproduce masked execute-sql database error`
- GREEN: `ef1e2b0 fix: preserve execute-sql database errors`
