package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// SaveSnapshot stores a schema snapshot to the filesystem
// Snapshots are organized as: <storageDir>/<profile>/<id>.json
func SaveSnapshot(snapshot SchemaSnapshot, storageDir string) error {
	// Validate snapshot before saving
	if err := snapshot.Validate(); err != nil {
		return fmt.Errorf("snapshot validation failed: %w", err)
	}

	// Compute tables hash if not already set
	if snapshot.TablesHash == "" {
		hash, err := ComputeTablesHash(snapshot.Tables)
		if err != nil {
			return fmt.Errorf("failed to compute tables hash: %w", err)
		}
		snapshot.TablesHash = hash
	}

	// Create profile directory
	profileDir := filepath.Join(storageDir, snapshot.Profile)
	if err := os.MkdirAll(profileDir, 0750); err != nil {
		return fmt.Errorf("failed to create profile directory: %w", err)
	}

	// Marshal snapshot to JSON
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal snapshot: %w", err)
	}

	// Write snapshot file
	filename := filepath.Join(profileDir, snapshot.ID+".json")
	if err := os.WriteFile(filename, data, 0600); err != nil {
		return fmt.Errorf("failed to write snapshot file: %w", err)
	}

	return nil
}

// GetSnapshot retrieves a schema snapshot by profile and ID
func GetSnapshot(profile, snapshotID, storageDir string) (*SchemaSnapshot, error) {
	// Validate inputs to prevent path traversal
	if profile == "" || snapshotID == "" || storageDir == "" {
		return nil, fmt.Errorf("invalid input parameters")
	}

	// Validate profile name (prevent path traversal)
	if containsPathTraversal(profile) {
		return nil, fmt.Errorf("invalid profile name")
	}

	// Validate snapshot ID (prevent path traversal)
	if containsPathTraversal(snapshotID) {
		return nil, fmt.Errorf("invalid snapshot ID")
	}

	filename := filepath.Join(storageDir, profile, snapshotID+".json")

	// Read snapshot file (inputs already validated for path traversal above)
	// #nosec G304 -- Input validation prevents path traversal
	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("snapshot not found: %s/%s", profile, snapshotID)
		}
		return nil, fmt.Errorf("failed to read snapshot file: %w", err)
	}

	// Unmarshal snapshot
	var snapshot SchemaSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, fmt.Errorf("failed to unmarshal snapshot: %w", err)
	}

	// Validate loaded snapshot
	if err := snapshot.Validate(); err != nil {
		return nil, fmt.Errorf("loaded snapshot is invalid: %w", err)
	}

	return &snapshot, nil
}

// ListSnapshots returns snapshots for a profile, sorted by timestamp (newest first)
// If limit is 0 or negative, returns all snapshots
func ListSnapshots(profile string, limit int, storageDir string) ([]SchemaSnapshot, error) {
	profileDir := filepath.Join(storageDir, profile)

	// Check if profile directory exists
	if _, err := os.Stat(profileDir); os.IsNotExist(err) {
		return []SchemaSnapshot{}, nil
	}

	// Read directory entries
	entries, err := os.ReadDir(profileDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read profile directory: %w", err)
	}

	var snapshots []SchemaSnapshot

	// Load all snapshots
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Only process .json files
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		// Extract snapshot ID from filename
		snapshotID := entry.Name()[:len(entry.Name())-5]

		snapshot, err := GetSnapshot(profile, snapshotID, storageDir)
		if err != nil {
			// Skip invalid snapshots
			continue
		}

		snapshots = append(snapshots, *snapshot)
	}

	// Sort by timestamp (newest first)
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].Timestamp.After(snapshots[j].Timestamp)
	})

	// Apply limit
	if limit > 0 && limit < len(snapshots) {
		snapshots = snapshots[:limit]
	}

	return snapshots, nil
}

// CompareSnapshots compares two schema snapshots and returns the differences
func CompareSnapshots(old, new SchemaSnapshot) SchemaDiff {
	diff := SchemaDiff{
		AddedTables:    []string{},
		RemovedTables:  []string{},
		ModifiedTables: []TableDiff{},
		Changes:        []SchemaChange{},
	}

	// Find added tables
	for tableName := range new.Tables {
		if _, exists := old.Tables[tableName]; !exists {
			diff.AddedTables = append(diff.AddedTables, tableName)
			diff.Changes = append(diff.Changes, SchemaChange{
				Type:   ChangeTypeAddTable,
				Table:  tableName,
				Impact: "compatible",
			})
		}
	}

	// Find removed tables
	for tableName := range old.Tables {
		if _, exists := new.Tables[tableName]; !exists {
			diff.RemovedTables = append(diff.RemovedTables, tableName)
			diff.Changes = append(diff.Changes, SchemaChange{
				Type:   ChangeTypeDropTable,
				Table:  tableName,
				Impact: "breaking",
			})
		}
	}

	// Find modified tables
	for tableName := range old.Tables {
		if newTable, exists := new.Tables[tableName]; exists {
			oldTable := old.Tables[tableName]

			tableDiff := compareTableColumns(tableName, oldTable, newTable)
			if len(tableDiff.AddedCols) > 0 || len(tableDiff.DroppedCols) > 0 || len(tableDiff.ModifiedCols) > 0 {
				diff.ModifiedTables = append(diff.ModifiedTables, tableDiff)
				diff.Changes = append(diff.Changes, tableDiff.AddedCols...)
				diff.Changes = append(diff.Changes, tableDiff.DroppedCols...)
				diff.Changes = append(diff.Changes, tableDiff.ModifiedCols...)
			}
		}
	}

	return diff
}

// compareTableColumns compares columns of two tables and returns differences
func compareTableColumns(tableName string, old, new SimpleTableInfo) TableDiff {
	diff := TableDiff{
		TableName:    tableName,
		AddedCols:    []SchemaChange{},
		DroppedCols:  []SchemaChange{},
		ModifiedCols: []SchemaChange{},
	}

	// Find added columns
	for colName := range new.Columns {
		if _, exists := old.Columns[colName]; !exists {
			diff.AddedCols = append(diff.AddedCols, SchemaChange{
				Type:   ChangeTypeAddColumn,
				Table:  tableName,
				Column: colName,
				Impact: "compatible",
			})
		}
	}

	// Find dropped columns
	for colName := range old.Columns {
		if _, exists := new.Columns[colName]; !exists {
			diff.DroppedCols = append(diff.DroppedCols, SchemaChange{
				Type:   ChangeTypeDropColumn,
				Table:  tableName,
				Column: colName,
				Impact: "breaking",
			})
		}
	}

	// Find modified columns
	for colName, newCol := range new.Columns {
		if oldCol, exists := old.Columns[colName]; exists {
			if oldCol.Type != newCol.Type {
				diff.ModifiedCols = append(diff.ModifiedCols, SchemaChange{
					Type:     ChangeTypeAlterType,
					Table:    tableName,
					Column:   colName,
					OldValue: oldCol.Type,
					NewValue: newCol.Type,
					Impact:   "compatible", // Type changes are typically compatible
				})
			}

			// Check nullable change
			if oldCol.Nullable != newCol.Nullable {
				impact := "compatible"
				if !newCol.Nullable {
					impact = "breaking" // Making a column non-nullable is breaking
				}
				diff.ModifiedCols = append(diff.ModifiedCols, SchemaChange{
					Type:     ChangeTypeAlterConstraint,
					Table:    tableName,
					Column:   colName,
					OldValue: oldCol.Nullable,
					NewValue: newCol.Nullable,
					Impact:   impact,
				})
			}
		}
	}

	return diff
}

// DetectDrift compares current schema state with the last snapshot
// Returns list of schema changes that represent drift
func DetectDrift(current map[string]SimpleTableInfo, lastSnapshot SchemaSnapshot) []SchemaChange {
	// Create a synthetic snapshot from current state
	currentSnapshot := SchemaSnapshot{
		ID:        "current",
		Timestamp: time.Now(),
		Profile:   lastSnapshot.Profile,
		Tables:    current,
	}

	// Compare snapshots
	diff := CompareSnapshots(lastSnapshot, currentSnapshot)

	return diff.Changes
}

// containsPathTraversal checks if a string contains path traversal patterns
func containsPathTraversal(s string) bool {
	return strings.Contains(s, "..") ||
		strings.Contains(s, "\\") ||
		strings.Contains(s, "/") && strings.Contains(s, "..")
}
