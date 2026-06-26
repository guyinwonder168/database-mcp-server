package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"testing"

	"database-mcp-provider/internal/config"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestQueryRowsForSQLPreservesMySQLUnknownColumnError(t *testing.T) {
	conn, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sqlmock: %v", err)
	}
	defer conn.Close()

	const query = "SELECT u.userName FROM ohrm_user u"
	driverErr := &mysql.MySQLError{
		Number:   1054,
		SQLState: [5]byte{'4', '2', 'S', '2', '2'},
		Message:  "Unknown column 'u.userName' in 'field list'",
	}
	mock.ExpectQuery(regexp.QuoteMeta(query)).WillReturnError(driverErr)

	server := &MCPServer{errorAnalyzer: NewErrorAnalyzer("")}
	rows, result, queryErr := server.queryRowsForSQL(
		context.Background(),
		conn,
		ExecuteSQLParams{ProfileName: "hrm-mariadb", SQL: query},
		&config.Profile{DBType: "mysql"},
	)

	if rows != nil {
		t.Fatal("expected no rows after query failure")
	}
	if queryErr != nil {
		t.Fatalf("expected structured error result, got Go error: %v", queryErr)
	}
	if result == nil || !result.IsError {
		t.Fatalf("expected MCP error result, got %#v", result)
	}
	if len(result.Content) != 1 {
		t.Fatalf("expected one error content item, got %d", len(result.Content))
	}

	text, ok := result.Content[0].(*mcpsdk.TextContent)
	if !ok {
		t.Fatalf("expected text error content, got %T", result.Content[0])
	}

	var structured StructuredError
	if err := json.Unmarshal([]byte(text.Text), &structured); err != nil {
		t.Fatalf("decode structured error: %v", err)
	}
	if structured.ErrorCode != ErrorCodeColumnNotFound {
		t.Fatalf("expected %s, got %s", ErrorCodeColumnNotFound, structured.ErrorCode)
	}
	if structured.Message != "Column 'u.userName' not found" {
		t.Fatalf("unexpected error message %q", structured.Message)
	}
	if structured.Details != driverErr.Error() {
		t.Fatalf("expected driver details %q, got %q", driverErr.Error(), structured.Details)
	}
	if structured.Details == "<nil>" {
		t.Fatal("database error details must not serialize as <nil>")
	}
	if structured.Context["database_error_code"] != float64(1054) {
		t.Fatalf("expected database error code 1054, got %#v", structured.Context["database_error_code"])
	}
	if structured.Context["sql_state"] != "42S22" {
		t.Fatalf("expected SQLSTATE 42S22, got %#v", structured.Context["sql_state"])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func TestTryExecuteSQLQueryPreservesRowIterationError(t *testing.T) {
	conn, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sqlmock: %v", err)
	}
	defer conn.Close()

	const query = "SELECT 1"
	driverErr := errors.New("stream interrupted while reading rows")
	rows := sqlmock.NewRows([]string{"value"}).
		AddRow(1).
		RowError(0, driverErr)
	mock.ExpectQuery(regexp.QuoteMeta(query)).WillReturnRows(rows)

	server := &MCPServer{errorAnalyzer: NewErrorAnalyzer("")}
	_, handled, result, queryErr := server.tryExecuteSQLQuery(
		context.Background(),
		conn,
		ExecuteSQLParams{ProfileName: "test-db", SQL: query},
		&config.Profile{DBType: "mysql"},
	)

	if handled {
		t.Fatal("row iteration failure must not be reported as a handled success")
	}
	if queryErr != nil {
		t.Fatalf("expected structured error result, got Go error: %v", queryErr)
	}
	if result == nil || !result.IsError {
		t.Fatalf("expected MCP error result, got %#v", result)
	}

	structured := decodeStructuredErrorResult(t, result)
	if structured.ErrorCode != ErrorCodeSQLExecutionError {
		t.Fatalf("expected %s, got %s", ErrorCodeSQLExecutionError, structured.ErrorCode)
	}
	if structured.Details != driverErr.Error() {
		t.Fatalf("expected driver details %q, got %q", driverErr.Error(), structured.Details)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func TestAnalyzeErrorDoesNotExposeNonSQLConnectionDetails(t *testing.T) {
	const sensitiveError = "dial tcp 10.0.0.5:3306 password=secret: connection refused"

	structured := NewErrorAnalyzer("").AnalyzeError(
		errors.New(sensitiveError),
		map[string]interface{}{"operation": "connect", "profile_name": "private-db"},
	)

	if structured.ErrorCode != ErrorCodeConnectionFailed {
		t.Fatalf("expected %s, got %s", ErrorCodeConnectionFailed, structured.ErrorCode)
	}
	if structured.Details != "Unable to connect to the configured database" {
		t.Fatalf("unexpected connection details %q", structured.Details)
	}
	if strings.Contains(structured.ToJSON(), "password=secret") {
		t.Fatal("non-SQL connection errors must not expose raw sensitive details")
	}
}

func TestAnalyzeErrorRedactsMySQLDuplicateEntryValue(t *testing.T) {
	driverErr := &mysql.MySQLError{
		Number:   1062,
		SQLState: [5]byte{'2', '3', '0', '0', '0'},
		Message:  "Duplicate entry 'O'Reilly@example.com' for key 'users.email'",
	}

	structured := NewErrorAnalyzer("").AnalyzeError(
		driverErr,
		map[string]interface{}{"operation": "prepared_exec", "profile_name": "app-db"},
	)

	if structured.ErrorCode != ErrorCodeConstraintViolation {
		t.Fatalf("expected %s, got %s", ErrorCodeConstraintViolation, structured.ErrorCode)
	}
	if strings.Contains(structured.Details, "Reilly@example.com") {
		t.Fatal("constraint details must not expose the conflicting value")
	}
	if !strings.Contains(structured.Details, "Duplicate entry '<redacted>'") {
		t.Fatalf("expected redacted duplicate-entry details, got %q", structured.Details)
	}
	if structured.Context["database_error_code"] != 1062 {
		t.Fatalf("expected database error code 1062, got %#v", structured.Context["database_error_code"])
	}
	if structured.Context["sql_state"] != "23000" {
		t.Fatalf("expected SQLSTATE 23000, got %#v", structured.Context["sql_state"])
	}
}

func TestAnalyzeErrorExtractsQualifiedMySQLTableName(t *testing.T) {
	driverErr := &mysql.MySQLError{
		Number:   1146,
		SQLState: [5]byte{'4', '2', 'S', '0', '2'},
		Message:  "Table 'hrm.ohrm_user' doesn't exist",
	}

	structured := NewErrorAnalyzer("").AnalyzeError(
		driverErr,
		map[string]interface{}{"operation": "query", "profile_name": "hrm-mariadb"},
	)

	if structured.ErrorCode != ErrorCodeTableNotFound {
		t.Fatalf("expected %s, got %s", ErrorCodeTableNotFound, structured.ErrorCode)
	}
	if structured.Message != "Table 'hrm.ohrm_user' not found" {
		t.Fatalf("unexpected error message %q", structured.Message)
	}
	if structured.Context["table_name"] != "hrm.ohrm_user" {
		t.Fatalf("expected qualified table name, got %#v", structured.Context["table_name"])
	}
	if structured.Context["database_error_code"] != 1146 {
		t.Fatalf("expected database error code 1146, got %#v", structured.Context["database_error_code"])
	}
	if structured.Context["sql_state"] != "42S02" {
		t.Fatalf("expected SQLSTATE 42S02, got %#v", structured.Context["sql_state"])
	}
}

func TestAnalyzeErrorPreservesDatabaseNameForMySQL1049(t *testing.T) {
	driverErr := &mysql.MySQLError{
		Number:   1049,
		SQLState: [5]byte{'4', '2', '0', '0', '0'},
		Message:  "Unknown database 'missing_db'",
	}

	structured := NewErrorAnalyzer("").AnalyzeError(
		driverErr,
		map[string]interface{}{
			"operation":    "connect",
			"profile_name": "hrm-mariadb",
			"database":     "missing_db",
		},
	)

	if structured.ErrorCode != ErrorCodeDatabaseNotFound {
		t.Fatalf("expected %s, got %s", ErrorCodeDatabaseNotFound, structured.ErrorCode)
	}
	if structured.Message != "Database 'missing_db' not found" {
		t.Fatalf("unexpected error message %q", structured.Message)
	}
	if structured.Details != "The specified database does not exist" {
		t.Fatalf("unexpected database details %q", structured.Details)
	}
	if structured.Context["database_name"] != "missing_db" {
		t.Fatalf("expected database_name context, got %#v", structured.Context["database_name"])
	}
	if strings.Contains(structured.ToJSON(), driverErr.Error()) {
		t.Fatal("non-SQL database diagnostics must not expose raw driver details")
	}
}

func decodeStructuredErrorResult(t *testing.T, result *mcpsdk.CallToolResult) StructuredError {
	t.Helper()
	if len(result.Content) != 1 {
		t.Fatalf("expected one error content item, got %d", len(result.Content))
	}
	text, ok := result.Content[0].(*mcpsdk.TextContent)
	if !ok {
		t.Fatalf("expected text error content, got %T", result.Content[0])
	}
	var structured StructuredError
	if err := json.Unmarshal([]byte(text.Text), &structured); err != nil {
		t.Fatalf("decode structured error: %v", err)
	}
	return structured
}
