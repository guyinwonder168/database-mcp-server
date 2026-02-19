//go:build cgo

package mcp

import (
	"context"
	"database/sql"
	"testing"

	"database-mcp-provider/internal/config"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestErrorResult(t *testing.T) {
	structErr := NewStructuredError(ErrorCodeProfileNotFound, "missing profile", "profile does not exist")
	result := errorResult(structErr)
	if result == nil || len(result.Content) != 1 {
		t.Fatalf("Expected error result with one content item")
	}
	if !result.IsError {
		t.Fatalf("Expected IsError=true for error result")
	}
	content, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("Expected TextContent, got %T", result.Content[0])
	}
	if content.Text == "" {
		t.Fatalf("Expected non-empty error payload")
	}
}

func TestFindProfile(t *testing.T) {
	testConfig := setupTestConfig(t)
	server := NewMCPServerWithConfig(testConfig)
	cfg, prof, err := server.findProfile("testsqlite")
	if err != nil {
		t.Fatalf("Expected profile lookup success, got %v", err)
	}
	if cfg == nil || prof == nil {
		t.Fatalf("Expected non-nil config and profile")
	}
}

func TestOpenConnection(t *testing.T) {
	testConfig := setupTestConfig(t)
	server := NewMCPServerWithConfig(testConfig)
	ctx := context.Background()
	conn, _, err := server.openConnection(ctx, "testsqlite", "")
	if err != nil {
		t.Fatalf("Expected connection success, got %v", err)
	}
	if conn == nil {
		t.Fatalf("Expected non-nil connection")
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("Failed to close connection: %v", err)
	}
}

func TestSwitchMySQLDatabase(t *testing.T) {
	server := NewMCPServerWithConfig("nonexistent.yaml")
	result := server.switchMySQLDatabase(context.Background(), nil, &config.Profile{DBType: "postgres"}, "testpg", "db")
	if result != nil {
		t.Fatalf("Expected no-op for non-mysql profile")
	}
}

func TestSwitchMySQLDatabaseMissingConn(t *testing.T) {
	server := NewMCPServerWithConfig("nonexistent.yaml")
	result := server.switchMySQLDatabase(context.Background(), nil, &config.Profile{DBType: "mysql", DatabaseName: "db"}, "testmysql", "otherdb")
	if result == nil {
		t.Fatalf("Expected error result for missing connection")
	}
}

func TestSwitchMySQLDatabaseExecError(t *testing.T) {
	server := NewMCPServerWithConfig("nonexistent.yaml")
	conn, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open sqlite connection: %v", err)
	}
	defer conn.Close()
	result := server.switchMySQLDatabase(context.Background(), conn, &config.Profile{DBType: "mysql", DatabaseName: "db"}, "testmysql", "otherdb")
	if result == nil {
		t.Fatalf("Expected error result for switch database failure")
	}
}
