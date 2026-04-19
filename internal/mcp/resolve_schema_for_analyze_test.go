//go:build cgo

package mcp

import (
	"context"
	"fmt"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

// TestResolveSchemaForAnalyze verifies that resolveSchemaForAnalyze correctly
// resolves the schema for different database types.
//
// BUG-009 fix: Previously, resolveSchemaForAnalyze returned "" for MySQL/MariaDB,
// causing INFORMATION_SCHEMA queries with WHERE TABLE_SCHEMA = '' to return 0 rows.
// This broke analyze-schema for MySQL/MariaDB (#75, #76, #77, #78, #79, #80).
func TestResolveSchemaForAnalyze(t *testing.T) {
	ctx := context.Background()

	t.Run("explicit_schema_override_takes_priority", func(t *testing.T) {
		// Regardless of db type, explicit schema should be returned as-is
		result := resolveSchemaForAnalyze(ctx, nil, "myschema", "mysql", "mydb")
		if result != "myschema" {
			t.Errorf("expected explicit schema 'myschema', got %q", result)
		}
	})

	t.Run("mysql_returns_database_name_when_schema_empty", func(t *testing.T) {
		// This is the core bug fix: MySQL should return the database name, not ""
		result := resolveSchemaForAnalyze(ctx, nil, "", "mysql", "voipdb")
		if result != "voipdb" {
			t.Errorf("expected database name 'voipdb' for MySQL, got %q", result)
		}
	})

	t.Run("mariadb_returns_database_name_when_schema_empty", func(t *testing.T) {
		result := resolveSchemaForAnalyze(ctx, nil, "", "mariadb", "production_db")
		if result != "production_db" {
			t.Errorf("expected database name 'production_db' for MariaDB, got %q", result)
		}
	})

	t.Run("postgres_queries_current_schema_when_empty", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("failed to create sqlmock: %v", err)
		}
		defer db.Close()

		rows := sqlmock.NewRows([]string{"current_schema"}).AddRow("myapp_schema")
		mock.ExpectQuery("SELECT current_schema").WillReturnRows(rows)

		result := resolveSchemaForAnalyze(ctx, db, "", "postgres", "mydb")
		if result != "myapp_schema" {
			t.Errorf("expected 'myapp_schema' from current_schema() for Postgres, got %q", result)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled expectations: %v", err)
		}
	})

	t.Run("postgresql_queries_current_schema_when_empty", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("failed to create sqlmock: %v", err)
		}
		defer db.Close()

		rows := sqlmock.NewRows([]string{"current_schema"}).AddRow("custom_schema")
		mock.ExpectQuery("SELECT current_schema").WillReturnRows(rows)

		result := resolveSchemaForAnalyze(ctx, db, "", "postgresql", "mydb")
		if result != "custom_schema" {
			t.Errorf("expected 'custom_schema' from current_schema() for PostgreSQL, got %q", result)
		}
	})

	t.Run("postgres_fallback_to_public_on_query_error", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("failed to create sqlmock: %v", err)
		}
		defer db.Close()

		mock.ExpectQuery("SELECT current_schema").WillReturnError(fmt.Errorf("connection lost"))

		result := resolveSchemaForAnalyze(ctx, db, "", "postgres", "mydb")
		if result != "public" {
			t.Errorf("expected fallback 'public' on Postgres query error, got %q", result)
		}
	})

	t.Run("sqlite_returns_empty_when_schema_empty", func(t *testing.T) {
		// SQLite has no schema concept, so empty is correct
		result := resolveSchemaForAnalyze(ctx, nil, "", "sqlite", "")
		if result != "" {
			t.Errorf("expected empty string for SQLite with no schema, got %q", result)
		}
	})

	t.Run("postgres_explicit_schema_overrides_query", func(t *testing.T) {
		// Even with a valid connection, explicit schema should take priority
		db, _, err := sqlmock.New()
		if err != nil {
			t.Fatalf("failed to create sqlmock: %v", err)
		}
		defer db.Close()
		// No expectations = query should NOT be called

		result := resolveSchemaForAnalyze(ctx, db, "explicit_schema", "postgres", "mydb")
		if result != "explicit_schema" {
			t.Errorf("expected explicit schema 'explicit_schema', got %q", result)
		}
	})
}

// TestResolveSchemaForAnalyze_BugRegression verifies the specific bug that caused
// issues #75-#80: MySQL/MariaDB returning empty schema.
//
// Before the fix, resolveSchemaForAnalyze(ctx, conn, "", "mysql", "testdb")
// returned "" because the function only handled Postgres.
// This caused WHERE TABLE_SCHEMA = '' → 0 rows in all INFORMATION_SCHEMA queries.
func TestResolveSchemaForAnalyze_BugRegression(t *testing.T) {
	ctx := context.Background()

	dbTypes := []struct {
		dbType       string
		databaseName string
		expected     string
	}{
		{"mysql", "voipdb", "voipdb"},
		{"mysql", "ecommerce_prod", "ecommerce_prod"},
		{"mariadb", "analytics_db", "analytics_db"},
		{"mariadb", "test", "test"},
	}

	for _, tc := range dbTypes {
		t.Run(tc.dbType+"_"+tc.databaseName, func(t *testing.T) {
			result := resolveSchemaForAnalyze(ctx, nil, "", tc.dbType, tc.databaseName)
			if result == "" {
				t.Errorf("BUG REGRESSION: resolveSchemaForAnalyze returned empty string for %s with databaseName=%q - this causes INFORMATION_SCHEMA queries to return 0 rows (#75-#80)", tc.dbType, tc.databaseName)
			}
			if result != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, result)
			}
		})
	}
}