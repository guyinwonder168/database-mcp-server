package mcp

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// MigrationImpact describes operational risk and estimated effort for a migration.
type MigrationImpact struct {
	TotalStatements         int    `json:"total_statements"`
	BreakingChanges         int    `json:"breaking_changes"`
	CompatibleChanges       int    `json:"compatible_changes"`
	InformationalChanges    int    `json:"informational_changes"`
	NonReversibleOperations int    `json:"non_reversible_operations"`
	RiskLevel               string `json:"risk_level"` // low, medium, high
	EstimatedDuration       string `json:"estimated_duration"`
	RequiresDowntime        bool   `json:"requires_downtime"`
}

const (
	sqlStatementAlterTable   = "ALTER TABLE"
	sqlStatementCreateTable  = "CREATE TABLE"
	sqlStatementDropTable    = "DROP TABLE"
	sqlStatementCreateIndex  = "CREATE INDEX"
	sqlStatementDropIndex    = "DROP INDEX"
	sqlStatementRenameTable  = "RENAME TABLE"
	sqlKeywordAddColumn      = "ADD COLUMN"
	sqlKeywordDropColumn     = "DROP COLUMN"
	sqlKeywordModifyColumn   = " MODIFY COLUMN "
	sqlKeywordAlterColumn    = "ALTER COLUMN"
	sqlKeywordType           = " TYPE "
	sqlKeywordSetNotNull     = " SET NOT NULL"
	sqlKeywordRenameColumn   = "RENAME COLUMN"
	errUnsupportedDialectFmt = "unsupported dialect: %s"
)

var statementPrefixes = []string{
	sqlStatementAlterTable,
	sqlStatementCreateTable,
	sqlStatementDropTable,
	sqlStatementCreateIndex,
	sqlStatementDropIndex,
	sqlStatementRenameTable,
}

var changePriorities = map[SchemaChangeType]int{
	ChangeTypeAddTable:        10,
	ChangeTypeAddColumn:       20,
	ChangeTypeRenameColumn:    30,
	ChangeTypeAlterType:       40,
	ChangeTypeAlterConstraint: 50,
	ChangeTypeDropColumn:      60,
	ChangeTypeDropTable:       70,
}

// GenerateMigration converts schema diffs into a migration script for a SQL dialect.
func GenerateMigration(diff SchemaDiff, dialect string) MigrationScript {
	normalized, err := normalizeDialect(dialect)
	if err != nil {
		script := MigrationScript{
			FromVersion:  "previous",
			ToVersion:    "current",
			Dialect:      strings.ToLower(strings.TrimSpace(dialect)),
			Statements:   []string{fmt.Sprintf("-- MANUAL ACTION REQUIRED: %v", err)},
			IsReversible: false,
		}
		script.EstimatedTime = EstimateMigrationImpact(script).EstimatedDuration

		return script
	}

	changes := make([]SchemaChange, len(diff.Changes))
	copy(changes, diff.Changes)
	sortSchemaChanges(changes)

	script := MigrationScript{
		FromVersion:  "previous",
		ToVersion:    "current",
		Dialect:      normalized,
		Statements:   make([]string, 0, max(1, len(changes))),
		IsReversible: true,
	}

	if len(changes) == 0 {
		script.Statements = append(script.Statements, "-- No schema changes detected")
		script.EstimatedTime = EstimateMigrationImpact(script).EstimatedDuration
		return script
	}

	for _, change := range changes {
		stmt, convertErr := ConvertChangeToSQL(change, normalized)
		if convertErr != nil {
			script.Statements = append(script.Statements, buildManualActionStatement(change, convertErr))
			script.IsReversible = false
			continue
		}

		script.Statements = append(script.Statements, stmt)
		if isNonReversibleChange(change) {
			script.IsReversible = false
		}
	}

	script.EstimatedTime = EstimateMigrationImpact(script).EstimatedDuration
	return script
}

// ValidateMigration validates migration metadata and SQL statements.
func ValidateMigration(script MigrationScript) []ValidationError {
	var validationErrors []ValidationError

	normalizedDialect, dialectErr := normalizeDialect(script.Dialect)
	if dialectErr == nil {
		script.Dialect = normalizedDialect
	}

	if err := script.Validate(); err != nil {
		validationErrors = append(validationErrors, ValidationError{
			Code:    "INVALID_SCRIPT",
			Message: err.Error(),
		})
	}

	if dialectErr != nil {
		normalizedDialect = strings.ToLower(strings.TrimSpace(script.Dialect))
	}

	for i, statement := range script.Statements {
		validationErrors = append(
			validationErrors,
			validateMigrationStatement(statement, i+1, normalizedDialect, script.IsReversible)...,
		)
	}

	return validationErrors
}

func validateMigrationStatement(statement string, lineNumber int, normalizedDialect string, isReversible bool) []ValidationError {
	trimmed := strings.TrimSpace(statement)
	if trimmed == "" {
		return []ValidationError{{
			Code:      "EMPTY_STATEMENT",
			Message:   "statement cannot be empty",
			Statement: statement,
			Line:      lineNumber,
		}}
	}

	if isCommentStatement(trimmed) {
		return nil
	}

	validationErrors := make([]ValidationError, 0, 3)
	if !isValidStatementPrefix(trimmed) {
		validationErrors = append(validationErrors, ValidationError{
			Code:      "INVALID_STATEMENT",
			Message:   "statement must be a DDL command",
			Statement: statement,
			Line:      lineNumber,
		})
	}

	upperStmt := strings.ToUpper(trimmed)
	if normalizedDialect == "sqlite" &&
		strings.Contains(upperStmt, sqlKeywordAlterColumn) &&
		strings.Contains(upperStmt, sqlKeywordType) {
		validationErrors = append(validationErrors, ValidationError{
			Code:      "UNSUPPORTED_OPERATION",
			Message:   "sqlite does not support direct ALTER COLUMN TYPE",
			Statement: statement,
			Line:      lineNumber,
		})
	}

	if isReversible && statementIsNonReversible(upperStmt) {
		validationErrors = append(validationErrors, ValidationError{
			Code:      "NON_REVERSIBLE",
			Message:   "script marked reversible but contains non-reversible operations",
			Statement: statement,
			Line:      lineNumber,
		})
	}

	return validationErrors
}

// EstimateMigrationImpact classifies migration risk and runtime heuristics.
func EstimateMigrationImpact(script MigrationScript) MigrationImpact {
	impact := MigrationImpact{
		TotalStatements:      len(script.Statements),
		RiskLevel:            "low",
		EstimatedDuration:    "about 1 minute",
		RequiresDowntime:     false,
		CompatibleChanges:    0,
		BreakingChanges:      0,
		InformationalChanges: 0,
	}

	minutes := 1
	for _, statement := range script.Statements {
		trimmed := strings.TrimSpace(statement)
		if trimmed == "" || isCommentStatement(trimmed) {
			continue
		}

		upperStmt := strings.ToUpper(trimmed)
		switch {
		case strings.Contains(upperStmt, sqlStatementDropTable), strings.Contains(upperStmt, sqlKeywordDropColumn):
			impact.BreakingChanges++
			impact.NonReversibleOperations++
			impact.RequiresDowntime = true
			minutes += 3
		case strings.Contains(upperStmt, sqlKeywordAlterColumn) && strings.Contains(upperStmt, sqlKeywordType),
			strings.Contains(upperStmt, sqlKeywordModifyColumn),
			strings.Contains(upperStmt, sqlKeywordSetNotNull):
			impact.BreakingChanges++
			impact.NonReversibleOperations++
			impact.RequiresDowntime = true
			minutes += 2
		case strings.Contains(upperStmt, sqlKeywordAddColumn),
			strings.Contains(upperStmt, sqlStatementCreateTable),
			strings.Contains(upperStmt, sqlStatementCreateIndex),
			strings.Contains(upperStmt, sqlStatementDropIndex),
			strings.Contains(upperStmt, sqlKeywordRenameColumn):
			impact.CompatibleChanges++
			minutes++
		default:
			impact.InformationalChanges++
		}
	}

	impact.RiskLevel = classifyRiskLevel(impact)
	impact.EstimatedDuration = fmt.Sprintf("about %d minute(s)", minutes)

	return impact
}

// ConvertChangeToSQL converts one schema change to dialect-specific SQL.
func ConvertChangeToSQL(change SchemaChange, dialect string) (string, error) {
	if err := change.Type.Validate(); err != nil {
		return "", err
	}
	if change.Table == "" {
		return "", fmt.Errorf("table is required")
	}

	normalized, err := normalizeDialect(dialect)
	if err != nil {
		return "", err
	}

	tableName := quoteIdentifier(change.Table, normalized)
	columnName := quoteIdentifier(change.Column, normalized)

	return convertChangeToDialectSQL(change, normalized, tableName, columnName)
}

func convertChangeToDialectSQL(change SchemaChange, dialect, tableName, columnName string) (string, error) {
	switch change.Type {
	case ChangeTypeAddTable:
		return buildAddTableSQL(dialect, tableName), nil

	case ChangeTypeDropTable:
		return fmt.Sprintf("%s %s;", sqlStatementDropTable, tableName), nil

	case ChangeTypeAddColumn:
		return buildAddColumnSQL(change, tableName, columnName)

	case ChangeTypeDropColumn:
		return buildDropColumnSQL(change, tableName, columnName)

	case ChangeTypeAlterType:
		return buildAlterTypeSQL(change, dialect, tableName, columnName)

	case ChangeTypeRenameColumn:
		return buildRenameColumnSQL(change, dialect, tableName)

	case ChangeTypeAlterConstraint:
		return buildAlterConstraintSQL(change, dialect, tableName, columnName)

	default:
		return "", fmt.Errorf("unsupported schema change type: %s", change.Type)
	}
}

func buildAddTableSQL(dialect, tableName string) string {
	if dialect == "sqlite" {
		return fmt.Sprintf("%s %s (id INTEGER PRIMARY KEY);", sqlStatementCreateTable, tableName)
	}
	return fmt.Sprintf("%s %s (id BIGINT PRIMARY KEY);", sqlStatementCreateTable, tableName)
}

func buildAddColumnSQL(change SchemaChange, tableName, columnName string) (string, error) {
	if change.Column == "" {
		return "", fmt.Errorf("column is required for add_column")
	}
	newType := extractSQLType(change.NewValue, "TEXT")
	return fmt.Sprintf("%s %s %s %s %s;", sqlStatementAlterTable, tableName, sqlKeywordAddColumn, columnName, newType), nil
}

func buildDropColumnSQL(change SchemaChange, tableName, columnName string) (string, error) {
	if change.Column == "" {
		return "", fmt.Errorf("column is required for drop_column")
	}
	return fmt.Sprintf("%s %s %s %s;", sqlStatementAlterTable, tableName, sqlKeywordDropColumn, columnName), nil
}

func buildAlterTypeSQL(change SchemaChange, dialect, tableName, columnName string) (string, error) {
	if change.Column == "" {
		return "", fmt.Errorf("column is required for alter_type")
	}
	newType := extractSQLType(change.NewValue, "")
	if newType == "" {
		return "", fmt.Errorf("new SQL type is required for alter_type")
	}

	switch dialect {
	case "mysql":
		return fmt.Sprintf("%s %s%s%s %s;", sqlStatementAlterTable, tableName, sqlKeywordModifyColumn, columnName, newType), nil
	case "postgresql":
		return fmt.Sprintf("%s %s %s %s%s;", sqlStatementAlterTable, tableName, sqlKeywordAlterColumn, columnName, sqlKeywordType+newType), nil
	case "sqlite":
		return "", fmt.Errorf("sqlite does not support direct ALTER COLUMN TYPE")
	default:
		return "", fmt.Errorf(errUnsupportedDialectFmt, dialect)
	}
}

func buildRenameColumnSQL(change SchemaChange, dialect, tableName string) (string, error) {
	oldName, newName, err := extractRenameColumns(change)
	if err != nil {
		return "", err
	}
	oldIdentifier := quoteIdentifier(oldName, dialect)
	newIdentifier := quoteIdentifier(newName, dialect)
	return fmt.Sprintf("%s %s %s %s TO %s;", sqlStatementAlterTable, tableName, sqlKeywordRenameColumn, oldIdentifier, newIdentifier), nil
}

func normalizeDialect(dialect string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(dialect)) {
	case "mysql", "mariadb":
		return "mysql", nil
	case "postgres", "postgresql", "pg":
		return "postgresql", nil
	case "sqlite", "sqlite3":
		return "sqlite", nil
	default:
		return "", fmt.Errorf(errUnsupportedDialectFmt, dialect)
	}
}

func sortSchemaChanges(changes []SchemaChange) {
	sort.SliceStable(changes, func(i, j int) bool {
		priorityI := changePriority(changes[i].Type)
		priorityJ := changePriority(changes[j].Type)
		if priorityI != priorityJ {
			return priorityI < priorityJ
		}
		if changes[i].Table != changes[j].Table {
			return changes[i].Table < changes[j].Table
		}
		if changes[i].Column != changes[j].Column {
			return changes[i].Column < changes[j].Column
		}
		return changes[i].Type < changes[j].Type
	})
}

func changePriority(changeType SchemaChangeType) int {
	priority, ok := changePriorities[changeType]
	if !ok {
		return 999
	}

	return priority
}

// validSQLIdentifier matches safe SQL identifiers: letters, digits, underscores,
// dots (for schema.table), starting with a letter or underscore.
var validSQLIdentifier = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_.]*$`)

// sanitizeSQLIdentifier validates that a name is a safe SQL identifier.
// Returns an error if the name contains characters that could enable SQL injection.
func sanitizeSQLIdentifier(name string) error {
	if !validSQLIdentifier.MatchString(name) {
		return fmt.Errorf("invalid SQL identifier %q: must match [a-zA-Z_][a-zA-Z0-9_.]*", name)
	}
	return nil
}

func quoteIdentifier(identifier, dialect string) string {
	if dialect == "mysql" {
		return fmt.Sprintf("`%s`", identifier)
	}

	return fmt.Sprintf(`"%s"`, identifier)
}

// safeQuoteIdentifier validates an identifier and returns it properly quoted
// for the given SQL dialect. Returns an error if the identifier contains
// characters that could enable SQL injection.
func safeQuoteIdentifier(identifier, dialect string) (string, error) {
	if err := sanitizeSQLIdentifier(identifier); err != nil {
		return "", err
	}
	return quoteIdentifier(identifier, dialect), nil
}

func extractSQLType(value interface{}, fallback string) string {
	typeValue, ok := value.(string)
	if !ok {
		return fallback
	}
	typeValue = strings.TrimSpace(typeValue)
	if typeValue == "" {
		return fallback
	}
	return typeValue
}

func extractRenameColumns(change SchemaChange) (string, string, error) {
	oldName := change.Column
	if oldName == "" {
		candidate, ok := change.OldValue.(string)
		if ok {
			oldName = strings.TrimSpace(candidate)
		}
	}

	newName, ok := change.NewValue.(string)
	if ok {
		newName = strings.TrimSpace(newName)
	}

	if oldName == "" || newName == "" {
		return "", "", fmt.Errorf("rename_column requires old and new column names")
	}

	return oldName, newName, nil
}

func buildAlterConstraintSQL(change SchemaChange, dialect, tableName, columnName string) (string, error) {
	if change.Column == "" {
		return "", fmt.Errorf("column is required for alter_constraint")
	}

	nullable, ok := change.NewValue.(bool)
	if !ok {
		return "", fmt.Errorf("new_value must be a boolean for alter_constraint")
	}

	switch dialect {
	case "postgresql":
		if nullable {
			return fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s DROP NOT NULL;", tableName, columnName), nil
		}
		return fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s SET NOT NULL;", tableName, columnName), nil
	case "mysql":
		return "", fmt.Errorf("mysql alter_constraint requires full column type metadata for MODIFY COLUMN")
	case "sqlite":
		return "", fmt.Errorf("sqlite requires table rebuild for constraint changes")
	default:
		return "", fmt.Errorf(errUnsupportedDialectFmt, dialect)
	}
}

func buildManualActionStatement(change SchemaChange, err error) string {
	location := change.Table
	if change.Column != "" {
		location = fmt.Sprintf("%s.%s", change.Table, change.Column)
	}
	return fmt.Sprintf("-- MANUAL ACTION REQUIRED (%s): %v", location, err)
}

func isNonReversibleChange(change SchemaChange) bool {
	return change.Type == ChangeTypeDropTable ||
		change.Type == ChangeTypeDropColumn ||
		change.Type == ChangeTypeAlterType
}

func statementIsNonReversible(statement string) bool {
	return strings.Contains(statement, sqlStatementDropTable) ||
		strings.Contains(statement, sqlKeywordDropColumn) ||
		strings.Contains(statement, sqlKeywordModifyColumn) ||
		(strings.Contains(statement, sqlKeywordAlterColumn) && strings.Contains(statement, sqlKeywordType))
}

func isCommentStatement(statement string) bool {
	return strings.HasPrefix(statement, "--") || strings.HasPrefix(statement, "/*")
}

func isValidStatementPrefix(statement string) bool {
	upperStmt := strings.ToUpper(strings.TrimSpace(statement))
	for _, prefix := range statementPrefixes {
		if strings.HasPrefix(upperStmt, prefix) {
			return true
		}
	}

	return false
}

func classifyRiskLevel(impact MigrationImpact) string {
	if impact.BreakingChanges > 0 || impact.NonReversibleOperations > 0 {
		return "high"
	}
	if impact.TotalStatements >= 6 {
		return "medium"
	}
	return "low"
}
