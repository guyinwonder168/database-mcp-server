package mcp

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Helper function to create a temporary storage directory
func tempStorageDir(t *testing.T) (string, func()) {
	t.Helper()
	dir, err := os.MkdirTemp("", "schema-storage-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	return dir, func() {
		os.RemoveAll(dir)
	}
}

// Helper function to create a sample snapshot
func sampleSnapshot(profile string) SchemaSnapshot {
	return SchemaSnapshot{
		ID:         "snap-001",
		Timestamp:  time.Now(),
		Profile:    profile,
		TablesHash: "abc123",
		Tables: map[string]SimpleTableInfo{
			"users": {
				Name: "users",
				Type: "table",
				Columns: map[string]ColumnInfo{
					"id":    {Name: "id", Type: "INT", Nullable: false},
					"email": {Name: "email", Type: "VARCHAR(255)", Nullable: false},
				},
			},
		},
	}
}

func TestSaveSnapshot(t *testing.T) {
	tests := []struct {
		name        string
		snapshot    SchemaSnapshot
		setupEnv    func() func()
		wantErr     bool
		errContains string
	}{
		{
			name:     "valid_snapshot",
			snapshot: sampleSnapshot("test-profile"),
			wantErr:  false,
		},
		{
			name: "valid_snapshot_with_multiple_tables",
			snapshot: SchemaSnapshot{
				ID:        "snap-002",
				Timestamp: time.Now(),
				Profile:   "test-profile",
				Tables: map[string]SimpleTableInfo{
					"users":  {Name: "users", Type: "table"},
					"orders": {Name: "orders", Type: "table"},
				},
			},
			wantErr: false,
		},
		{
			name: "missing_profile",
			snapshot: SchemaSnapshot{
				ID:        "snap-003",
				Timestamp: time.Now(),
				Profile:   "",
				Tables:    map[string]SimpleTableInfo{},
			},
			wantErr:     true,
			errContains: "profile is required",
		},
		{
			name: "empty_tables",
			snapshot: SchemaSnapshot{
				ID:        "snap-004",
				Timestamp: time.Now(),
				Profile:   "test-profile",
				Tables:    map[string]SimpleTableInfo{},
			},
			wantErr:     true,
			errContains: "at least one table",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir, cleanup := tempStorageDir(t)
			defer cleanup()

			err := SaveSnapshot(tt.snapshot, dir)
			if (err != nil) != tt.wantErr {
				t.Errorf("SaveSnapshot() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.errContains != "" && err != nil {
				if !contains(err.Error(), tt.errContains) {
					t.Errorf("Error message = %v, want to contain %v", err.Error(), tt.errContains)
				}
				return
			}

			// Verify file was created
			if !tt.wantErr {
				expectedFile := filepath.Join(dir, tt.snapshot.Profile, tt.snapshot.ID+".json")
				if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
					t.Errorf("Snapshot file not created at %s", expectedFile)
				}
			}
		})
	}
}

func TestGetSnapshot(t *testing.T) {
	tests := []struct {
		name        string
		setup       func() (string, SchemaSnapshot)
		profile     string
		snapshotID  string
		want        *SchemaSnapshot
		wantErr     bool
		errContains string
	}{
		{
			name: "existing_snapshot",
			setup: func() (string, SchemaSnapshot) {
				dir, _ := tempStorageDir(t)
				snapshot := sampleSnapshot("test-profile")
				if err := SaveSnapshot(snapshot, dir); err != nil {
					t.Fatalf("Setup failed: %v", err)
				}
				return dir, snapshot
			},
			profile:    "test-profile",
			snapshotID: "snap-001",
			want: func() *SchemaSnapshot {
				s := sampleSnapshot("test-profile")
				return &s
			}(),
			wantErr: false,
		},
		{
			name: "nonexistent_snapshot",
			setup: func() (string, SchemaSnapshot) {
				dir, _ := tempStorageDir(t)
				return dir, SchemaSnapshot{}
			},
			profile:     "test-profile",
			snapshotID:  "nonexistent",
			want:        nil,
			wantErr:     true,
			errContains: "not found",
		},
		{
			name: "invalid_directory",
			setup: func() (string, SchemaSnapshot) {
				return "/nonexistent/directory", SchemaSnapshot{}
			},
			profile:     "test-profile",
			snapshotID:  "snap-001",
			want:        nil,
			wantErr:     true,
			errContains: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storageDir, _ := tt.setup()
			defer os.RemoveAll(storageDir)

			got, err := GetSnapshot(tt.profile, tt.snapshotID, storageDir)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetSnapshot() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.errContains != "" && err != nil {
				if !contains(err.Error(), tt.errContains) {
					t.Errorf("Error message = %v, want to contain %v", err.Error(), tt.errContains)
				}
				return
			}

			if tt.wantErr == false {
				if got == nil {
					t.Error("GetSnapshot() returned nil, want snapshot")
					return
				}
				if got.ID != tt.want.ID {
					t.Errorf("ID = %v, want %v", got.ID, tt.want.ID)
				}
				if got.Profile != tt.want.Profile {
					t.Errorf("Profile = %v, want %v", got.Profile, tt.want.Profile)
				}
			}
		})
	}
}

func TestListSnapshots(t *testing.T) {
	tests := []struct {
		name    string
		setup   func() string
		profile string
		limit   int
		wantLen int
		wantErr bool
	}{
		{
			name: "list_multiple_snapshots",
			setup: func() string {
				dir, _ := tempStorageDir(t)
				profile := "test-profile"

				// Create 3 snapshots
				for i := 1; i <= 3; i++ {
					snap := SchemaSnapshot{
						ID:        "snap-00" + string(rune('0'+i)),
						Timestamp: time.Now().Add(time.Duration(i) * time.Hour),
						Profile:   profile,
						Tables: map[string]SimpleTableInfo{
							"users": {Name: "users", Type: "table"},
						},
					}
					if err := SaveSnapshot(snap, dir); err != nil {
						t.Fatalf("Setup failed: %v", err)
					}
				}
				return dir
			},
			profile: "test-profile",
			limit:   10,
			wantLen: 3,
			wantErr: false,
		},
		{
			name: "limit_results",
			setup: func() string {
				dir, _ := tempStorageDir(t)
				profile := "test-profile"

				// Create 5 snapshots
				for i := 1; i <= 5; i++ {
					snap := SchemaSnapshot{
						ID:        "snap-00" + string(rune('0'+i)),
						Timestamp: time.Now(),
						Profile:   profile,
						Tables: map[string]SimpleTableInfo{
							"users": {Name: "users", Type: "table"},
						},
					}
					if err := SaveSnapshot(snap, dir); err != nil {
						t.Fatalf("Setup failed: %v", err)
					}
				}
				return dir
			},
			profile: "test-profile",
			limit:   2,
			wantLen: 2,
			wantErr: false,
		},
		{
			name: "empty_profile",
			setup: func() string {
				dir, _ := tempStorageDir(t)
				return dir
			},
			profile: "nonexistent",
			limit:   10,
			wantLen: 0,
			wantErr: false,
		},
		{
			name: "invalid_directory",
			setup: func() string {
				return "/nonexistent/directory"
			},
			profile: "test-profile",
			limit:   10,
			wantLen: 0,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storageDir := tt.setup()
			defer os.RemoveAll(storageDir)

			got, err := ListSnapshots(tt.profile, tt.limit, storageDir)
			if (err != nil) != tt.wantErr {
				t.Errorf("ListSnapshots() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && len(got) != tt.wantLen {
				t.Errorf("ListSnapshots() returned %d snapshots, want %d", len(got), tt.wantLen)
			}
		})
	}
}

func TestCompareSnapshots(t *testing.T) {
	tests := []struct {
		name     string
		old      SchemaSnapshot
		new      SchemaSnapshot
		expected SchemaDiff
	}{
		{
			name: "added_table",
			old: SchemaSnapshot{
				ID:        "snap-old",
				Timestamp: time.Now(),
				Profile:   "test",
				Tables: map[string]SimpleTableInfo{
					"users": {Name: "users", Type: "table"},
				},
			},
			new: SchemaSnapshot{
				ID:        "snap-new",
				Timestamp: time.Now(),
				Profile:   "test",
				Tables: map[string]SimpleTableInfo{
					"users":  {Name: "users", Type: "table"},
					"orders": {Name: "orders", Type: "table"},
				},
			},
			expected: SchemaDiff{
				AddedTables:   []string{"orders"},
				RemovedTables: []string{},
				Changes: []SchemaChange{
					{Type: ChangeTypeAddTable, Table: "orders", Impact: "compatible"},
				},
			},
		},
		{
			name: "dropped_table",
			old: SchemaSnapshot{
				ID:        "snap-old",
				Timestamp: time.Now(),
				Profile:   "test",
				Tables: map[string]SimpleTableInfo{
					"users":  {Name: "users", Type: "table"},
					"orders": {Name: "orders", Type: "table"},
				},
			},
			new: SchemaSnapshot{
				ID:        "snap-new",
				Timestamp: time.Now(),
				Profile:   "test",
				Tables: map[string]SimpleTableInfo{
					"users": {Name: "users", Type: "table"},
				},
			},
			expected: SchemaDiff{
				AddedTables:   []string{},
				RemovedTables: []string{"orders"},
				Changes: []SchemaChange{
					{Type: ChangeTypeDropTable, Table: "orders", Impact: "breaking"},
				},
			},
		},
		{
			name: "added_column",
			old: SchemaSnapshot{
				ID:        "snap-old",
				Timestamp: time.Now(),
				Profile:   "test",
				Tables: map[string]SimpleTableInfo{
					"users": {
						Name: "users",
						Type: "table",
						Columns: map[string]ColumnInfo{
							"id": {Name: "id", Type: "INT"},
						},
					},
				},
			},
			new: SchemaSnapshot{
				ID:        "snap-new",
				Timestamp: time.Now(),
				Profile:   "test",
				Tables: map[string]SimpleTableInfo{
					"users": {
						Name: "users",
						Type: "table",
						Columns: map[string]ColumnInfo{
							"id":    {Name: "id", Type: "INT"},
							"email": {Name: "email", Type: "VARCHAR(255)"},
						},
					},
				},
			},
			expected: SchemaDiff{
				ModifiedTables: []TableDiff{
					{
						TableName: "users",
						AddedCols: []SchemaChange{
							{Type: ChangeTypeAddColumn, Table: "users", Column: "email", Impact: "compatible"},
						},
					},
				},
				Changes: []SchemaChange{
					{Type: ChangeTypeAddColumn, Table: "users", Column: "email", Impact: "compatible"},
				},
			},
		},
		{
			name: "dropped_column",
			old: SchemaSnapshot{
				ID:        "snap-old",
				Timestamp: time.Now(),
				Profile:   "test",
				Tables: map[string]SimpleTableInfo{
					"users": {
						Name: "users",
						Type: "table",
						Columns: map[string]ColumnInfo{
							"id":    {Name: "id", Type: "INT"},
							"email": {Name: "email", Type: "VARCHAR(255)"},
						},
					},
				},
			},
			new: SchemaSnapshot{
				ID:        "snap-new",
				Timestamp: time.Now(),
				Profile:   "test",
				Tables: map[string]SimpleTableInfo{
					"users": {
						Name: "users",
						Type: "table",
						Columns: map[string]ColumnInfo{
							"id": {Name: "id", Type: "INT"},
						},
					},
				},
			},
			expected: SchemaDiff{
				ModifiedTables: []TableDiff{
					{
						TableName: "users",
						DroppedCols: []SchemaChange{
							{Type: ChangeTypeDropColumn, Table: "users", Column: "email", Impact: "breaking"},
						},
					},
				},
				Changes: []SchemaChange{
					{Type: ChangeTypeDropColumn, Table: "users", Column: "email", Impact: "breaking"},
				},
			},
		},
		{
			name: "altered_column_type",
			old: SchemaSnapshot{
				ID:        "snap-old",
				Timestamp: time.Now(),
				Profile:   "test",
				Tables: map[string]SimpleTableInfo{
					"users": {
						Name: "users",
						Type: "table",
						Columns: map[string]ColumnInfo{
							"email": {Name: "email", Type: "VARCHAR(100)"},
						},
					},
				},
			},
			new: SchemaSnapshot{
				ID:        "snap-new",
				Timestamp: time.Now(),
				Profile:   "test",
				Tables: map[string]SimpleTableInfo{
					"users": {
						Name: "users",
						Type: "table",
						Columns: map[string]ColumnInfo{
							"email": {Name: "email", Type: "VARCHAR(255)"},
						},
					},
				},
			},
			expected: SchemaDiff{
				ModifiedTables: []TableDiff{
					{
						TableName: "users",
						ModifiedCols: []SchemaChange{
							{
								Type:     ChangeTypeAlterType,
								Table:    "users",
								Column:   "email",
								OldValue: "VARCHAR(100)",
								NewValue: "VARCHAR(255)",
								Impact:   "compatible",
							},
						},
					},
				},
				Changes: []SchemaChange{
					{
						Type:     ChangeTypeAlterType,
						Table:    "users",
						Column:   "email",
						OldValue: "VARCHAR(100)",
						NewValue: "VARCHAR(255)",
						Impact:   "compatible",
					},
				},
			},
		},
		{
			name: "no_changes",
			old: SchemaSnapshot{
				ID:        "snap-old",
				Timestamp: time.Now(),
				Profile:   "test",
				Tables: map[string]SimpleTableInfo{
					"users": {
						Name: "users",
						Type: "table",
						Columns: map[string]ColumnInfo{
							"id": {Name: "id", Type: "INT"},
						},
					},
				},
			},
			new: SchemaSnapshot{
				ID:        "snap-new",
				Timestamp: time.Now(),
				Profile:   "test",
				Tables: map[string]SimpleTableInfo{
					"users": {
						Name: "users",
						Type: "table",
						Columns: map[string]ColumnInfo{
							"id": {Name: "id", Type: "INT"},
						},
					},
				},
			},
			expected: SchemaDiff{
				AddedTables:    []string{},
				RemovedTables:  []string{},
				ModifiedTables: []TableDiff{},
				Changes:        []SchemaChange{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Ensure timestamps have hashes for validation
			hash, _ := ComputeTablesHash(tt.old.Tables)
			tt.old.TablesHash = hash
			tt.new.TablesHash = hash

			got := CompareSnapshots(tt.old, tt.new)

			// Compare changes (length and properties)
			if len(got.AddedTables) != len(tt.expected.AddedTables) {
				t.Errorf("AddedTables = %v, want %v", got.AddedTables, tt.expected.AddedTables)
			}

			if len(got.RemovedTables) != len(tt.expected.RemovedTables) {
				t.Errorf("RemovedTables = %v, want %v", got.RemovedTables, tt.expected.RemovedTables)
			}

			if len(got.Changes) != len(tt.expected.Changes) {
				t.Errorf("Changes length = %d, want %d", len(got.Changes), len(tt.expected.Changes))
			}

			// Verify change types
			for i, change := range got.Changes {
				if i < len(tt.expected.Changes) {
					if change.Type != tt.expected.Changes[i].Type {
						t.Errorf("Change[%d].Type = %v, want %v", i, change.Type, tt.expected.Changes[i].Type)
					}
					if change.Table != tt.expected.Changes[i].Table {
						t.Errorf("Change[%d].Table = %v, want %v", i, change.Table, tt.expected.Changes[i].Table)
					}
					if change.Impact != tt.expected.Changes[i].Impact {
						t.Errorf("Change[%d].Impact = %v, want %v", i, change.Impact, tt.expected.Changes[i].Impact)
					}
				}
			}
		})
	}
}

func TestDetectDrift(t *testing.T) {
	tests := []struct {
		name        string
		current     map[string]SimpleTableInfo
		snapshot    SchemaSnapshot
		wantChanges int
	}{
		{
			name: "no_drift",
			current: map[string]SimpleTableInfo{
				"users": {
					Name: "users",
					Type: "table",
					Columns: map[string]ColumnInfo{
						"id": {Name: "id", Type: "INT"},
					},
				},
			},
			snapshot: SchemaSnapshot{
				ID:        "snap-001",
				Timestamp: time.Now(),
				Profile:   "test",
				Tables: map[string]SimpleTableInfo{
					"users": {
						Name: "users",
						Type: "table",
						Columns: map[string]ColumnInfo{
							"id": {Name: "id", Type: "INT"},
						},
					},
				},
			},
			wantChanges: 0,
		},
		{
			name: "column_added_drift",
			current: map[string]SimpleTableInfo{
				"users": {
					Name: "users",
					Type: "table",
					Columns: map[string]ColumnInfo{
						"id":    {Name: "id", Type: "INT"},
						"email": {Name: "email", Type: "VARCHAR(255)"},
					},
				},
			},
			snapshot: SchemaSnapshot{
				ID:        "snap-001",
				Timestamp: time.Now(),
				Profile:   "test",
				Tables: map[string]SimpleTableInfo{
					"users": {
						Name: "users",
						Type: "table",
						Columns: map[string]ColumnInfo{
							"id": {Name: "id", Type: "INT"},
						},
					},
				},
			},
			wantChanges: 1,
		},
		{
			name: "table_added_drift",
			current: map[string]SimpleTableInfo{
				"users": {
					Name: "users",
					Type: "table",
					Columns: map[string]ColumnInfo{
						"id": {Name: "id", Type: "INT"},
					},
				},
				"orders": {
					Name: "orders",
					Type: "table",
					Columns: map[string]ColumnInfo{
						"id": {Name: "id", Type: "INT"},
					},
				},
			},
			snapshot: SchemaSnapshot{
				ID:        "snap-001",
				Timestamp: time.Now(),
				Profile:   "test",
				Tables: map[string]SimpleTableInfo{
					"users": {
						Name: "users",
						Type: "table",
						Columns: map[string]ColumnInfo{
							"id": {Name: "id", Type: "INT"},
						},
					},
				},
			},
			wantChanges: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, _ := ComputeTablesHash(tt.snapshot.Tables)
			tt.snapshot.TablesHash = hash

			changes := DetectDrift(tt.current, tt.snapshot)

			if len(changes) != tt.wantChanges {
				t.Errorf("DetectDrift() returned %d changes, want %d", len(changes), tt.wantChanges)
			}
		})
	}
}
