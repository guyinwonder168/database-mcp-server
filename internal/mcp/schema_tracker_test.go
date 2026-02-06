//go:build cgo

package mcp

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestHandleTrackSchemaChanges_TrackHistoryMigrationAndDrift(t *testing.T) {
	testConfig := setupTestConfig(t)
	defer os.Remove(testConfig)
	t.Setenv(schemaStorageDirEnv, t.TempDir())

	server := NewMCPServerWithConfig(testConfig)
	ctx := context.Background()

	_, _, err := server.handleExecuteSQL(ctx, nil, ExecuteSQLParams{
		ProfileName:  "testsqlite",
		DatabaseName: testSQLiteDBPath,
		SQL:          "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)",
	})
	if err != nil {
		t.Fatalf("failed to create users table: %v", err)
	}

	firstTrackRes, _, err := server.handleTrackSchemaChanges(ctx, nil, TrackSchemaChangesParams{
		ProfileName:  "testsqlite",
		DatabaseName: testSQLiteDBPath,
	})
	if err != nil {
		t.Fatalf("first track-schema-changes failed: %v", err)
	}
	firstTrack := decodeTrackSchemaResult(t, firstTrackRes)
	if firstTrack.Operation != schemaTrackOperationTrack {
		t.Fatalf("operation = %q, want %q", firstTrack.Operation, schemaTrackOperationTrack)
	}
	if firstTrack.SnapshotID == "" {
		t.Fatalf("expected snapshot ID from track operation")
	}
	if len(firstTrack.Changes) != 0 {
		t.Fatalf("expected no changes for initial snapshot, got %d", len(firstTrack.Changes))
	}

	_, _, err = server.handleExecuteSQL(ctx, nil, ExecuteSQLParams{
		ProfileName:  "testsqlite",
		DatabaseName: testSQLiteDBPath,
		SQL:          "ALTER TABLE users ADD COLUMN email TEXT",
	})
	if err != nil {
		t.Fatalf("failed to alter users table: %v", err)
	}

	secondTrackRes, _, err := server.handleTrackSchemaChanges(ctx, nil, TrackSchemaChangesParams{
		ProfileName:  "testsqlite",
		DatabaseName: testSQLiteDBPath,
	})
	if err != nil {
		t.Fatalf("second track-schema-changes failed: %v", err)
	}
	secondTrack := decodeTrackSchemaResult(t, secondTrackRes)
	if secondTrack.PreviousSnapshotID != firstTrack.SnapshotID {
		t.Fatalf("previous_snapshot_id = %q, want %q", secondTrack.PreviousSnapshotID, firstTrack.SnapshotID)
	}
	if !hasSchemaChangeType(secondTrack.Changes, ChangeTypeAddColumn) {
		t.Fatalf("expected add_column change, got %#v", secondTrack.Changes)
	}
	if secondTrack.Migration == nil || len(secondTrack.Migration.Statements) == 0 {
		t.Fatalf("expected migration script from track operation")
	}

	historyRes, _, err := server.handleTrackSchemaChanges(ctx, nil, TrackSchemaChangesParams{
		ProfileName: "testsqlite",
		Operation:   schemaTrackOperationHistory,
		Limit:       2,
	})
	if err != nil {
		t.Fatalf("history operation failed: %v", err)
	}
	history := decodeTrackSchemaResult(t, historyRes)
	if len(history.History) != 2 {
		t.Fatalf("history size = %d, want 2", len(history.History))
	}
	if history.History[0].ID != secondTrack.SnapshotID {
		t.Fatalf("latest snapshot = %q, want %q", history.History[0].ID, secondTrack.SnapshotID)
	}

	migrationRes, _, err := server.handleTrackSchemaChanges(ctx, nil, TrackSchemaChangesParams{
		ProfileName:    "testsqlite",
		Operation:      schemaTrackOperationGenerateMigration,
		FromSnapshotID: firstTrack.SnapshotID,
		ToSnapshotID:   secondTrack.SnapshotID,
		Dialect:        "sqlite",
	})
	if err != nil {
		t.Fatalf("generate_migration operation failed: %v", err)
	}
	migration := decodeTrackSchemaResult(t, migrationRes)
	if migration.Migration == nil {
		t.Fatalf("expected migration in response")
	}
	if len(migration.MigrationValidation) != 0 {
		t.Fatalf("expected no migration validation errors, got %#v", migration.MigrationValidation)
	}
	if len(migration.Migration.Statements) == 0 || !strings.Contains(strings.ToUpper(migration.Migration.Statements[0]), "ADD COLUMN") {
		t.Fatalf("unexpected migration statements: %#v", migration.Migration.Statements)
	}

	_, _, err = server.handleExecuteSQL(ctx, nil, ExecuteSQLParams{
		ProfileName:  "testsqlite",
		DatabaseName: testSQLiteDBPath,
		SQL:          "ALTER TABLE users ADD COLUMN phone TEXT",
	})
	if err != nil {
		t.Fatalf("failed to alter users table for drift test: %v", err)
	}

	driftRes, _, err := server.handleTrackSchemaChanges(ctx, nil, TrackSchemaChangesParams{
		ProfileName:  "testsqlite",
		DatabaseName: testSQLiteDBPath,
		Operation:    schemaTrackOperationDetectDrift,
		SnapshotID:   secondTrack.SnapshotID,
	})
	if err != nil {
		t.Fatalf("detect_drift operation failed: %v", err)
	}
	drift := decodeTrackSchemaResult(t, driftRes)
	if !drift.DriftDetected {
		t.Fatalf("expected drift_detected=true")
	}
	if !hasSchemaChangeType(drift.Changes, ChangeTypeAddColumn) {
		t.Fatalf("expected drift changes to include add_column, got %#v", drift.Changes)
	}
}

func TestHandleTrackSchemaChanges_InvalidOperation(t *testing.T) {
	testConfig := setupTestConfig(t)
	defer os.Remove(testConfig)

	server := NewMCPServerWithConfig(testConfig)
	ctx := context.Background()

	res, _, err := server.handleTrackSchemaChanges(ctx, nil, TrackSchemaChangesParams{
		ProfileName: "testsqlite",
		Operation:   "unknown",
	})
	if err != nil {
		t.Fatalf("expected nil error for structured invalid operation response, got %v", err)
	}

	payload := extractToolResultText(t, res)
	if !strings.Contains(payload, string(ErrorCodeInvalidInput)) {
		t.Fatalf("expected INVALID_INPUT in response, got %s", payload)
	}
}

func TestNormalizeSchemaTrackOperation(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "", want: schemaTrackOperationTrack},
		{input: "track", want: schemaTrackOperationTrack},
		{input: "history", want: schemaTrackOperationHistory},
		{input: "migration", want: schemaTrackOperationGenerateMigration},
		{input: "generate-migration", want: schemaTrackOperationGenerateMigration},
		{input: "detect-drift", want: schemaTrackOperationDetectDrift},
		{input: "drift", want: schemaTrackOperationDetectDrift},
		{input: "other", want: "other"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeSchemaTrackOperation(tt.input)
			if got != tt.want {
				t.Fatalf("normalizeSchemaTrackOperation(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func decodeTrackSchemaResult(t *testing.T, res *mcp.CallToolResult) TrackSchemaChangesResult {
	t.Helper()

	payload := extractToolResultText(t, res)

	var result TrackSchemaChangesResult
	if err := json.Unmarshal([]byte(payload), &result); err != nil {
		t.Fatalf("failed to unmarshal TrackSchemaChangesResult: %v; payload=%s", err, payload)
	}

	return result
}

func extractToolResultText(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()

	if res == nil || len(res.Content) == 0 {
		t.Fatalf("tool result content is empty")
	}

	textContent, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", res.Content[0])
	}

	return textContent.Text
}

func hasSchemaChangeType(changes []SchemaChange, changeType SchemaChangeType) bool {
	for _, change := range changes {
		if change.Type == changeType {
			return true
		}
	}
	return false
}
