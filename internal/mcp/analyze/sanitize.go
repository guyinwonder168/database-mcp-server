package analyze

import (
	"fmt"
	"regexp"
)

// validIdentifier matches SQL identifiers: letters, digits, underscores.
// Must start with a letter or underscore.
var validIdentifier = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// sanitizeIdentifier validates that a name is a safe SQL identifier.
// Returns an error if the name contains characters that could enable SQL injection.
// Table names, column names, and schema names should all pass through this before
// being interpolated into SQL strings.
func sanitizeIdentifier(name string) error {
	if !validIdentifier.MatchString(name) {
		return fmt.Errorf("invalid identifier %q: must match [a-zA-Z_][a-zA-Z0-9_]*", name)
	}
	return nil
}

// quoteMySQL wraps an identifier in backticks for MySQL/MariaDB.
// Caller must validate the identifier first via sanitizeIdentifier.
func quoteMySQL(name string) string {
	return "`" + name + "`"
}

// quotePostgres wraps an identifier in double quotes for PostgreSQL.
// Caller must validate the identifier first via sanitizeIdentifier.
func quotePostgres(name string) string {
	return `"` + name + `"`
}

// quoteSQLite wraps an identifier in double quotes for SQLite.
// Caller must validate the identifier first via sanitizeIdentifier.
func quoteSQLite(name string) string {
	return `"` + name + `"`
}
