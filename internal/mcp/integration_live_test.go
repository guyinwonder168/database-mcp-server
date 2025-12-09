//go:build !short

package mcp

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"database-mcp-provider/internal/config"
)

const aesKey32 = "12345678901234567890123456789012"

func requireEnvOrSkip(t *testing.T, key string) string {
	t.Helper()
	v := os.Getenv(key)
	if v == "" {
		t.Skipf("skipping: environment variable %s not set", key)
	}
	return v
}

func TestLivePostgres_SelectAndList(t *testing.T) {
	host := requireEnvOrSkip(t, "DB_MCP_IT_PG_HOST")
	portStr := requireEnvOrSkip(t, "DB_MCP_IT_PG_PORT")
	user := requireEnvOrSkip(t, "DB_MCP_IT_PG_USER")
	pass := requireEnvOrSkip(t, "DB_MCP_IT_PG_PASS")
	dbName := requireEnvOrSkip(t, "DB_MCP_IT_PG_DB")
	sslmode := os.Getenv("DB_MCP_IT_PG_SSLMODE")
	if sslmode == "" {
		sslmode = "disable"
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("invalid port %q: %v", portStr, err)
	}

	cfg := &config.Config{
		Profiles: []config.Profile{{
			ProfileName:  "pg_live",
			DBType:       "postgres",
			Host:         host,
			Port:         port,
			Username:     user,
			Password:     pass,
			DatabaseName: dbName,
			SSLMode:      sslmode,
		}},
		MaxPoolSize: 5,
		AESKey:      aesKey32,
	}
	configPath := filepath.Join(t.TempDir(), "config_pg.yaml")
	if err := config.SaveConfig(configPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	server := NewMCPServerWithConfig(configPath)
	ctx := context.Background()

	if _, _, err := server.handleExecuteSQL(ctx, nil, ExecuteSQLParams{
		ProfileName:  "pg_live",
		DatabaseName: dbName,
		SQL:          "SELECT 1",
	}); err != nil {
		t.Fatalf("postgres SELECT 1 failed: %v", err)
	}

	if res, _, err := server.handleListDatabases(ctx, nil, ListDatabasesParams{
		ProfileName: "pg_live",
	}); err != nil {
		t.Fatalf("postgres list-databases failed: %v", err)
	} else if res == nil || len(res.Content) == 0 {
		t.Fatalf("postgres list-databases returned empty content")
	}
}

func TestLiveMySQL_SelectAndList(t *testing.T) {
	host := requireEnvOrSkip(t, "DB_MCP_IT_MYSQL_HOST")
	portStr := requireEnvOrSkip(t, "DB_MCP_IT_MYSQL_PORT")
	user := requireEnvOrSkip(t, "DB_MCP_IT_MYSQL_USER")
	pass := requireEnvOrSkip(t, "DB_MCP_IT_MYSQL_PASS")
	dbName := requireEnvOrSkip(t, "DB_MCP_IT_MYSQL_DB")
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("invalid port %q: %v", portStr, err)
	}

	cfg := &config.Config{
		Profiles: []config.Profile{{
			ProfileName:  "mysql_live",
			DBType:       "mysql",
			Host:         host,
			Port:         port,
			Username:     user,
			Password:     pass,
			DatabaseName: dbName,
		}},
		MaxPoolSize: 5,
		AESKey:      aesKey32,
	}
	configPath := filepath.Join(t.TempDir(), "config_mysql.yaml")
	if err := config.SaveConfig(configPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	server := NewMCPServerWithConfig(configPath)
	ctx := context.Background()

	if _, _, err := server.handleExecuteSQL(ctx, nil, ExecuteSQLParams{
		ProfileName:  "mysql_live",
		DatabaseName: dbName,
		SQL:          "SELECT 1",
	}); err != nil {
		t.Fatalf("mysql SELECT 1 failed: %v", err)
	}

	if res, _, err := server.handleListDatabases(ctx, nil, ListDatabasesParams{
		ProfileName: "mysql_live",
	}); err != nil {
		t.Fatalf("mysql list-databases failed: %v", err)
	} else if res == nil || len(res.Content) == 0 {
		t.Fatalf("mysql list-databases returned empty content")
	}
}
