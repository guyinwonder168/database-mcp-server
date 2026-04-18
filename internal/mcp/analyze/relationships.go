package analyze

import (
	"context"
	"database/sql"
	"fmt"
)

// relationships.go handles foreign key discovery and implicit relationship detection.
// BUG-006 fix: queries information_schema.KEY_COLUMN_USAGE for real FKs,
// reduces false positives from implicit column-name matching.

// DiscoverForeignKeys retrieves real foreign key relationships from the database.
// MySQL/MariaDB: uses information_schema.KEY_COLUMN_USAGE with REFERENCED_TABLE_NAME.
// PostgreSQL: uses information_schema.table_constraints + key_column_usage + constraint_column_usage.
// SQLite: uses PRAGMA foreign_key_list per table.
func DiscoverForeignKeys(ctx context.Context, db *sql.DB, dbType, schema string, sqliteTables ...[]string) ([]ForeignKeyRelationship, error) {
	switch dbType {
	case "mysql", "mariadb":
		return discoverFKsMySQL(ctx, db, schema)
	case "postgres", "postgresql":
		return discoverFKsPostgres(ctx, db, schema)
	case "sqlite":
		tables := optionalStrings(sqliteTables)
		return discoverFKsSQLite(ctx, db, tables)
	default:
		return nil, fmt.Errorf("unsupported db type for FK discovery: %s", dbType)
	}
}

func optionalStrings(slices [][]string) []string {
	if len(slices) > 0 {
		return slices[0]
	}
	return nil
}

func discoverFKsMySQL(ctx context.Context, db *sql.DB, schema string) ([]ForeignKeyRelationship, error) {
	query := `SELECT TABLE_NAME, COLUMN_NAME, REFERENCED_TABLE_NAME, REFERENCED_COLUMN_NAME
		FROM information_schema.KEY_COLUMN_USAGE
		WHERE TABLE_SCHEMA = ? AND REFERENCED_TABLE_NAME IS NOT NULL
		ORDER BY TABLE_NAME, COLUMN_NAME`

	rows, err := db.QueryContext(ctx, query, schema)
	if err != nil {
		return nil, fmt.Errorf("mysql FK discovery: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var fks []ForeignKeyRelationship
	for rows.Next() {
		var fromTable, fromColumn, toTable, toColumn string
		if err := rows.Scan(&fromTable, &fromColumn, &toTable, &toColumn); err != nil {
			continue
		}
		fks = append(fks, ForeignKeyRelationship{
			FromTable:  fromTable,
			FromColumn: fromColumn,
			ToTable:    toTable,
			ToColumn:   toColumn,
		})
	}
	return fks, rows.Err()
}

func discoverFKsPostgres(ctx context.Context, db *sql.DB, schema string) ([]ForeignKeyRelationship, error) {
	query := `SELECT tc.table_name, kcu.column_name,
		ccu.table_name AS referenced_table_name, ccu.column_name AS referenced_column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
		  ON tc.constraint_name = kcu.constraint_name
		JOIN information_schema.constraint_column_usage ccu
		  ON tc.constraint_name = ccu.constraint_name
		WHERE tc.constraint_type = 'FOREIGN KEY' AND tc.table_schema = $1
		ORDER BY tc.table_name, kcu.column_name`

	rows, err := db.QueryContext(ctx, query, schema)
	if err != nil {
		return nil, fmt.Errorf("postgres FK discovery: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var fks []ForeignKeyRelationship
	for rows.Next() {
		var fromTable, fromColumn, toTable, toColumn string
		if err := rows.Scan(&fromTable, &fromColumn, &toTable, &toColumn); err != nil {
			continue
		}
		fks = append(fks, ForeignKeyRelationship{
			FromTable:  fromTable,
			FromColumn: fromColumn,
			ToTable:    toTable,
			ToColumn:   toColumn,
		})
	}
	return fks, rows.Err()
}

func discoverFKsSQLite(ctx context.Context, db *sql.DB, tableNames []string) ([]ForeignKeyRelationship, error) {
	var fks []ForeignKeyRelationship

	for _, table := range tableNames {
		rows, err := db.QueryContext(ctx,
			`SELECT id, seq, "table", "from", "to", on_update, on_delete, "match" FROM pragma_foreign_key_list WHERE arg = ?`,
			table)
		if err != nil {
			continue
		}

		for rows.Next() {
			var id, seq int
			var refTable, fromCol, toCol, onUpdate, onDelete, match string
			if err := rows.Scan(&id, &seq, &refTable, &fromCol, &toCol, &onUpdate, &onDelete, &match); err != nil {
				continue
			}
			fks = append(fks, ForeignKeyRelationship{
				FromTable:  table,
				FromColumn: fromCol,
				ToTable:    refTable,
				ToColumn:   toCol,
			})
		}
		_ = rows.Close()
	}

	return fks, nil
}

// commonColumns lists column names too generic to imply FK relationships.
// These appear in many tables (audit columns, generic identifiers) and cause false positives.
var commonColumns = map[string]bool{
	"id": true, "created": true, "updated": true, "deleted": true,
	"name": true, "description": true, "status": true, "type": true,
	"notes": true, "active": true, "enabled": true, "sort_order": true,
	"created_at": true, "updated_at": true, "deleted_at": true,
	"created_by": true, "updated_by": true,
}

// isCommonColumn returns true if the column name is too generic to imply a FK relationship.
func isCommonColumn(col string) bool {
	return commonColumns[col]
}

// DetectImplicitRelationships infers relationships from column naming conventions.
// BUG-006 fix: only matches columns with _id suffix that reference a table name.
// Common columns (id, created, name, etc.) are excluded to prevent false positives.
func DetectImplicitRelationships(tableColumns map[string][]SchemaColumnInfo) []SemanticRelationship {
	tableSet := make(map[string]bool)
	for t := range tableColumns {
		tableSet[t] = true
	}

	var rels []SemanticRelationship
	for fromTable, columns := range tableColumns {
		for _, col := range columns {
			if rel, ok := matchColumnRelationship(fromTable, col, tableSet); ok {
				rels = append(rels, rel)
			}
		}
	}
	return rels
}

// matchColumnRelationship checks if a single column implies a semantic relationship.
func matchColumnRelationship(fromTable string, col SchemaColumnInfo, tableSet map[string]bool) (SemanticRelationship, bool) {
	if col.IsPrimaryKey || isCommonColumn(col.ColumnName) {
		return SemanticRelationship{}, false
	}
	if matchedTable, ok := matchExactID(col.ColumnName, tableSet); ok {
		return SemanticRelationship{
			Tables:           []string{fromTable, matchedTable},
			RelationshipType: "many_to_one",
			ConnectionBasis:  "naming_convention:exact_id",
			ConfidenceScore:  0.8,
			FromColumn:       col.ColumnName,
			ToColumn:         "id",
		}, true
	}
	if matchedTable, ok := matchSuffixID(col.ColumnName, tableSet); ok {
		return SemanticRelationship{
			Tables:           []string{fromTable, matchedTable},
			RelationshipType: "many_to_one",
			ConnectionBasis:  "naming_convention:suffix_id",
			ConfidenceScore:  0.7,
			FromColumn:       col.ColumnName,
			ToColumn:         "id",
		}, true
	}
	return SemanticRelationship{}, false
}

// matchExactID checks if columnName is "tableName_id" or a singularized variant.
// e.g., "user_id" matches "users", "product_id" matches "products".
func matchExactID(colName string, tableSet map[string]bool) (string, bool) {
	if len(colName) <= 3 {
		return "", false
	}
	if colName[len(colName)-3:] != "_id" {
		return "", false
	}
	prefix := colName[:len(colName)-3]

	// Direct match: column "users_id" matches table "users"
	if tableSet[prefix] {
		return prefix, true
	}

	// Singular→plural: column "user_id" matches table "users"
	if matched, ok := singularToPluralMatch(prefix, tableSet); ok {
		return matched, true
	}

	// Plural→singular: column "users_id" matches table "user"
	if len(prefix) > 1 && prefix[len(prefix)-1] == 's' {
		singular := prefix[:len(prefix)-1]
		if tableSet[singular] {
			return singular, true
		}
	}

	return "", false
}

// singularToPluralMatch tries common English pluralization rules to find a matching table.
// Handles: +s, +es, -y→-ies, -on→-a.
func singularToPluralMatch(singular string, tableSet map[string]bool) (string, bool) {
	candidates := []string{
		singular + "s",  // user → users
		singular + "es", // box → boxes
	}
	// -y → -ies (category → categories)
	if len(singular) > 1 && singular[len(singular)-1] == 'y' {
		candidates = append(candidates, singular[:len(singular)-1]+"ies")
	}
	// -on → -a (criterion → criteria)
	if len(singular) > 2 && singular[len(singular)-2:] == "on" {
		candidates = append(candidates, singular[:len(singular)-2]+"a")
	}

	for _, c := range candidates {
		if tableSet[c] {
			return c, true
		}
	}
	return "", false
}

// matchSuffixID checks if columnName ends with "_id" and the part before _id
// contains a table name (e.g., "order_items_product_id" matches "products").
func matchSuffixID(colName string, tableSet map[string]bool) (string, bool) {
	if len(colName) <= 3 || colName[len(colName)-3:] != "_id" {
		return "", false
	}
	prefix := colName[:len(colName)-3]

	// Already exact-matched by matchExactID
	if tableSet[prefix] {
		return "", false
	}

	// Check each underscore-delimited segment as a potential table name
	for i := 0; i < len(prefix); i++ {
		if prefix[i] == '_' {
			if matched, ok := matchTableSegment(prefix[i+1:], tableSet); ok {
				return matched, true
			}
		}
	}
	return "", false
}

// matchTableSegment tries to match a substring segment against table names.
func matchTableSegment(sub string, tableSet map[string]bool) (string, bool) {
	if tableSet[sub] {
		return sub, true
	}
	if matched, ok := singularToPluralMatch(sub, tableSet); ok {
		return matched, true
	}
	if len(sub) > 1 && sub[len(sub)-1] == 's' {
		singular := sub[:len(sub)-1]
		if tableSet[singular] {
			return singular, true
		}
	}
	return "", false
}
