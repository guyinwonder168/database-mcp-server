package mcp

import (
	"testing"
	"time"
)

func TestSchemaChangeType_String(t *testing.T) {
	tests := []struct {
		input    SchemaChangeType
		expected string
	}{
		{ChangeTypeAddColumn, "add_column"},
		{ChangeTypeDropColumn, "drop_column"},
		{ChangeTypeAlterType, "alter_type"},
		{ChangeTypeRenameColumn, "rename_column"},
		{ChangeTypeAddTable, "add_table"},
		{ChangeTypeDropTable, "drop_table"},
		{ChangeTypeAlterConstraint, "alter_constraint"},
	}

	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			if got := tt.input.String(); got != tt.expected {
				t.Errorf("SchemaChangeType.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSchemaChangeType_Validate(t *testing.T) {
	tests := []struct {
		name    string
		ctype   SchemaChangeType
		wantErr bool
	}{
		{
			name:    "valid_add_column",
			ctype:   ChangeTypeAddColumn,
			wantErr: false,
		},
		{
			name:    "valid_drop_table",
			ctype:   ChangeTypeDropTable,
			wantErr: false,
		},
		{
			name:    "invalid_type",
			ctype:   SchemaChangeType("invalid"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.ctype.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSchemaChange_Validate(t *testing.T) {
	tests := []struct {
		name    string
		change  SchemaChange
		wantErr bool
	}{
		{
			name: "valid_breaking_change",
			change: SchemaChange{
				Type:   ChangeTypeDropColumn,
				Table:  "users",
				Column: "email",
				Impact: "breaking",
			},
			wantErr: false,
		},
		{
			name: "valid_compatible_change",
			change: SchemaChange{
				Type:   ChangeTypeAddColumn,
				Table:  "users",
				Column: "phone",
				Impact: "compatible",
			},
			wantErr: false,
		},
		{
			name: "invalid_impact",
			change: SchemaChange{
				Type:   ChangeTypeAddColumn,
				Table:  "users",
				Impact: "invalid",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.change.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMigrationScript_Validate(t *testing.T) {
	tests := []struct {
		name    string
		script  MigrationScript
		wantErr bool
	}{
		{
			name: "valid_script",
			script: MigrationScript{
				FromVersion:  "v1.0.0",
				ToVersion:    "v1.1.0",
				Dialect:      "mysql",
				Statements:   []string{"ALTER TABLE users ADD COLUMN phone VARCHAR(20)"},
				IsReversible: true,
			},
			wantErr: false,
		},
		{
			name: "missing_from_version",
			script: MigrationScript{
				ToVersion:  "v1.1.0",
				Dialect:    "mysql",
				Statements: []string{"ALTER TABLE users ADD COLUMN phone VARCHAR(20)"},
			},
			wantErr: true,
		},
		{
			name: "invalid_dialect",
			script: MigrationScript{
				FromVersion: "v1.0.0",
				ToVersion:   "v1.1.0",
				Dialect:     "mongodb",
				Statements:  []string{"ALTER TABLE users ADD COLUMN phone VARCHAR(20)"},
			},
			wantErr: true,
		},
		{
			name: "no_statements",
			script: MigrationScript{
				FromVersion: "v1.0.0",
				ToVersion:   "v1.1.0",
				Dialect:     "mysql",
				Statements:  []string{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.script.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSchemaSnapshot_Validate(t *testing.T) {
	tests := []struct {
		name     string
		snapshot SchemaSnapshot
		wantErr  bool
	}{
		{
			name: "valid_snapshot",
			snapshot: SchemaSnapshot{
				ID:         "snap-001",
				Timestamp:  time.Now(),
				Profile:    "test-profile",
				TablesHash: "abc123",
				Tables: map[string]SimpleTableInfo{
					"users": {Name: "users", Type: "table"},
				},
			},
			wantErr: false,
		},
		{
			name: "missing_id",
			snapshot: SchemaSnapshot{
				Timestamp: time.Now(),
				Profile:   "test-profile",
				Tables:    map[string]SimpleTableInfo{},
			},
			wantErr: true,
		},
		{
			name: "missing_timestamp",
			snapshot: SchemaSnapshot{
				ID:      "snap-001",
				Profile: "test-profile",
				Tables:  map[string]SimpleTableInfo{},
			},
			wantErr: true,
		},
		{
			name: "empty_tables",
			snapshot: SchemaSnapshot{
				ID:        "snap-001",
				Timestamp: time.Now(),
				Profile:   "test-profile",
				Tables:    map[string]SimpleTableInfo{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.snapshot.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestComputeTablesHash(t *testing.T) {
	tests := []struct {
		name    string
		tables  map[string]SimpleTableInfo
		wantErr bool
	}{
		{
			name: "valid_tables",
			tables: map[string]SimpleTableInfo{
				"users":  {Name: "users", Type: "table"},
				"orders": {Name: "orders", Type: "table"},
			},
			wantErr: false,
		},
		{
			name:    "empty_tables",
			tables:  map[string]SimpleTableInfo{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := ComputeTablesHash(tt.tables)
			if (err != nil) != tt.wantErr {
				t.Errorf("ComputeTablesHash() error = %v, wantErr %v", err, tt.wantErr)
			}
			// SHA-256 hash is 64 hex characters
			if err == nil && len(hash) != 64 {
				t.Errorf("Hash length = %d, want 64", len(hash))
			}
		})
	}
}

func TestSchemaDiff(t *testing.T) {
	diff := SchemaDiff{
		AddedTables:   []string{"new_table"},
		RemovedTables: []string{"old_table"},
		ModifiedTables: []TableDiff{
			{
				TableName: "users",
				AddedCols: []SchemaChange{
					{Type: ChangeTypeAddColumn, Table: "users", Column: "phone"},
				},
			},
		},
		Changes: []SchemaChange{
			{Type: ChangeTypeAddTable, Table: "new_table"},
		},
	}

	if len(diff.AddedTables) != 1 {
		t.Errorf("AddedTables length = %d, want 1", len(diff.AddedTables))
	}

	if len(diff.RemovedTables) != 1 {
		t.Errorf("RemovedTables length = %d, want 1", len(diff.RemovedTables))
	}

	if len(diff.ModifiedTables) != 1 {
		t.Errorf("ModifiedTables length = %d, want 1", len(diff.ModifiedTables))
	}

	if len(diff.Changes) != 1 {
		t.Errorf("Changes length = %d, want 1", len(diff.Changes))
	}
}
