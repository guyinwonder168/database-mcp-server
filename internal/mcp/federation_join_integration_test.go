//go:build cgo

package mcp

import (
	"context"
	"database-mcp-provider/internal/config"
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestExecuteSubQuerySQLite(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "federation_exec.sqlite")
	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	defer func() { _ = conn.Close() }()

	ctx := context.Background()
	if _, err := conn.ExecContext(ctx, `CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)`); err != nil {
		t.Fatalf("failed to create test table: %v", err)
	}
	if _, err := conn.ExecContext(ctx, `INSERT INTO users (name) VALUES ('Alice'), ('Bob')`); err != nil {
		t.Fatalf("failed to seed test data: %v", err)
	}

	profile := config.Profile{
		ProfileName:  "sqlite_profile",
		DBType:       "sqlite",
		DatabaseName: dbPath,
	}

	result, err := ExecuteSubQuery(ctx, SubQuery{
		Profile: "sqlite_profile",
		Alias:   "u",
		SQL:     "SELECT id, name FROM users ORDER BY id",
	}, profile)
	if err != nil {
		t.Fatalf("expected ExecuteSubQuery success, got %v", err)
	}
	if len(result.Rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(result.Rows))
	}
	if result.Columns[0] != "id" || result.Columns[1] != "name" {
		t.Fatalf("unexpected result columns: %+v", result.Columns)
	}
}

func TestExecuteSubQueryRejectsUnsafeSQL(t *testing.T) {
	_, err := ExecuteSubQuery(context.Background(), SubQuery{
		Profile: "sqlite_profile",
		Alias:   "u",
		SQL:     "DELETE FROM users",
	}, config.Profile{DBType: "sqlite", DatabaseName: ":memory:"})
	if err == nil {
		t.Fatalf("expected read-only SQL rejection")
	}
}
