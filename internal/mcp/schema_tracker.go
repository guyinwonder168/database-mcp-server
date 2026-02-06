package mcp

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	defaultSchemaSnapshotRetentionDays = 30
	defaultSchemaHistoryLimit          = 20
	defaultSchemaStorageDirName        = ".schema-snapshots"
	schemaStorageDirEnv                = "DB_MCP_SCHEMA_STORAGE_DIR"

	schemaTrackOperationTrack             = "track"
	schemaTrackOperationHistory           = "history"
	schemaTrackOperationGenerateMigration = "generate_migration"
	schemaTrackOperationDetectDrift       = "detect_drift"
)

// TrackSchemaChangesParams defines input for track-schema-changes operations.
type TrackSchemaChangesParams struct {
	ProfileName    string   `json:"profile_name" jsonschema:"profile to use for schema tracking"`
	DatabaseName   string   `json:"database_name,omitempty" jsonschema:"optional database/schema override"`
	Operation      string   `json:"operation,omitempty" jsonschema:"operation: track|history|generate_migration|detect_drift (default: track)"`
	Dialect        string   `json:"dialect,omitempty" jsonschema:"SQL dialect override for migration generation (mysql|postgresql|sqlite)"`
	FromSnapshotID string   `json:"from_snapshot_id,omitempty" jsonschema:"source snapshot ID for migration generation"`
	ToSnapshotID   string   `json:"to_snapshot_id,omitempty" jsonschema:"target snapshot ID for migration generation"`
	SnapshotID     string   `json:"snapshot_id,omitempty" jsonschema:"baseline snapshot ID for drift detection"`
	Limit          int      `json:"limit,omitempty" jsonschema:"maximum number of history snapshots to return (default 20)"`
	RetentionDays  int      `json:"retention_days,omitempty" jsonschema:"snapshot retention window in days (default 30)"`
	ChangeTypes    []string `json:"change_types,omitempty" jsonschema:"reserved for future filtering compatibility"`
	TimeRange      string   `json:"time_range,omitempty" jsonschema:"reserved for future filtering compatibility"`
}

// TrackSchemaChangesResult is the response payload for track-schema-changes operations.
type TrackSchemaChangesResult struct {
	Operation           string            `json:"operation"`
	ProfileName         string            `json:"profile_name"`
	DatabaseName        string            `json:"database_name,omitempty"`
	FromSnapshotID      string            `json:"from_snapshot_id,omitempty"`
	ToSnapshotID        string            `json:"to_snapshot_id,omitempty"`
	SnapshotID          string            `json:"snapshot_id,omitempty"`
	PreviousSnapshotID  string            `json:"previous_snapshot_id,omitempty"`
	Changes             []SchemaChange    `json:"changes,omitempty"`
	Diff                *SchemaDiff       `json:"diff,omitempty"`
	History             []SchemaSnapshot  `json:"history,omitempty"`
	Migration           *MigrationScript  `json:"migration,omitempty"`
	MigrationValidation []ValidationError `json:"migration_validation,omitempty"`
	MigrationImpact     *MigrationImpact  `json:"migration_impact,omitempty"`
	DriftDetected       bool              `json:"drift_detected,omitempty"`
	RetentionDays       int               `json:"retention_days,omitempty"`
	Summary             string            `json:"summary"`
}

type schemaObject struct {
	Name string
	Type string
}

func (s *MCPServer) handleTrackSchemaChanges(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input TrackSchemaChangesParams,
) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(input.ProfileName) == "" {
		structErr := NewStructuredError(
			ErrorCodeMissingParameter,
			"Missing required parameter",
			"profile_name is required for track-schema-changes",
		)
		return errorResult(structErr), nil, nil
	}

	switch normalizeSchemaTrackOperation(input.Operation) {
	case schemaTrackOperationTrack:
		return s.handleTrackSchemaSnapshot(ctx, input)
	case schemaTrackOperationHistory:
		return s.handleGetSchemaHistory(ctx, input)
	case schemaTrackOperationGenerateMigration:
		return s.handleGenerateMigration(ctx, input)
	case schemaTrackOperationDetectDrift:
		return s.handleDetectSchemaDrift(ctx, input)
	default:
		structErr := NewStructuredError(
			ErrorCodeInvalidInput,
			"Invalid operation",
			fmt.Sprintf("operation must be one of: %s, %s, %s, %s", schemaTrackOperationTrack, schemaTrackOperationHistory, schemaTrackOperationGenerateMigration, schemaTrackOperationDetectDrift),
		).WithContext("operation", input.Operation)
		return errorResult(structErr), nil, nil
	}
}

func normalizeSchemaTrackOperation(op string) string {
	normalized := strings.ToLower(strings.TrimSpace(op))
	switch normalized {
	case "", schemaTrackOperationTrack:
		return schemaTrackOperationTrack
	case schemaTrackOperationHistory:
		return schemaTrackOperationHistory
	case schemaTrackOperationGenerateMigration, "generate-migration", "migration":
		return schemaTrackOperationGenerateMigration
	case schemaTrackOperationDetectDrift, "detect-drift", "drift":
		return schemaTrackOperationDetectDrift
	default:
		return normalized
	}
}

func (s *MCPServer) handleTrackSchemaSnapshot(
	ctx context.Context,
	input TrackSchemaChangesParams,
) (*mcp.CallToolResult, any, error) {
	storageDir := s.schemaStorageDir()
	previousSnapshot, err := latestSnapshot(input.ProfileName, storageDir)
	if err != nil {
		return nil, nil, err
	}

	currentSnapshot, currentDBType, err := s.captureSchemaSnapshot(ctx, input.ProfileName, input.DatabaseName)
	if err != nil {
		return nil, nil, err
	}

	diff := SchemaDiff{Changes: []SchemaChange{}}
	previousID := ""
	if previousSnapshot != nil {
		previousID = previousSnapshot.ID
		diff = CompareSnapshots(*previousSnapshot, currentSnapshot)
	}

	if err := SaveSnapshot(currentSnapshot, storageDir); err != nil {
		return nil, nil, fmt.Errorf("failed to save schema snapshot: %w", err)
	}

	retentionDays := retentionDaysOrDefault(input.RetentionDays)
	if err := applySnapshotRetention(input.ProfileName, storageDir, retentionDays); err != nil {
		return nil, nil, err
	}

	dialect := chooseMigrationDialect(input.Dialect, currentDBType)
	migration := GenerateMigration(diff, dialect)
	migration.FromVersion = previousIDOrBaseline(previousID)
	migration.ToVersion = currentSnapshot.ID

	validation := ValidateMigration(migration)
	impact := EstimateMigrationImpact(migration)

	var summary string
	if previousSnapshot == nil {
		summary = fmt.Sprintf("Initial schema snapshot captured with %d table(s)", len(currentSnapshot.Tables))
	} else {
		summary = fmt.Sprintf("Schema changes tracked: %d change(s) detected", len(diff.Changes))
	}

	result := TrackSchemaChangesResult{
		Operation:           schemaTrackOperationTrack,
		ProfileName:         input.ProfileName,
		DatabaseName:        input.DatabaseName,
		SnapshotID:          currentSnapshot.ID,
		PreviousSnapshotID:  previousID,
		Changes:             diff.Changes,
		Diff:                &diff,
		Migration:           &migration,
		MigrationValidation: validation,
		MigrationImpact:     &impact,
		RetentionDays:       retentionDays,
		Summary:             summary,
	}

	return marshalTrackSchemaResult(result)
}

func (s *MCPServer) handleGetSchemaHistory(
	_ context.Context,
	input TrackSchemaChangesParams,
) (*mcp.CallToolResult, any, error) {
	limit := input.Limit
	if limit <= 0 {
		limit = defaultSchemaHistoryLimit
	}

	history, err := ListSnapshots(input.ProfileName, limit, s.schemaStorageDir())
	if err != nil {
		return nil, nil, err
	}

	result := TrackSchemaChangesResult{
		Operation:   schemaTrackOperationHistory,
		ProfileName: input.ProfileName,
		History:     history,
		Summary:     fmt.Sprintf("Found %d schema snapshot(s)", len(history)),
	}

	return marshalTrackSchemaResult(result)
}

func (s *MCPServer) handleGenerateMigration(
	_ context.Context,
	input TrackSchemaChangesParams,
) (*mcp.CallToolResult, any, error) {
	fromSnapshot, toSnapshot, err := resolveMigrationSnapshots(input, s.schemaStorageDir())
	if err != nil {
		structErr := NewStructuredError(
			ErrorCodeInvalidInput,
			"Unable to resolve snapshots for migration",
			err.Error(),
		)
		return errorResult(structErr), nil, nil
	}

	dialect, err := s.resolveDialect(input.ProfileName, input.Dialect)
	if err != nil {
		structErr := NewStructuredError(
			ErrorCodeInvalidInput,
			"Unable to resolve SQL dialect",
			err.Error(),
		)
		return errorResult(structErr), nil, nil
	}

	diff := CompareSnapshots(fromSnapshot, toSnapshot)
	migration := GenerateMigration(diff, dialect)
	migration.FromVersion = fromSnapshot.ID
	migration.ToVersion = toSnapshot.ID

	validation := ValidateMigration(migration)
	impact := EstimateMigrationImpact(migration)

	result := TrackSchemaChangesResult{
		Operation:           schemaTrackOperationGenerateMigration,
		ProfileName:         input.ProfileName,
		FromSnapshotID:      fromSnapshot.ID,
		ToSnapshotID:        toSnapshot.ID,
		Changes:             diff.Changes,
		Diff:                &diff,
		Migration:           &migration,
		MigrationValidation: validation,
		MigrationImpact:     &impact,
		Summary:             fmt.Sprintf("Migration generated with %d statement(s)", len(migration.Statements)),
	}

	return marshalTrackSchemaResult(result)
}

func (s *MCPServer) handleDetectSchemaDrift(
	ctx context.Context,
	input TrackSchemaChangesParams,
) (*mcp.CallToolResult, any, error) {
	baselineSnapshot, err := resolveDriftBaseline(input.ProfileName, input.SnapshotID, s.schemaStorageDir())
	if err != nil {
		structErr := NewStructuredError(
			ErrorCodeInvalidInput,
			"Unable to resolve baseline snapshot",
			err.Error(),
		)
		return errorResult(structErr), nil, nil
	}

	currentSnapshot, _, err := s.captureSchemaSnapshot(ctx, input.ProfileName, input.DatabaseName)
	if err != nil {
		return nil, nil, err
	}

	changes := DetectDrift(currentSnapshot.Tables, baselineSnapshot)
	result := TrackSchemaChangesResult{
		Operation:          schemaTrackOperationDetectDrift,
		ProfileName:        input.ProfileName,
		SnapshotID:         baselineSnapshot.ID,
		DriftDetected:      len(changes) > 0,
		Changes:            changes,
		PreviousSnapshotID: baselineSnapshot.ID,
		Summary:            fmt.Sprintf("Schema drift detection completed: %d change(s)", len(changes)),
	}

	return marshalTrackSchemaResult(result)
}

func (s *MCPServer) captureSchemaSnapshot(
	ctx context.Context,
	profileName, databaseName string,
) (SchemaSnapshot, string, error) {
	conn, prof, err := s.openConnection(ctx, profileName, databaseName)
	if err != nil {
		return SchemaSnapshot{}, "", err
	}
	defer conn.Close() //nolint:errcheck // Standard pattern: error in deferred close is not critical

	objects, err := listSchemaObjects(ctx, conn, prof.DBType)
	if err != nil {
		return SchemaSnapshot{}, "", err
	}
	if len(objects) == 0 {
		return SchemaSnapshot{}, "", fmt.Errorf("no schema objects found")
	}

	tables := make(map[string]SimpleTableInfo, len(objects))
	rawDDL := make(map[string]string)
	for _, object := range objects {
		cols, err := s.getTableColumns(ctx, conn, prof, object.Name)
		if err != nil {
			return SchemaSnapshot{}, "", fmt.Errorf("failed to read columns for %s: %w", object.Name, err)
		}

		tables[object.Name] = SimpleTableInfo{
			Name:    object.Name,
			Type:    object.Type,
			Columns: toColumnMap(cols),
		}

		if ddl := readSQLiteDDL(ctx, conn, prof.DBType, object.Name); ddl != "" {
			rawDDL[object.Name] = ddl
		}
	}

	snapshot := SchemaSnapshot{
		ID:        fmt.Sprintf("snap-%d", time.Now().UTC().UnixNano()),
		Timestamp: time.Now().UTC(),
		Profile:   profileName,
		Tables:    tables,
	}
	if len(rawDDL) > 0 {
		snapshot.RawDDL = rawDDL
	}

	hash, err := ComputeTablesHash(snapshot.Tables)
	if err != nil {
		return SchemaSnapshot{}, "", err
	}
	snapshot.TablesHash = hash

	return snapshot, prof.DBType, nil
}

func listSchemaObjects(ctx context.Context, conn *sql.DB, dbType string) ([]schemaObject, error) {
	switch dbType {
	case "mysql", "mariadb":
		return listSchemaObjectsMySQL(ctx, conn)
	case "postgres":
		return listSchemaObjectsPostgreSQL(ctx, conn)
	case "sqlite":
		return listSchemaObjectsSQLite(ctx, conn)
	default:
		return nil, fmt.Errorf("unsupported db_type: %s", dbType)
	}
}

func listSchemaObjectsMySQL(ctx context.Context, conn *sql.DB) ([]schemaObject, error) {
	rows, err := conn.QueryContext(ctx, "SHOW FULL TABLES")
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck // Standard pattern: error in deferred close is not critical

	var objects []schemaObject
	for rows.Next() {
		var name, objectType string
		if rows.Scan(&name, &objectType) != nil {
			continue
		}
		objects = append(objects, schemaObject{Name: name, Type: strings.ToLower(objectType)})
	}

	return objects, nil
}

func listSchemaObjectsPostgreSQL(ctx context.Context, conn *sql.DB) ([]schemaObject, error) {
	rows, err := conn.QueryContext(ctx, `
		SELECT table_name, table_type
		FROM information_schema.tables
		WHERE table_schema = 'public'
		ORDER BY table_name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck // Standard pattern: error in deferred close is not critical

	var objects []schemaObject
	for rows.Next() {
		var name, objectType string
		if rows.Scan(&name, &objectType) != nil {
			continue
		}
		objects = append(objects, schemaObject{Name: name, Type: strings.ToLower(objectType)})
	}

	return objects, nil
}

func listSchemaObjectsSQLite(ctx context.Context, conn *sql.DB) ([]schemaObject, error) {
	rows, err := conn.QueryContext(ctx, `
		SELECT name, type
		FROM sqlite_master
		WHERE type IN ('table', 'view') AND name NOT LIKE 'sqlite_%'
		ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck // Standard pattern: error in deferred close is not critical

	var objects []schemaObject
	for rows.Next() {
		var name, objectType string
		if rows.Scan(&name, &objectType) != nil {
			continue
		}
		objects = append(objects, schemaObject{Name: name, Type: strings.ToLower(objectType)})
	}

	return objects, nil
}

func readSQLiteDDL(ctx context.Context, conn *sql.DB, dbType, objectName string) string {
	if dbType != "sqlite" {
		return ""
	}

	var ddl sql.NullString
	err := conn.QueryRowContext(ctx, "SELECT sql FROM sqlite_master WHERE name = ?", objectName).Scan(&ddl)
	if err != nil || !ddl.Valid {
		return ""
	}

	return ddl.String
}

func toColumnMap(columns []ColumnInfo) map[string]ColumnInfo {
	colMap := make(map[string]ColumnInfo, len(columns))
	for _, col := range columns {
		colMap[col.Name] = col
	}
	return colMap
}

func latestSnapshot(profileName, storageDir string) (*SchemaSnapshot, error) {
	snapshots, err := ListSnapshots(profileName, 1, storageDir)
	if err != nil {
		return nil, err
	}
	if len(snapshots) == 0 {
		return nil, nil
	}
	latest := snapshots[0]
	return &latest, nil
}

func applySnapshotRetention(profileName, storageDir string, retentionDays int) error {
	if retentionDays <= 0 {
		retentionDays = defaultSchemaSnapshotRetentionDays
	}

	snapshots, err := ListSnapshots(profileName, 0, storageDir)
	if err != nil {
		return err
	}

	cutoff := time.Now().UTC().AddDate(0, 0, -retentionDays)
	for _, snapshot := range snapshots {
		if snapshot.Timestamp.After(cutoff) {
			continue
		}
		filename := filepath.Join(storageDir, profileName, snapshot.ID+".json")
		if removeErr := os.Remove(filename); removeErr != nil && !os.IsNotExist(removeErr) {
			return fmt.Errorf("failed to remove expired snapshot %s: %w", snapshot.ID, removeErr)
		}
	}

	return nil
}

func resolveMigrationSnapshots(input TrackSchemaChangesParams, storageDir string) (SchemaSnapshot, SchemaSnapshot, error) {
	if input.FromSnapshotID != "" || input.ToSnapshotID != "" {
		if input.FromSnapshotID == "" || input.ToSnapshotID == "" {
			return SchemaSnapshot{}, SchemaSnapshot{}, fmt.Errorf("both from_snapshot_id and to_snapshot_id are required")
		}
		fromSnapshot, err := GetSnapshot(input.ProfileName, input.FromSnapshotID, storageDir)
		if err != nil {
			return SchemaSnapshot{}, SchemaSnapshot{}, err
		}
		toSnapshot, err := GetSnapshot(input.ProfileName, input.ToSnapshotID, storageDir)
		if err != nil {
			return SchemaSnapshot{}, SchemaSnapshot{}, err
		}
		return *fromSnapshot, *toSnapshot, nil
	}

	latestTwo, err := ListSnapshots(input.ProfileName, 2, storageDir)
	if err != nil {
		return SchemaSnapshot{}, SchemaSnapshot{}, err
	}
	if len(latestTwo) < 2 {
		return SchemaSnapshot{}, SchemaSnapshot{}, fmt.Errorf("at least two snapshots are required")
	}

	return latestTwo[1], latestTwo[0], nil
}

func resolveDriftBaseline(profileName, snapshotID, storageDir string) (SchemaSnapshot, error) {
	if strings.TrimSpace(snapshotID) != "" {
		snapshot, err := GetSnapshot(profileName, snapshotID, storageDir)
		if err != nil {
			return SchemaSnapshot{}, err
		}
		return *snapshot, nil
	}

	latest, err := latestSnapshot(profileName, storageDir)
	if err != nil {
		return SchemaSnapshot{}, err
	}
	if latest == nil {
		return SchemaSnapshot{}, fmt.Errorf("no baseline snapshot found")
	}

	return *latest, nil
}

func chooseMigrationDialect(explicitDialect, profileDBType string) string {
	if strings.TrimSpace(explicitDialect) != "" {
		return explicitDialect
	}
	return profileDBType
}

func (s *MCPServer) resolveDialect(profileName, explicitDialect string) (string, error) {
	if strings.TrimSpace(explicitDialect) != "" {
		return explicitDialect, nil
	}

	_, prof, err := s.findProfile(profileName)
	if err != nil {
		return "", err
	}
	return prof.DBType, nil
}

func previousIDOrBaseline(previousID string) string {
	if previousID == "" {
		return "baseline"
	}
	return previousID
}

func retentionDaysOrDefault(retentionDays int) int {
	if retentionDays <= 0 {
		return defaultSchemaSnapshotRetentionDays
	}
	return retentionDays
}

func (s *MCPServer) schemaStorageDir() string {
	if path := strings.TrimSpace(os.Getenv(schemaStorageDirEnv)); path != "" {
		return path
	}

	baseDir := filepath.Dir(s.ConfigPath)
	if strings.TrimSpace(baseDir) == "" || baseDir == "." {
		return defaultSchemaStorageDirName
	}
	return filepath.Join(baseDir, defaultSchemaStorageDirName)
}

func marshalTrackSchemaResult(result TrackSchemaChangesResult) (*mcp.CallToolResult, any, error) {
	data, err := json.Marshal(result)
	if err != nil {
		return nil, nil, err
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(data)}},
	}, nil, nil
}
