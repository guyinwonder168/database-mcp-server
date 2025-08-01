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

// Additional tests for execute-sql, list-tables, describe-table, list-databases would follow a similar pattern.
// For full integration tests, use a test database and mock connections as needed.
