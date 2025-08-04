// errors.go
// Enhanced error handling with structured feedback and suggestions for the MCP server.

package mcp

import (
	"database-mcp-provider/internal/log"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// ErrorCode represents standardized error codes for the MCP server
type ErrorCode string

const (
	// Database connection errors
	ErrorCodeConnectionFailed ErrorCode = "CONNECTION_FAILED"
	ErrorCodeProfileNotFound  ErrorCode = "PROFILE_NOT_FOUND"
	ErrorCodeUnsupportedDB    ErrorCode = "UNSUPPORTED_DATABASE"

	// SQL execution errors
	ErrorCodeSQLSyntax           ErrorCode = "SQL_SYNTAX_ERROR"
	ErrorCodeTableNotFound       ErrorCode = "TABLE_NOT_FOUND"
	ErrorCodeColumnNotFound      ErrorCode = "COLUMN_NOT_FOUND"
	ErrorCodePermissionDenied    ErrorCode = "PERMISSION_DENIED"
	ErrorCodeReadOnlyViolation   ErrorCode = "READONLY_VIOLATION"
	ErrorCodeConstraintViolation ErrorCode = "CONSTRAINT_VIOLATION"
	ErrorCodeDataTypeMismatch    ErrorCode = "DATA_TYPE_MISMATCH"

	// Schema introspection errors
	ErrorCodeDatabaseNotFound ErrorCode = "DATABASE_NOT_FOUND"
	ErrorCodeNoTablesFound    ErrorCode = "NO_TABLES_FOUND"

	// Input validation errors
	ErrorCodeInvalidInput     ErrorCode = "INVALID_INPUT"
	ErrorCodeMissingParameter ErrorCode = "MISSING_PARAMETER"

	// Generic errors
	ErrorCodeInternalError     ErrorCode = "INTERNAL_ERROR"
	ErrorCodeUnknownError      ErrorCode = "UNKNOWN_ERROR"
	ErrorCodeConfigNotFound    ErrorCode = "CONFIG_NOT_FOUND"
	ErrorCodeSQLExecutionError ErrorCode = "SQL_EXECUTION_ERROR"
)

// ErrorSuggestion represents a suggestion to fix an error
type ErrorSuggestion struct {
	Tool        string `json:"tool,omitempty"`
	Action      string `json:"action,omitempty"`
	Description string `json:"description"`
	Example     string `json:"example,omitempty"`
}

// StructuredError represents an enhanced error response with suggestions
type StructuredError struct {
	Status      string                 `json:"status"`
	ErrorCode   ErrorCode              `json:"error_code"`
	Message     string                 `json:"message"`
	Details     string                 `json:"details,omitempty"`
	Suggestions []ErrorSuggestion      `json:"suggestions,omitempty"`
	Context     map[string]interface{} `json:"context,omitempty"`
}

// Error implements the error interface
func (e *StructuredError) Error() string {
	return e.Message
}

// ToJSON converts the structured error to JSON string
func (e *StructuredError) ToJSON() string {
	b, _ := json.Marshal(e)
	return string(b)
}

// NewStructuredError creates a new structured error
func NewStructuredError(code ErrorCode, message string, details string) *StructuredError {
	return &StructuredError{
		Status:    "error",
		ErrorCode: code,
		Message:   message,
		Details:   details,
		Context:   make(map[string]interface{}),
	}
}

// WithSuggestions adds suggestions to the error
func (e *StructuredError) WithSuggestions(suggestions ...ErrorSuggestion) *StructuredError {
	e.Suggestions = append(e.Suggestions, suggestions...)
	return e
}

// WithContext adds context information to the error
func (e *StructuredError) WithContext(key string, value interface{}) *StructuredError {
	e.Context[key] = value
	return e
}

// WithContextMap adds multiple context values to the error
func (e *StructuredError) WithContextMap(contextMap map[string]interface{}) *StructuredError {
	for k, v := range contextMap {
		e.Context[k] = v
	}
	return e
}

// ErrorAnalyzer analyzes errors and provides suggestions
type ErrorAnalyzer struct {
	configPath string
}

// NewErrorAnalyzer creates a new error analyzer
func NewErrorAnalyzer(configPath string) *ErrorAnalyzer {
	return &ErrorAnalyzer{configPath: configPath}
}

// AnalyzeError analyzes an error and returns a structured error with suggestions
func (a *ErrorAnalyzer) AnalyzeError(err error, context map[string]interface{}) *StructuredError {
	if err == nil {
		return nil
	}

	errMsg := err.Error()
	log.JSONLog("debug", "Analyzing error", map[string]interface{}{
		"error":   errMsg,
		"context": context,
	})

	// Check for specific error patterns
	switch {
	case strings.Contains(errMsg, "profile not found"):
		return a.handleProfileNotFound(context)

	case strings.Contains(errMsg, "no such table") || strings.Contains(errMsg, "table") && strings.Contains(errMsg, "does not exist"):
		return a.handleTableNotFound(context)

	case strings.Contains(errMsg, "no such column") || strings.Contains(errMsg, "column") && strings.Contains(errMsg, "does not exist"):
		return a.handleColumnNotFound(context)

	case strings.Contains(errMsg, "syntax error") || strings.Contains(errMsg, "SQL syntax"):
		return a.handleSQLSyntaxError(context)

	case strings.Contains(errMsg, "denied") || strings.Contains(errMsg, "permission"):
		return a.handlePermissionDenied(context)

	case strings.Contains(errMsg, "read-only") || strings.Contains(errMsg, "readonly"):
		return a.handleReadOnlyViolation(context)

	case strings.Contains(errMsg, "constraint") || strings.Contains(errMsg, "foreign key"):
		return a.handleConstraintViolation(context)

	case strings.Contains(errMsg, "connection") || strings.Contains(errMsg, "connect"):
		return a.handleConnectionError(context)

	case strings.Contains(errMsg, "database") && (strings.Contains(errMsg, "does not exist") || strings.Contains(errMsg, "not found")):
		return a.handleDatabaseNotFound(context)

	case strings.Contains(errMsg, "data type") || strings.Contains(errMsg, "type mismatch"):
		return a.handleDataTypeMismatch(context)

	case strings.Contains(errMsg, "unsupported db_type"):
		return a.handleUnsupportedDatabase(context)

	default:
		return a.handleUnknownError(context)
	}
}

// handleProfileNotFound handles profile not found errors
func (a *ErrorAnalyzer) handleProfileNotFound(context map[string]interface{}) *StructuredError {
	profileName := ""
	if name, ok := context["profile_name"].(string); ok {
		profileName = name
	}

	err := NewStructuredError(
		ErrorCodeProfileNotFound,
		fmt.Sprintf("Database profile '%s' not found", profileName),
		"The specified database profile does not exist in the configuration",
	).WithContext("profile_name", profileName)

	err.WithSuggestions(
		ErrorSuggestion{
			Action:      "List available profiles",
			Description: "Use the list-profiles tool to see all configured profiles",
			Example:     `{"tool": "list-profiles"}`,
		},
		ErrorSuggestion{
			Action:      "Create a new profile",
			Description: "Use the configure-profile tool to create this profile",
			Example:     fmt.Sprintf(`{"tool": "configure-profile", "profile_name": "%s", "db_type": "mysql", "host": "localhost", "port": 3306, "username": "user", "password": "pass", "database_name": "mydb"}`, profileName),
		},
	)

	return err
}

// handleTableNotFound handles table not found errors
func (a *ErrorAnalyzer) handleTableNotFound(context map[string]interface{}) *StructuredError {
	// Extract table name from error message
	tableName := extractTableName(fmt.Sprint(context["error"]))
	if tn, ok := context["table_name"].(string); ok && tableName == "" {
		tableName = tn
	}

	err := NewStructuredError(
		ErrorCodeTableNotFound,
		fmt.Sprintf("Table '%s' not found", tableName),
		"The specified table does not exist in the database",
	).WithContext("table_name", tableName)

	profileName := ""
	if pn, ok := context["profile_name"].(string); ok {
		profileName = pn
		err.WithContext("profile_name", profileName)
	}

	suggestions := []ErrorSuggestion{
		{
			Action:      "List available tables",
			Description: "Use the list-tables tool to see all tables in the database",
			Example:     fmt.Sprintf(`{"tool": "list-tables", "profile_name": "%s"}`, profileName),
		},
		{
			Action:      "Check table name spelling",
			Description: "Verify the table name is spelled correctly and matches the case sensitivity requirements of your database",
		},
	}

	if dbName, ok := context["database_name"].(string); ok {
		suggestions = append(suggestions, ErrorSuggestion{
			Action:      "Check database context",
			Description: fmt.Sprintf("Ensure you're connected to the correct database (%s)", dbName),
			Example:     fmt.Sprintf(`{"tool": "list-tables", "profile_name": "%s", "database_name": "%s"}`, profileName, dbName),
		})
		err.WithContext("database_name", dbName)
	}

	err.WithSuggestions(suggestions...)
	return err
}

// handleColumnNotFound handles column not found errors
func (a *ErrorAnalyzer) handleColumnNotFound(context map[string]interface{}) *StructuredError {
	// Extract column and table names from error message
	columnName := extractColumnName(fmt.Sprint(context["error"]))
	tableName := extractTableName(fmt.Sprint(context["error"]))

	if cn, ok := context["column_name"].(string); ok && columnName == "" {
		columnName = cn
	}
	if tn, ok := context["table_name"].(string); ok && tableName == "" {
		tableName = tn
	}

	err := NewStructuredError(
		ErrorCodeColumnNotFound,
		fmt.Sprintf("Column '%s' not found in table '%s'", columnName, tableName),
		"The specified column does not exist in the table",
	).WithContext("column_name", columnName).WithContext("table_name", tableName)

	profileName := ""
	if pn, ok := context["profile_name"].(string); ok {
		profileName = pn
		err.WithContext("profile_name", profileName)
	}

	suggestions := []ErrorSuggestion{
		{
			Action:      "Describe table schema",
			Description: "Use the describe-table tool to see all columns in the table",
			Example:     fmt.Sprintf(`{"tool": "describe-table", "profile_name": "%s", "table_name": "%s"}`, profileName, tableName),
		},
		{
			Action:      "Check column name spelling",
			Description: "Verify the column name is spelled correctly and matches the case sensitivity requirements",
		},
	}

	// Add sample data suggestion
	if tableName != "" {
		suggestions = append(suggestions, ErrorSuggestion{
			Action:      "View sample data",
			Description: "Use the sample-data tool to see actual column names and data",
			Example:     fmt.Sprintf(`{"tool": "sample-data", "profile_name": "%s", "table_name": "%s", "sample_size": 3}`, profileName, tableName),
		})
	}

	err.WithSuggestions(suggestions...)
	return err
}

// handleSQLSyntaxError handles SQL syntax errors
func (a *ErrorAnalyzer) handleSQLSyntaxError(context map[string]interface{}) *StructuredError {
	err := NewStructuredError(
		ErrorCodeSQLSyntax,
		"SQL syntax error",
		fmt.Sprint(context["error"]),
	)

	sql := ""
	if s, ok := context["sql"].(string); ok {
		sql = s
		err.WithContext("sql", sql)
	}

	dbType := ""
	if dt, ok := context["db_type"].(string); ok {
		dbType = dt
		err.WithContext("db_type", dbType)
	}

	suggestions := []ErrorSuggestion{
		{
			Action:      "Check SQL syntax",
			Description: fmt.Sprintf("Verify your SQL syntax is valid for %s", dbType),
		},
	}

	// Analyze common syntax issues
	if sql != "" {
		sqlLower := strings.ToLower(sql)

		if strings.Contains(sqlLower, "limit") && dbType == "mssql" {
			suggestions = append(suggestions, ErrorSuggestion{
				Action:      "Use TOP instead of LIMIT",
				Description: "SQL Server uses TOP instead of LIMIT",
				Example:     "SELECT TOP 10 * FROM table_name",
			})
		}

		if !strings.HasSuffix(strings.TrimSpace(sql), ";") {
			suggestions = append(suggestions, ErrorSuggestion{
				Action:      "Add semicolon",
				Description: "Some databases require a semicolon at the end of SQL statements",
			})
		}

		// Check for common quote issues
		if strings.Count(sql, "'")%2 != 0 {
			suggestions = append(suggestions, ErrorSuggestion{
				Action:      "Check string quotes",
				Description: "Ensure all string literals have matching quotes",
			})
		}
	}

	err.WithSuggestions(suggestions...)
	return err
}

// handleReadOnlyViolation handles read-only profile violations
func (a *ErrorAnalyzer) handleReadOnlyViolation(context map[string]interface{}) *StructuredError {
	err := NewStructuredError(
		ErrorCodeReadOnlyViolation,
		"Read-only profile violation",
		"Write or unsafe operations are not allowed on read-only profiles",
	)

	if sql, ok := context["sql"].(string); ok {
		err.WithContext("attempted_sql", sql)
	}

	if profileName, ok := context["profile_name"].(string); ok {
		err.WithContext("profile_name", profileName)
	}

	err.WithSuggestions(
		ErrorSuggestion{
			Action:      "Use a different profile",
			Description: "Switch to a profile that allows write operations",
		},
		ErrorSuggestion{
			Action:      "Update profile settings",
			Description: "Use configure-profile to update the profile and set readonly to false",
		},
		ErrorSuggestion{
			Action:      "Use read-only queries",
			Description: "Limit your queries to SELECT, SHOW, DESCRIBE, EXPLAIN, or PRAGMA statements",
		},
	)

	return err
}

// Helper functions to extract information from error messages
func extractTableName(errMsg string) string {
	// Try to extract table name from common error patterns
	patterns := []string{
		`table ['"]?(\w+)['"]?`,
		`relation ['"]?(\w+)['"]?`,
		`no such table: (\w+)`,
		`Table '(\w+)'`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(errMsg)
		if len(matches) > 1 {
			return matches[1]
		}
	}
	return ""
}

func extractColumnName(errMsg string) string {
	// Try to extract column name from common error patterns
	patterns := []string{
		`column ['"]?(\w+)['"]?`,
		`field ['"]?(\w+)['"]?`,
		`no such column: (\w+)`,
		`Column '(\w+)'`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(errMsg)
		if len(matches) > 1 {
			return matches[1]
		}
	}
	return ""
}

// Additional error handlers...

func (a *ErrorAnalyzer) handlePermissionDenied(context map[string]interface{}) *StructuredError {
	err := NewStructuredError(
		ErrorCodePermissionDenied,
		"Permission denied",
		fmt.Sprint(context["error"]),
	)

	err.WithSuggestions(
		ErrorSuggestion{
			Action:      "Check database permissions",
			Description: "Verify the database user has the necessary permissions for this operation",
		},
		ErrorSuggestion{
			Action:      "Contact database administrator",
			Description: "Request the necessary permissions from your database administrator",
		},
	)

	return err
}

func (a *ErrorAnalyzer) handleConstraintViolation(context map[string]interface{}) *StructuredError {
	err := NewStructuredError(
		ErrorCodeConstraintViolation,
		"Database constraint violation",
		fmt.Sprint(context["error"]),
	)

	err.WithSuggestions(
		ErrorSuggestion{
			Action:      "Check foreign key relationships",
			Description: "Use the discover-joins tool to understand table relationships",
		},
		ErrorSuggestion{
			Action:      "Verify data integrity",
			Description: "Ensure referenced records exist and data meets all constraints",
		},
	)

	return err
}

func (a *ErrorAnalyzer) handleConnectionError(context map[string]interface{}) *StructuredError {
	err := NewStructuredError(
		ErrorCodeConnectionFailed,
		"Database connection failed",
		fmt.Sprint(context["error"]),
	)

	err.WithSuggestions(
		ErrorSuggestion{
			Action:      "Check database server",
			Description: "Verify the database server is running and accessible",
		},
		ErrorSuggestion{
			Action:      "Verify connection details",
			Description: "Check host, port, username, and password in the profile configuration",
		},
		ErrorSuggestion{
			Action:      "Check network connectivity",
			Description: "Ensure there are no firewall or network issues blocking the connection",
		},
	)

	return err
}

func (a *ErrorAnalyzer) handleDatabaseNotFound(context map[string]interface{}) *StructuredError {
	dbName := ""
	if dn, ok := context["database_name"].(string); ok {
		dbName = dn
	}

	err := NewStructuredError(
		ErrorCodeDatabaseNotFound,
		fmt.Sprintf("Database '%s' not found", dbName),
		"The specified database does not exist",
	).WithContext("database_name", dbName)

	profileName := ""
	if pn, ok := context["profile_name"].(string); ok {
		profileName = pn
	}

	err.WithSuggestions(
		ErrorSuggestion{
			Action:      "List available databases",
			Description: "Use the list-databases tool to see all databases",
			Example:     fmt.Sprintf(`{"tool": "list-databases", "profile_name": "%s"}`, profileName),
		},
		ErrorSuggestion{
			Action:      "Create the database",
			Description: "Create the database using your database management tool",
		},
	)

	return err
}

func (a *ErrorAnalyzer) handleDataTypeMismatch(context map[string]interface{}) *StructuredError {
	err := NewStructuredError(
		ErrorCodeDataTypeMismatch,
		"Data type mismatch",
		fmt.Sprint(context["error"]),
	)

	tableName := ""
	if tn, ok := context["table_name"].(string); ok {
		tableName = tn
	}

	suggestions := []ErrorSuggestion{
		{
			Action:      "Check column data types",
			Description: "Verify the data types match the column definitions",
		},
	}

	if tableName != "" {
		profileName := ""
		if pn, ok := context["profile_name"].(string); ok {
			profileName = pn
		}

		suggestions = append(suggestions, ErrorSuggestion{
			Action:      "Review table schema",
			Description: "Use describe-table to see column data types",
			Example:     fmt.Sprintf(`{"tool": "describe-table", "profile_name": "%s", "table_name": "%s"}`, profileName, tableName),
		})
	}

	err.WithSuggestions(suggestions...)
	return err
}

func (a *ErrorAnalyzer) handleUnsupportedDatabase(context map[string]interface{}) *StructuredError {
	dbType := ""
	if dt, ok := context["db_type"].(string); ok {
		dbType = dt
	}

	err := NewStructuredError(
		ErrorCodeUnsupportedDB,
		fmt.Sprintf("Unsupported database type: %s", dbType),
		"This database type is not supported by the MCP server",
	).WithContext("db_type", dbType)

	err.WithSuggestions(
		ErrorSuggestion{
			Action:      "Use a supported database",
			Description: "Supported databases: mysql, mariadb, postgres, sqlite",
		},
		ErrorSuggestion{
			Action:      "Check database type spelling",
			Description: "Ensure the db_type is spelled correctly (lowercase)",
		},
	)

	return err
}

func (a *ErrorAnalyzer) handleUnknownError(context map[string]interface{}) *StructuredError {
	return NewStructuredError(
		ErrorCodeUnknownError,
		"An unexpected error occurred",
		fmt.Sprint(context["error"]),
	).WithSuggestions(
		ErrorSuggestion{
			Action:      "Check error details",
			Description: "Review the error message for more information",
		},
		ErrorSuggestion{
			Action:      "Verify input parameters",
			Description: "Ensure all required parameters are provided correctly",
		},
	)
}
