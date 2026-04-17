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
	defer rows.Close()

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
	defer rows.Close()

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
		query := fmt.Sprintf("PRAGMA foreign_key_list('%s')", table)
		rows, err := db.QueryContext(ctx, query)
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
		rows.Close()
	}

	return fks, nil
}
