package mcp

import (
	"fmt"
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

var statementPrefixes = []string{
	"ALTER TABLE",
	"CREATE TABLE",
	"DROP TABLE",
	"CREATE INDEX",
	"DROP INDEX",
	"RENAME TABLE",
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
		lineNumber := i + 1
		trimmed := strings.TrimSpace(statement)

		if trimmed == "" {
			validationErrors = append(validationErrors, ValidationError{
				Code:      "EMPTY_STATEMENT",
				Message:   "statement cannot be empty",
				Statement: statement,
				Line:      lineNumber,
			})
			continue
		}

		if isCommentStatement(trimmed) {
			continue
		}

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
			strings.Contains(upperStmt, "ALTER COLUMN") &&
			strings.Contains(upperStmt, " TYPE ") {
			validationErrors = append(validationErrors, ValidationError{
				Code:      "UNSUPPORTED_OPERATION",
				Message:   "sqlite does not support direct ALTER COLUMN TYPE",
				Statement: statement,
				Line:      lineNumber,
			})
		}

		if script.IsReversible && statementIsNonReversible(upperStmt) {
			validationErrors = append(validationErrors, ValidationError{
				Code:      "NON_REVERSIBLE",
				Message:   "script marked reversible but contains non-reversible operations",
				Statement: statement,
				Line:      lineNumber,
			})
		}
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
		case strings.Contains(upperStmt, "DROP TABLE"), strings.Contains(upperStmt, "DROP COLUMN"):
			impact.BreakingChanges++
			impact.NonReversibleOperations++
			impact.RequiresDowntime = true
			minutes += 3
		case strings.Contains(upperStmt, "ALTER COLUMN") && strings.Contains(upperStmt, " TYPE "),
			strings.Contains(upperStmt, " MODIFY COLUMN "),
			strings.Contains(upperStmt, " SET NOT NULL"):
			impact.BreakingChanges++
			impact.NonReversibleOperations++
			impact.RequiresDowntime = true
			minutes += 2
		case strings.Contains(upperStmt, "ADD COLUMN"),
			strings.Contains(upperStmt, "CREATE TABLE"),
			strings.Contains(upperStmt, "CREATE INDEX"),
			strings.Contains(upperStmt, "DROP INDEX"),
			strings.Contains(upperStmt, "RENAME COLUMN"):
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

	switch change.Type {
	case ChangeTypeAddTable:
		if normalized == "sqlite" {
			return fmt.Sprintf("CREATE TABLE %s (id INTEGER PRIMARY KEY);", tableName), nil
		}
		return fmt.Sprintf("CREATE TABLE %s (id BIGINT PRIMARY KEY);", tableName), nil

	case ChangeTypeDropTable:
		return fmt.Sprintf("DROP TABLE %s;", tableName), nil

	case ChangeTypeAddColumn:
		if change.Column == "" {
			return "", fmt.Errorf("column is required for add_column")
		}
		newType := extractSQLType(change.NewValue, "TEXT")
		return fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s;", tableName, columnName, newType), nil

	case ChangeTypeDropColumn:
		if change.Column == "" {
			return "", fmt.Errorf("column is required for drop_column")
		}
		return fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s;", tableName, columnName), nil

	case ChangeTypeAlterType:
		if change.Column == "" {
			return "", fmt.Errorf("column is required for alter_type")
		}
		newType := extractSQLType(change.NewValue, "")
		if newType == "" {
			return "", fmt.Errorf("new SQL type is required for alter_type")
		}

		switch normalized {
		case "mysql":
			return fmt.Sprintf("ALTER TABLE %s MODIFY COLUMN %s %s;", tableName, columnName, newType), nil
		case "postgresql":
			return fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s TYPE %s;", tableName, columnName, newType), nil
		case "sqlite":
			return "", fmt.Errorf("sqlite does not support direct ALTER COLUMN TYPE")
		default:
			return "", fmt.Errorf("unsupported dialect: %s", normalized)
		}

	case ChangeTypeRenameColumn:
		oldName, newName, err := extractRenameColumns(change)
		if err != nil {
			return "", err
		}
		oldIdentifier := quoteIdentifier(oldName, normalized)
		newIdentifier := quoteIdentifier(newName, normalized)
		return fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s;", tableName, oldIdentifier, newIdentifier), nil

	case ChangeTypeAlterConstraint:
		return buildAlterConstraintSQL(change, normalized, tableName, columnName)

	default:
		return "", fmt.Errorf("unsupported schema change type: %s", change.Type)
	}
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
		return "", fmt.Errorf("unsupported dialect: %s", dialect)
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

func quoteIdentifier(identifier, dialect string) string {
	if dialect == "mysql" {
		return fmt.Sprintf("`%s`", identifier)
	}

	return fmt.Sprintf(`"%s"`, identifier)
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
		return "", fmt.Errorf("unsupported dialect: %s", dialect)
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
	return strings.Contains(statement, "DROP TABLE") ||
		strings.Contains(statement, "DROP COLUMN") ||
		strings.Contains(statement, " MODIFY COLUMN ") ||
		(strings.Contains(statement, "ALTER COLUMN") && strings.Contains(statement, " TYPE "))
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
