//go:build cgo

package mcp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHandleTrackSchemaChanges_ErrorBranches(t *testing.T) {
	testConfig := setupTestConfig(t)
	defer os.Remove(testConfig)

	t.Setenv(schemaStorageDirEnv, t.TempDir())
	server := NewMCPServerWithConfig(testConfig)
	ctx := context.Background()

	missingProfileRes, _, err := server.handleTrackSchemaChanges(ctx, nil, TrackSchemaChangesParams{})
	if err != nil {
		t.Fatalf("missing profile call failed: %v", err)
	}
	missingProfilePayload := extractToolResultText(t, missingProfileRes)
	if !strings.Contains(missingProfilePayload, string(ErrorCodeMissingParameter)) {
		t.Fatalf("expected MISSING_PARAMETER error, got %s", missingProfilePayload)
	}

	historyRes, _, err := server.handleTrackSchemaChanges(ctx, nil, TrackSchemaChangesParams{
		ProfileName: "testsqlite",
		Operation:   schemaTrackOperationHistory,
	})
	if err != nil {
		t.Fatalf("history operation failed: %v", err)
	}
	history := decodeTrackSchemaResult(t, historyRes)
	if len(history.History) != 0 {
		t.Fatalf("history size = %d, want 0", len(history.History))
	}

	migrationRes, _, err := server.handleTrackSchemaChanges(ctx, nil, TrackSchemaChangesParams{
		ProfileName: "testsqlite",
		Operation:   schemaTrackOperationGenerateMigration,
	})
	if err != nil {
		t.Fatalf("generate migration error branch failed: %v", err)
	}
	migrationPayload := extractToolResultText(t, migrationRes)
	if !strings.Contains(migrationPayload, string(ErrorCodeInvalidInput)) {
		t.Fatalf("expected INVALID_INPUT error, got %s", migrationPayload)
	}

	driftRes, _, err := server.handleTrackSchemaChanges(ctx, nil, TrackSchemaChangesParams{
		ProfileName: "testsqlite",
		Operation:   schemaTrackOperationDetectDrift,
	})
	if err != nil {
		t.Fatalf("detect drift error branch failed: %v", err)
	}
	driftPayload := extractToolResultText(t, driftRes)
	if !strings.Contains(driftPayload, string(ErrorCodeInvalidInput)) {
		t.Fatalf("expected INVALID_INPUT error, got %s", driftPayload)
	}
}

func TestResolveMigrationSnapshots(t *testing.T) {
	storageDir := t.TempDir()
	profile := "snapshot-profile"

	oldSnapshot := trackerTestSnapshot(profile, "snap-old", time.Now().UTC().Add(-2*time.Hour))
	newSnapshot := trackerTestSnapshot(profile, "snap-new", time.Now().UTC().Add(-1*time.Hour))

	if err := SaveSnapshot(oldSnapshot, storageDir); err != nil {
		t.Fatalf("save old snapshot failed: %v", err)
	}
	if err := SaveSnapshot(newSnapshot, storageDir); err != nil {
		t.Fatalf("save new snapshot failed: %v", err)
	}

	fromSnapshot, toSnapshot, err := resolveMigrationSnapshots(TrackSchemaChangesParams{
		ProfileName:    profile,
		FromSnapshotID: oldSnapshot.ID,
		ToSnapshotID:   newSnapshot.ID,
	}, storageDir)
	if err != nil {
		t.Fatalf("resolve explicit snapshots failed: %v", err)
	}
	if fromSnapshot.ID != oldSnapshot.ID || toSnapshot.ID != newSnapshot.ID {
		t.Fatalf("explicit snapshots = %s -> %s, want %s -> %s", fromSnapshot.ID, toSnapshot.ID, oldSnapshot.ID, newSnapshot.ID)
	}

	fromSnapshot, toSnapshot, err = resolveMigrationSnapshots(TrackSchemaChangesParams{
		ProfileName: profile,
	}, storageDir)
	if err != nil {
		t.Fatalf("resolve latest snapshots failed: %v", err)
	}
	if fromSnapshot.ID != oldSnapshot.ID || toSnapshot.ID != newSnapshot.ID {
		t.Fatalf("latest snapshots = %s -> %s, want %s -> %s", fromSnapshot.ID, toSnapshot.ID, oldSnapshot.ID, newSnapshot.ID)
	}

	_, _, err = resolveMigrationSnapshots(TrackSchemaChangesParams{
		ProfileName:    profile,
		FromSnapshotID: oldSnapshot.ID,
	}, storageDir)
	if err == nil {
		t.Fatalf("expected error when only one explicit snapshot ID is provided")
	}

	oneSnapshotDir := t.TempDir()
	if err := SaveSnapshot(oldSnapshot, oneSnapshotDir); err != nil {
		t.Fatalf("save single snapshot failed: %v", err)
	}
	_, _, err = resolveMigrationSnapshots(TrackSchemaChangesParams{
		ProfileName: profile,
	}, oneSnapshotDir)
	if err == nil {
		t.Fatalf("expected error when less than two snapshots exist")
	}
}

func TestResolveDriftBaseline(t *testing.T) {
	storageDir := t.TempDir()
	profile := "drift-profile"

	oldSnapshot := trackerTestSnapshot(profile, "snap-old", time.Now().UTC().Add(-2*time.Hour))
	newSnapshot := trackerTestSnapshot(profile, "snap-new", time.Now().UTC().Add(-1*time.Hour))

	if err := SaveSnapshot(oldSnapshot, storageDir); err != nil {
		t.Fatalf("save old snapshot failed: %v", err)
	}
	if err := SaveSnapshot(newSnapshot, storageDir); err != nil {
		t.Fatalf("save new snapshot failed: %v", err)
	}

	baseline, err := resolveDriftBaseline(profile, oldSnapshot.ID, storageDir)
	if err != nil {
		t.Fatalf("resolve explicit baseline failed: %v", err)
	}
	if baseline.ID != oldSnapshot.ID {
		t.Fatalf("explicit baseline = %s, want %s", baseline.ID, oldSnapshot.ID)
	}

	baseline, err = resolveDriftBaseline(profile, "", storageDir)
	if err != nil {
		t.Fatalf("resolve latest baseline failed: %v", err)
	}
	if baseline.ID != newSnapshot.ID {
		t.Fatalf("latest baseline = %s, want %s", baseline.ID, newSnapshot.ID)
	}

	_, err = resolveDriftBaseline(profile, "missing", storageDir)
	if err == nil {
		t.Fatalf("expected error for missing explicit snapshot")
	}

	_, err = resolveDriftBaseline("empty-profile", "", storageDir)
	if err == nil {
		t.Fatalf("expected error when no baseline snapshots exist")
	}
}

func TestApplySnapshotRetention(t *testing.T) {
	storageDir := t.TempDir()
	profile := "retention-profile"

	expiredSnapshot := trackerTestSnapshot(profile, "snap-expired", time.Now().UTC().AddDate(0, 0, -45))
	freshSnapshot := trackerTestSnapshot(profile, "snap-fresh", time.Now().UTC())

	if err := SaveSnapshot(expiredSnapshot, storageDir); err != nil {
		t.Fatalf("save expired snapshot failed: %v", err)
	}
	if err := SaveSnapshot(freshSnapshot, storageDir); err != nil {
		t.Fatalf("save fresh snapshot failed: %v", err)
	}

	if err := applySnapshotRetention(profile, storageDir, 30); err != nil {
		t.Fatalf("apply retention failed: %v", err)
	}

	expiredPath := filepath.Join(storageDir, profile, expiredSnapshot.ID+".json")
	if _, err := os.Stat(expiredPath); !os.IsNotExist(err) {
		t.Fatalf("expired snapshot should be removed, stat err = %v", err)
	}

	freshPath := filepath.Join(storageDir, profile, freshSnapshot.ID+".json")
	if _, err := os.Stat(freshPath); err != nil {
		t.Fatalf("fresh snapshot should remain, stat err = %v", err)
	}

	if err := applySnapshotRetention(profile, storageDir, 0); err != nil {
		t.Fatalf("apply default retention failed: %v", err)
	}
}

func TestSchemaTrackerSimpleHelpers(t *testing.T) {
	if got := chooseMigrationDialect("postgres", "sqlite"); got != "postgres" {
		t.Fatalf("chooseMigrationDialect explicit = %q, want postgres", got)
	}
	if got := chooseMigrationDialect("", "sqlite"); got != "sqlite" {
		t.Fatalf("chooseMigrationDialect fallback = %q, want sqlite", got)
	}

	if got := previousIDOrBaseline(""); got != "baseline" {
		t.Fatalf("previousIDOrBaseline empty = %q, want baseline", got)
	}
	if got := previousIDOrBaseline("snap-1"); got != "snap-1" {
		t.Fatalf("previousIDOrBaseline set = %q, want snap-1", got)
	}

	if got := retentionDaysOrDefault(0); got != defaultSchemaSnapshotRetentionDays {
		t.Fatalf("retentionDaysOrDefault(0) = %d, want %d", got, defaultSchemaSnapshotRetentionDays)
	}
	if got := retentionDaysOrDefault(7); got != 7 {
		t.Fatalf("retentionDaysOrDefault(7) = %d, want 7", got)
	}

	res, _, err := marshalTrackSchemaResult(TrackSchemaChangesResult{
		Operation:   schemaTrackOperationHistory,
		ProfileName: "profile",
		Summary:     "ok",
	})
	if err != nil {
		t.Fatalf("marshalTrackSchemaResult failed: %v", err)
	}
	payload := extractToolResultText(t, res)
	if !strings.Contains(payload, `"operation":"history"`) {
		t.Fatalf("unexpected marshaled payload: %s", payload)
	}
}

func TestSchemaStorageDir(t *testing.T) {
	server := &MCPServer{ConfigPath: "config.yaml"}

	customDir := filepath.Join(t.TempDir(), "schema-snapshots")
	t.Setenv(schemaStorageDirEnv, customDir)
	if got := server.schemaStorageDir(); got != customDir {
		t.Fatalf("schemaStorageDir env override = %q, want %q", got, customDir)
	}

	t.Setenv(schemaStorageDirEnv, "  ")
	server.ConfigPath = "config.yaml"
	if got := server.schemaStorageDir(); got != defaultSchemaStorageDirName {
		t.Fatalf("schemaStorageDir default = %q, want %q", got, defaultSchemaStorageDirName)
	}

	server.ConfigPath = filepath.Join("/tmp", "project", "config.yaml")
	expected := filepath.Join("/tmp", "project", defaultSchemaStorageDirName)
	if got := server.schemaStorageDir(); got != expected {
		t.Fatalf("schemaStorageDir from config path = %q, want %q", got, expected)
	}
}

func TestResolveDialect(t *testing.T) {
	server := &MCPServer{}
	dialect, err := server.resolveDialect("missing", "mysql")
	if err != nil {
		t.Fatalf("resolveDialect explicit failed: %v", err)
	}
	if dialect != "mysql" {
		t.Fatalf("resolveDialect explicit = %q, want mysql", dialect)
	}

	testConfig := setupTestConfig(t)
	defer os.Remove(testConfig)

	server = NewMCPServerWithConfig(testConfig)
	dialect, err = server.resolveDialect("testsqlite", "")
	if err != nil {
		t.Fatalf("resolveDialect profile failed: %v", err)
	}
	if dialect != "sqlite" {
		t.Fatalf("resolveDialect profile = %q, want sqlite", dialect)
	}

	_, err = server.resolveDialect("missing-profile", "")
	if err == nil {
		t.Fatalf("expected error for missing profile")
	}
}

func TestListSchemaObjectsHelpers(t *testing.T) {
	testConfig := setupTestConfig(t)
	defer os.Remove(testConfig)

	server := NewMCPServerWithConfig(testConfig)
	ctx := context.Background()

	conn, _, err := server.openConnection(ctx, "testsqlite", "")
	if err != nil {
		t.Fatalf("openConnection failed: %v", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, "CREATE TABLE schema_obj_users (id INTEGER PRIMARY KEY)"); err != nil {
		t.Fatalf("create table failed: %v", err)
	}
	if _, err := conn.ExecContext(ctx, "CREATE VIEW schema_obj_users_view AS SELECT id FROM schema_obj_users"); err != nil {
		t.Fatalf("create view failed: %v", err)
	}

	objects, err := listSchemaObjects(ctx, conn, "sqlite")
	if err != nil {
		t.Fatalf("listSchemaObjects sqlite failed: %v", err)
	}
	if !containsSchemaObject(objects, "schema_obj_users", "table") {
		t.Fatalf("expected sqlite table in objects: %#v", objects)
	}
	if !containsSchemaObject(objects, "schema_obj_users_view", "view") {
		t.Fatalf("expected sqlite view in objects: %#v", objects)
	}

	if _, err := listSchemaObjects(ctx, conn, "unsupported"); err == nil {
		t.Fatalf("expected error for unsupported db type")
	}

	if _, err := listSchemaObjectsMySQL(ctx, conn); err == nil {
		t.Fatalf("expected mysql listing to fail on sqlite connection")
	}

	if _, err := conn.ExecContext(ctx, "ATTACH DATABASE ':memory:' AS information_schema"); err != nil {
		t.Fatalf("attach information_schema failed: %v", err)
	}
	if _, err := conn.ExecContext(ctx, "CREATE TABLE information_schema.tables (table_name TEXT, table_type TEXT, table_schema TEXT)"); err != nil {
		t.Fatalf("create information_schema.tables failed: %v", err)
	}
	if _, err := conn.ExecContext(ctx, "INSERT INTO information_schema.tables (table_name, table_type, table_schema) VALUES ('audit_log', 'BASE TABLE', 'public')"); err != nil {
		t.Fatalf("insert information_schema.tables failed: %v", err)
	}

	pgObjects, err := listSchemaObjectsPostgreSQL(ctx, conn)
	if err != nil {
		t.Fatalf("listSchemaObjectsPostgreSQL failed: %v", err)
	}
	if !containsSchemaObject(pgObjects, "audit_log", "base table") {
		t.Fatalf("expected postgres object in result: %#v", pgObjects)
	}
}

func TestReadSQLiteDDL(t *testing.T) {
	testConfig := setupTestConfig(t)
	defer os.Remove(testConfig)

	server := NewMCPServerWithConfig(testConfig)
	ctx := context.Background()

	conn, _, err := server.openConnection(ctx, "testsqlite", "")
	if err != nil {
		t.Fatalf("openConnection failed: %v", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, "CREATE TABLE ddl_users (id INTEGER PRIMARY KEY, name TEXT)"); err != nil {
		t.Fatalf("create table failed: %v", err)
	}

	ddl := readSQLiteDDL(ctx, conn, "sqlite", "ddl_users")
	if ddl == "" || !strings.Contains(strings.ToUpper(ddl), "CREATE TABLE") {
		t.Fatalf("unexpected sqlite ddl: %q", ddl)
	}

	if ddl := readSQLiteDDL(ctx, conn, "mysql", "ddl_users"); ddl != "" {
		t.Fatalf("non-sqlite ddl should be empty, got %q", ddl)
	}

	if ddl := readSQLiteDDL(ctx, conn, "sqlite", "missing_table"); ddl != "" {
		t.Fatalf("missing table ddl should be empty, got %q", ddl)
	}
}

func containsSchemaObject(objects []schemaObject, name, objectType string) bool {
	for _, object := range objects {
		if object.Name == name && object.Type == objectType {
			return true
		}
	}
	return false
}

func trackerTestSnapshot(profile, snapshotID string, timestamp time.Time) SchemaSnapshot {
	snapshot := sampleSnapshot(profile)
	snapshot.ID = snapshotID
	snapshot.Timestamp = timestamp
	return snapshot
}
