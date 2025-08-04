package mcp

import (
	"context"
	"os"
	"testing"

	"database-mcp-provider/internal/config"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Helper: create a test config file
func setupTestConfig(t *testing.T) string {
	const testConfig = "test_config.yaml"
	os.Remove(testConfig)
	cfg := &config.Config{
		Profiles: []config.Profile{
			{
				ProfileName:  "testpg",
				DBType:       "postgres",
				Host:         "localhost",
				Port:         5432,
				Username:     "testuser",
				Password:     "testpass",
				DatabaseName: "testdb",
				Readonly:     false,
			},
			{
				ProfileName:  "testsqlite",
				DBType:       "sqlite",
				DatabaseName: ":memory:",
				Readonly:     true,
			},
		},
		MaxPoolSize: 5,
	}
	if err := config.SaveConfig(testConfig, cfg); err != nil {
		t.Fatalf("Failed to save test config: %v", err)
	}
	os.Setenv("DB_MCP_AES_KEY", "12345678901234567890123456789012") // 32 bytes for test
	return testConfig
}

func TestHandleListProfiles(t *testing.T) {
	testConfig := setupTestConfig(t)
	defer os.Remove(testConfig)

	server := NewMCPServerWithConfig(testConfig)
	ctx := context.Background()
	session := &mcp.ServerSession{}
	params := &mcp.CallToolParamsFor[any]{}

	res, err := server.handleListProfiles(ctx, session, params)
	if err != nil {
		t.Fatalf("handleListProfiles error: %v", err)
	}
	if res == nil || res.Content == nil {
		t.Fatalf("handleListProfiles returned nil content")
	}
}

func TestHandleConfigureProfile(t *testing.T) {
	testConfig := setupTestConfig(t)
	defer os.Remove(testConfig)

	server := NewMCPServerWithConfig(testConfig)
	ctx := context.Background()
	session := &mcp.ServerSession{}
	params := &mcp.CallToolParamsFor[ConfigureProfileParams]{
		Arguments: ConfigureProfileParams{
			ProfileName:  "newprofile",
			DBType:       "sqlite",
			DatabaseName: ":memory:",
			Readonly:     false,
		},
	}

	res, err := server.handleConfigureProfile(ctx, session, params)
	if err != nil {
		t.Fatalf("handleConfigureProfile error: %v", err)
	}
	if res == nil || res.Content == nil {
		t.Fatalf("handleConfigureProfile returned nil content")
	}
}

// TestHandleExecuteSQL tests the execute-sql MCP action.
func TestHandleExecuteSQL(t *testing.T) {
	testConfig := setupTestConfig(t)
	defer os.Remove(testConfig)

	server := NewMCPServerWithConfig(testConfig)
	ctx := context.Background()
	session := &mcp.ServerSession{}
	params := &mcp.CallToolParamsFor[ExecuteSQLParams]{
		Arguments: ExecuteSQLParams{
			ProfileName:  "testsqlite",
			SQL:          "SELECT 1",
			DatabaseName: ":memory:",
		},
	}

	res, err := server.handleExecuteSQL(ctx, session, params)
	if err != nil {
		t.Fatalf("handleExecuteSQL error: %v", err)
	}
	if res == nil || res.Content == nil {
		t.Fatalf("handleExecuteSQL returned nil content")
	}
}

// Additional tests for execute-sql, list-tables, describe-table, list-databases would follow a similar pattern.
// For full integration tests, use a test database and mock connections as needed.

// TestHandleListTables tests the list-tables MCP action.
func TestHandleListTables(t *testing.T) {
	testConfig := setupTestConfig(t)
	defer os.Remove(testConfig)

	server := NewMCPServerWithConfig(testConfig)
	ctx := context.Background()
	session := &mcp.ServerSession{}
	params := &mcp.CallToolParamsFor[ListTablesParams]{
		Arguments: ListTablesParams{
			ProfileName:  "testsqlite",
			DatabaseName: ":memory:",
		},
	}

	res, err := server.handleListTables(ctx, session, params)
	if err != nil {
		t.Fatalf("handleListTables error: %v", err)
	}
	if res == nil || res.Content == nil {
		t.Fatalf("handleListTables returned nil content")
	}
}

// TestHandleDescribeTable tests the describe-table MCP action.
func TestHandleDescribeTable(t *testing.T) {
	testConfig := setupTestConfig(t)
	defer os.Remove(testConfig)

	server := NewMCPServerWithConfig(testConfig)
	ctx := context.Background()
	session := &mcp.ServerSession{}
	params := &mcp.CallToolParamsFor[DescribeTableParams]{
		Arguments: DescribeTableParams{
			ProfileName: "testsqlite",
			TableName:   "sqlite_master",
		},
	}

	res, err := server.handleDescribeTable(ctx, session, params)
	if err != nil {
		t.Fatalf("handleDescribeTable error: %v", err)
	}
	if res == nil || res.Content == nil {
		t.Fatalf("handleDescribeTable returned nil content")
	}
}

// TestHandleListDatabases tests the list-databases MCP action.
func TestHandleListDatabases(t *testing.T) {
	testConfig := setupTestConfig(t)
	defer os.Remove(testConfig)

	server := NewMCPServerWithConfig(testConfig)
	ctx := context.Background()
	session := &mcp.ServerSession{}
	params := &mcp.CallToolParamsFor[ListDatabasesParams]{
		Arguments: ListDatabasesParams{
			ProfileName: "testsqlite",
		},
	}

	res, err := server.handleListDatabases(ctx, session, params)
	if err != nil {
		t.Fatalf("handleListDatabases error: %v", err)
	}
	if res == nil || res.Content == nil {
		t.Fatalf("handleListDatabases returned nil content")
	}
}
