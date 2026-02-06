package mcp

import (
	"strings"
	"testing"
)

func TestConvertChangeToSQL(t *testing.T) {
	tests := []struct {
		name           string
		change         SchemaChange
		dialect        string
		wantContains   []string
		wantErr        bool
		wantErrContain string
	}{
		{
			name: "add_column_mysql",
			change: SchemaChange{
				Type:     ChangeTypeAddColumn,
				Table:    "users",
				Column:   "email",
				NewValue: "VARCHAR(255)",
			},
			dialect:      "mysql",
			wantContains: []string{"ALTER TABLE `users` ADD COLUMN `email` VARCHAR(255)"},
		},
		{
			name: "drop_table_postgresql",
			change: SchemaChange{
				Type:  ChangeTypeDropTable,
				Table: "sessions",
			},
			dialect:      "postgresql",
			wantContains: []string{`DROP TABLE "sessions"`},
		},
		{
			name: "alter_type_postgresql",
			change: SchemaChange{
				Type:     ChangeTypeAlterType,
				Table:    "users",
				Column:   "age",
				NewValue: "BIGINT",
			},
			dialect:      "postgres",
			wantContains: []string{`ALTER TABLE "users" ALTER COLUMN "age" TYPE BIGINT`},
		},
		{
			name: "rename_column_sqlite",
			change: SchemaChange{
				Type:     ChangeTypeRenameColumn,
				Table:    "users",
				OldValue: "full_name",
				NewValue: "name",
			},
			dialect:      "sqlite",
			wantContains: []string{`RENAME COLUMN "full_name" TO "name"`},
		},
		{
			name: "alter_type_sqlite_unsupported",
			change: SchemaChange{
				Type:     ChangeTypeAlterType,
				Table:    "users",
				Column:   "age",
				NewValue: "INTEGER",
			},
			dialect:        "sqlite",
			wantErr:        true,
			wantErrContain: "does not support direct ALTER COLUMN TYPE",
		},
		{
			name: "invalid_dialect",
			change: SchemaChange{
				Type:   ChangeTypeAddTable,
				Table:  "events",
				Impact: "compatible",
			},
			dialect:        "oracle",
			wantErr:        true,
			wantErrContain: "unsupported dialect",
		},
		{
			name: "missing_table",
			change: SchemaChange{
				Type:   ChangeTypeAddTable,
				Impact: "compatible",
			},
			dialect:        "mysql",
			wantErr:        true,
			wantErrContain: "table is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ConvertChangeToSQL(tt.change, tt.dialect)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ConvertChangeToSQL() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr {
				if tt.wantErrContain != "" && !strings.Contains(err.Error(), tt.wantErrContain) {
					t.Fatalf("error = %q, want to contain %q", err.Error(), tt.wantErrContain)
				}
				return
			}

			for _, expected := range tt.wantContains {
				if !strings.Contains(got, expected) {
					t.Errorf("SQL = %q, want to contain %q", got, expected)
				}
			}
		})
	}
}

func TestGenerateMigration(t *testing.T) {
	t.Run("deterministic_order_and_reversibility", func(t *testing.T) {
		diff := SchemaDiff{
			Changes: []SchemaChange{
				{Type: ChangeTypeDropColumn, Table: "users", Column: "legacy_code", Impact: "breaking"},
				{Type: ChangeTypeAddColumn, Table: "users", Column: "email", NewValue: "VARCHAR(255)", Impact: "compatible"},
				{Type: ChangeTypeAddTable, Table: "audit_log", Impact: "compatible"},
			},
		}

		script := GenerateMigration(diff, "postgres")

		if script.Dialect != "postgresql" {
			t.Fatalf("Dialect = %q, want %q", script.Dialect, "postgresql")
		}
		if len(script.Statements) != 3 {
			t.Fatalf("Statements length = %d, want 3", len(script.Statements))
		}
		if !strings.Contains(script.Statements[0], `CREATE TABLE "audit_log"`) {
			t.Fatalf("First statement = %q, want CREATE TABLE first", script.Statements[0])
		}
		if script.IsReversible {
			t.Fatalf("IsReversible = true, want false because drop operations are not reversible")
		}
		if script.EstimatedTime == "" {
			t.Fatalf("EstimatedTime should not be empty")
		}
	})

	t.Run("empty_diff_returns_noop", func(t *testing.T) {
		script := GenerateMigration(SchemaDiff{}, "mysql")
		if len(script.Statements) != 1 {
			t.Fatalf("Statements length = %d, want 1", len(script.Statements))
		}
		if !strings.Contains(script.Statements[0], "No schema changes detected") {
			t.Fatalf("Statement = %q, want no-op marker", script.Statements[0])
		}
		if !script.IsReversible {
			t.Fatalf("IsReversible = false, want true for no-op migrations")
		}
	})

	t.Run("unsupported_change_falls_back_to_manual_action", func(t *testing.T) {
		diff := SchemaDiff{
			Changes: []SchemaChange{
				{Type: ChangeTypeAlterType, Table: "users", Column: "age", NewValue: "INTEGER", Impact: "breaking"},
			},
		}

		script := GenerateMigration(diff, "sqlite")
		if len(script.Statements) != 1 {
			t.Fatalf("Statements length = %d, want 1", len(script.Statements))
		}
		if !strings.Contains(script.Statements[0], "MANUAL ACTION REQUIRED") {
			t.Fatalf("Statement = %q, want manual action fallback", script.Statements[0])
		}
		if script.IsReversible {
			t.Fatalf("IsReversible = true, want false for manual-only changes")
		}
	})
}

func TestValidateMigration(t *testing.T) {
	tests := []struct {
		name      string
		script    MigrationScript
		wantCodes []string
	}{
		{
			name: "valid_script",
			script: MigrationScript{
				FromVersion:  "v1",
				ToVersion:    "v2",
				Dialect:      "mysql",
				Statements:   []string{"ALTER TABLE `users` ADD COLUMN `email` VARCHAR(255);"},
				IsReversible: true,
			},
			wantCodes: nil,
		},
		{
			name: "invalid_script_metadata",
			script: MigrationScript{
				ToVersion:    "v2",
				Dialect:      "mysql",
				Statements:   []string{"ALTER TABLE `users` ADD COLUMN `email` VARCHAR(255);"},
				IsReversible: true,
			},
			wantCodes: []string{"INVALID_SCRIPT"},
		},
		{
			name: "empty_statement",
			script: MigrationScript{
				FromVersion:  "v1",
				ToVersion:    "v2",
				Dialect:      "mysql",
				Statements:   []string{""},
				IsReversible: true,
			},
			wantCodes: []string{"EMPTY_STATEMENT"},
		},
		{
			name: "invalid_statement",
			script: MigrationScript{
				FromVersion:  "v1",
				ToVersion:    "v2",
				Dialect:      "postgresql",
				Statements:   []string{"SELECT * FROM users;"},
				IsReversible: true,
			},
			wantCodes: []string{"INVALID_STATEMENT"},
		},
		{
			name: "sqlite_unsupported_operation",
			script: MigrationScript{
				FromVersion:  "v1",
				ToVersion:    "v2",
				Dialect:      "sqlite",
				Statements:   []string{`ALTER TABLE "users" ALTER COLUMN "age" TYPE INTEGER;`},
				IsReversible: false,
			},
			wantCodes: []string{"UNSUPPORTED_OPERATION"},
		},
		{
			name: "reversibility_mismatch",
			script: MigrationScript{
				FromVersion:  "v1",
				ToVersion:    "v2",
				Dialect:      "mysql",
				Statements:   []string{"DROP TABLE `users`;"},
				IsReversible: true,
			},
			wantCodes: []string{"NON_REVERSIBLE"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := ValidateMigration(tt.script)
			if len(tt.wantCodes) == 0 {
				if len(errors) != 0 {
					t.Fatalf("ValidateMigration() returned %d errors, want 0", len(errors))
				}
				return
			}

			for _, code := range tt.wantCodes {
				if !hasValidationCode(errors, code) {
					t.Errorf("expected validation code %q, got %#v", code, errors)
				}
			}
		})
	}
}

func TestEstimateMigrationImpact(t *testing.T) {
	t.Run("low_risk", func(t *testing.T) {
		script := MigrationScript{
			Dialect:    "mysql",
			Statements: []string{"ALTER TABLE `users` ADD COLUMN `nickname` VARCHAR(50);"},
		}

		impact := EstimateMigrationImpact(script)
		if impact.TotalStatements != 1 {
			t.Fatalf("TotalStatements = %d, want 1", impact.TotalStatements)
		}
		if impact.RiskLevel != "low" {
			t.Fatalf("RiskLevel = %q, want %q", impact.RiskLevel, "low")
		}
		if impact.RequiresDowntime {
			t.Fatalf("RequiresDowntime = true, want false")
		}
		if impact.CompatibleChanges != 1 {
			t.Fatalf("CompatibleChanges = %d, want 1", impact.CompatibleChanges)
		}
	})

	t.Run("high_risk", func(t *testing.T) {
		script := MigrationScript{
			Dialect: "postgresql",
			Statements: []string{
				`DROP TABLE "legacy_orders";`,
				`ALTER TABLE "users" DROP COLUMN "obsolete_id";`,
			},
		}

		impact := EstimateMigrationImpact(script)
		if impact.RiskLevel != "high" {
			t.Fatalf("RiskLevel = %q, want %q", impact.RiskLevel, "high")
		}
		if !impact.RequiresDowntime {
			t.Fatalf("RequiresDowntime = false, want true")
		}
		if impact.BreakingChanges < 1 {
			t.Fatalf("BreakingChanges = %d, want >= 1", impact.BreakingChanges)
		}
	})

	t.Run("medium_risk_from_volume", func(t *testing.T) {
		script := MigrationScript{
			Dialect: "mysql",
			Statements: []string{
				"ALTER TABLE `users` ADD COLUMN `a` INT;",
				"ALTER TABLE `users` ADD COLUMN `b` INT;",
				"ALTER TABLE `users` ADD COLUMN `c` INT;",
				"ALTER TABLE `users` ADD COLUMN `d` INT;",
				"ALTER TABLE `users` ADD COLUMN `e` INT;",
				"ALTER TABLE `users` ADD COLUMN `f` INT;",
			},
		}

		impact := EstimateMigrationImpact(script)
		if impact.RiskLevel != "medium" {
			t.Fatalf("RiskLevel = %q, want %q", impact.RiskLevel, "medium")
		}
		if impact.TotalStatements != 6 {
			t.Fatalf("TotalStatements = %d, want 6", impact.TotalStatements)
		}
	})
}

func hasValidationCode(errors []ValidationError, code string) bool {
	for _, err := range errors {
		if err.Code == code {
			return true
		}
	}

	return false
}
