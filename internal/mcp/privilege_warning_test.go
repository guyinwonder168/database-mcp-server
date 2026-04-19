package mcp

import (
	"strings"
	"testing"
)

// TestBuildPrivilegeWarning verifies that privilege warning messages are generated
// correctly when tables exist but column data is missing (likely a permissions issue).
func TestBuildPrivilegeWarning(t *testing.T) {
	t.Run("no_warning_when_no_tables", func(t *testing.T) {
		result := buildPrivilegeWarning("mysql", 0, 0, "testdb", "")
		if result != "" {
			t.Errorf("expected no warning when no tables, got %q", result)
		}
	})

	t.Run("no_warning_when_columns_present", func(t *testing.T) {
		result := buildPrivilegeWarning("mysql", 10, 10, "testdb", "")
		if result != "" {
			t.Errorf("expected no warning when columns present, got %q", result)
		}
	})

	t.Run("mysql_warning_with_grant_command", func(t *testing.T) {
		result := buildPrivilegeWarning("mysql", 5, 0, "voipdb", "")
		if result == "" {
			t.Fatal("expected warning for MySQL with tables but no columns")
		}
		if !strings.Contains(result, "GRANT SELECT ON voipdb.*") {
			t.Errorf("expected GRANT command in warning, got %q", result)
		}
		if !strings.Contains(result, "5 tables") {
			t.Errorf("expected table count in warning, got %q", result)
		}
	})

	t.Run("mariadb_warning_with_grant_command", func(t *testing.T) {
		result := buildPrivilegeWarning("mariadb", 12, 0, "analytics_db", "")
		if result == "" {
			t.Fatal("expected warning for MariaDB with tables but no columns")
		}
		if !strings.Contains(result, "GRANT SELECT ON analytics_db.*") {
			t.Errorf("expected GRANT command in warning, got %q", result)
		}
	})

	t.Run("postgres_warning_with_grant_usage", func(t *testing.T) {
		result := buildPrivilegeWarning("postgres", 8, 0, "mydb", "app_schema")
		if result == "" {
			t.Fatal("expected warning for Postgres with tables but no columns")
		}
		if !strings.Contains(result, "GRANT USAGE ON SCHEMA app_schema") {
			t.Errorf("expected GRANT USAGE command in warning, got %q", result)
		}
	})

	t.Run("postgres_uses_public_when_schema_empty", func(t *testing.T) {
		result := buildPrivilegeWarning("postgresql", 3, 0, "mydb", "")
		if result == "" {
			t.Fatal("expected warning for PostgreSQL with tables but no columns")
		}
		if !strings.Contains(result, "GRANT USAGE ON SCHEMA public") {
			t.Errorf("expected 'public' schema fallback in warning, got %q", result)
		}
	})

	t.Run("sqlite_generic_warning", func(t *testing.T) {
		result := buildPrivilegeWarning("sqlite", 2, 0, "", "")
		if result == "" {
			t.Fatal("expected warning for SQLite with tables but no columns")
		}
		if !strings.Contains(result, "insufficient database privileges") {
			t.Errorf("expected generic privilege warning, got %q", result)
		}
	})

	t.Run("no_warning_when_some_columns_present", func(t *testing.T) {
		// Even partial column data means privileges are working
		result := buildPrivilegeWarning("mysql", 20, 15, "testdb", "")
		if result != "" {
			t.Errorf("expected no warning when some columns present, got %q", result)
		}
	})
}