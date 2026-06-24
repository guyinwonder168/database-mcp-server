package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"testing"

	"database-mcp-provider/internal/config"

	"github.com/DATA-DOG/go-sqlmock"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestQueryRowsForSQLPreservesMySQLUnknownColumnError(t *testing.T) {
	conn, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sqlmock: %v", err)
	}
	defer conn.Close()

	const query = "SELECT u.userName FROM ohrm_user u"
	driverErr := errors.New("Error 1054 (42S22): Unknown column 'u.userName' in 'field list'")
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
	if structured.Details != driverErr.Error() {
		t.Fatalf("expected driver details %q, got %q", driverErr.Error(), structured.Details)
	}
	if structured.Details == "<nil>" {
		t.Fatal("database error details must not serialize as <nil>")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}
