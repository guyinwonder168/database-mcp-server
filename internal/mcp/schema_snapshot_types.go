package mcp

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// SimpleTableInfo represents basic table info for schema snapshots
type SimpleTableInfo struct {
	Name    string            `json:"name"`
	Type    string            `json:"type"`
	Columns map[string]ColumnInfo `json:"columns,omitempty"`
}

// SchemaSnapshot represents a snapshot of database schema at a point in time
type SchemaSnapshot struct {
	ID         string                  `json:"id"`
	Timestamp  time.Time               `json:"timestamp"`
	Profile    string                  `json:"profile"`
	TablesHash string                  `json:"tables_hash"` // SHA-256 for integrity
	Tables     map[string]SimpleTableInfo `json:"tables"`
	RawDDL     map[string]string         `json:"raw_ddl,omitempty"`
}

// SchemaChangeType represents the type of schema change
type SchemaChangeType string

const (
	ChangeTypeAddColumn      SchemaChangeType = "add_column"
	ChangeTypeDropColumn     SchemaChangeType = "drop_column"
	ChangeTypeAlterType      SchemaChangeType = "alter_type"
	ChangeTypeRenameColumn   SchemaChangeType = "rename_column"
	ChangeTypeAddTable       SchemaChangeType = "add_table"
	ChangeTypeDropTable      SchemaChangeType = "drop_table"
	ChangeTypeAlterConstraint SchemaChangeType = "alter_constraint"
)

// String returns string representation of SchemaChangeType
func (sct SchemaChangeType) String() string {
	return string(sct)
}

// SchemaChange represents a single schema change detected
type SchemaChange struct {
	Type     SchemaChangeType `json:"type"`
	Table    string            `json:"table"`
	Column   string            `json:"column,omitempty"`
	OldValue interface{}       `json:"old_value,omitempty"`
	NewValue interface{}       `json:"new_value,omitempty"`
	Impact   string            `json:"impact"` // breaking, compatible, informational
}

// SchemaDiff represents differences between two schema snapshots
type SchemaDiff struct {
	AddedTables    []string       `json:"added_tables,omitempty"`
	RemovedTables  []string       `json:"removed_tables,omitempty"`
	ModifiedTables []TableDiff    `json:"modified_tables,omitempty"`
	Changes        []SchemaChange `json:"changes"`
}

// TableDiff represents changes to a single table
type TableDiff struct {
	TableName     string          `json:"table_name"`
	AddedCols     []SchemaChange `json:"added_columns,omitempty"`
	DroppedCols    []SchemaChange `json:"dropped_columns,omitempty"`
	ModifiedCols   []SchemaChange `json:"modified_columns,omitempty"`
}

// MigrationScript represents a generated migration script
type MigrationScript struct {
	FromVersion   string   `json:"from_version"`
	ToVersion     string   `json:"to_version"`
	Dialect       string   `json:"dialect"`       // mysql, postgresql, sqlite
	Statements    []string `json:"statements"`
	EstimatedTime string   `json:"estimated_time,omitempty"`
	IsReversible  bool     `json:"is_reversible"`
}

// ValidationError represents a migration validation issue
type ValidationError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Statement string `json:"statement,omitempty"`
	Line      int    `json:"line,omitempty"`
}

// Validate checks if SchemaChangeType is valid
func (sct SchemaChangeType) Validate() error {
	validTypes := []SchemaChangeType{
		ChangeTypeAddColumn,
		ChangeTypeDropColumn,
		ChangeTypeAlterType,
		ChangeTypeRenameColumn,
		ChangeTypeAddTable,
		ChangeTypeDropTable,
		ChangeTypeAlterConstraint,
	}

	for _, vt := range validTypes {
		if sct == vt {
			return nil
		}
	}

	return fmt.Errorf("invalid schema change type: %s", sct)
}

// Validate checks if Impact is valid
func (sc SchemaChange) Validate() error {
	validImpacts := []string{"breaking", "compatible", "informational"}

	for _, vi := range validImpacts {
		if sc.Impact == vi {
			return nil
		}
	}

	return fmt.Errorf("invalid impact level: %s", sc.Impact)
}

// Validate checks if MigrationScript is valid
func (ms MigrationScript) Validate() error {
	if ms.FromVersion == "" {
		return errors.New("from_version is required")
	}

	if ms.ToVersion == "" {
		return errors.New("to_version is required")
	}

	if ms.Dialect == "" {
		return errors.New("dialect is required (mysql, postgresql, sqlite)")
	}

	validDialects := []string{"mysql", "postgresql", "sqlite"}
	found := false
	for _, vd := range validDialects {
		if ms.Dialect == vd {
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("invalid dialect: %s (must be mysql, postgresql, or sqlite)", ms.Dialect)
	}

	if len(ms.Statements) == 0 {
		return errors.New("at least one statement is required")
	}

	return nil
}

// ComputeTablesHash generates SHA-256 hash of tables for integrity
func ComputeTablesHash(tables map[string]SimpleTableInfo) (string, error) {
	data, err := json.Marshal(tables)
	if err != nil {
		return "", fmt.Errorf("failed to marshal tables: %w", err)
	}

	return computeSHA256(data), nil
}

// computeSHA256 computes SHA-256 hash of data
func computeSHA256(data []byte) string {
	h := sha256.New()
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

// ValidateSnapshot checks if a snapshot is valid
func (ss SchemaSnapshot) Validate() error {
	if ss.ID == "" {
		return errors.New("snapshot ID is required")
	}

	if ss.Timestamp.IsZero() {
		return errors.New("timestamp is required")
	}

	if ss.Profile == "" {
		return errors.New("profile is required")
	}

	if len(ss.Tables) == 0 {
		return errors.New("at least one table is required")
	}

	return nil
}
