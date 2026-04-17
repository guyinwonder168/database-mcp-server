package analyze

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// columns.go handles column metadata queries for MySQL/MariaDB, PostgreSQL, and SQLite.
// BUG-004 fix: uses bulk information_schema.COLUMNS queries instead of per-table SHOW COLUMNS.

// FetchColumnsBulk retrieves column metadata for all tables in the given schema/database.
// Uses bulk information_schema queries for MySQL and PostgreSQL.
// For SQLite, use FetchColumnsPerTable instead (no information_schema available).
func FetchColumnsBulk(ctx context.Context, db *sql.DB, dbType, schema string) (map[string][]SchemaColumnInfo, error) {
	result := make(map[string][]SchemaColumnInfo)

	switch dbType {
	case "mysql", "mariadb":
		if err := fetchColumnsBulkMySQL(ctx, db, schema, result); err != nil {
			return nil, fmt.Errorf("mysql bulk columns: %w", err)
		}
	case "postgres", "postgresql":
		if err := fetchColumnsBulkPostgres(ctx, db, schema, result); err != nil {
			return nil, fmt.Errorf("postgres bulk columns: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported db type for bulk columns: %s (use FetchColumnsPerTable for sqlite)", dbType)
	}

	return result, nil
}

func fetchColumnsBulkMySQL(ctx context.Context, db *sql.DB, schema string, result map[string][]SchemaColumnInfo) error {
	query := `SELECT TABLE_NAME, COLUMN_NAME, DATA_TYPE, IS_NULLABLE, COLUMN_DEFAULT, COLUMN_KEY, EXTRA
		FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = ?
		ORDER BY TABLE_NAME, ORDINAL_POSITION`

	rows, err := db.QueryContext(ctx, query, schema)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var tableName, colName, dataType, isNullable, colKey, extra string
		var colDefault sql.NullString

		if err := rows.Scan(&tableName, &colName, &dataType, &isNullable, &colDefault, &colKey, &extra); err != nil {
			return err
		}

		col := SchemaColumnInfo{
			ColumnName:   colName,
			DataType:     dataType,
			IsNullable:   strings.ToUpper(isNullable) == "YES",
			IsPrimaryKey: colKey == "PRI",
			Unique:       colKey == "PRI" || colKey == "UNI",
			Indexed:      colKey != "",
		}
		if colDefault.Valid {
			col.DefaultValue = colDefault.String
		}

		result[tableName] = append(result[tableName], col)
	}

	return rows.Err()
}

func fetchColumnsBulkPostgres(ctx context.Context, db *sql.DB, schema string, result map[string][]SchemaColumnInfo) error {
	query := `SELECT c.table_name, c.column_name, c.data_type, c.is_nullable, c.column_default,
		COALESCE(
			(SELECT tc.constraint_type
			 FROM information_schema.key_column_usage kcu
			 JOIN information_schema.table_constraints tc
			   ON kcu.constraint_name = tc.constraint_name
			  AND kcu.table_schema = tc.table_schema
			WHERE kcu.table_name = c.table_name
			  AND kcu.column_name = c.column_name
			  AND kcu.table_schema = c.table_schema
			LIMIT 1), '') AS constraint_type
		FROM information_schema.columns c
		WHERE c.table_schema = ?
		ORDER BY c.table_name, c.ordinal_position`

	rows, err := db.QueryContext(ctx, query, schema)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var tableName, colName, dataType, isNullable, constraintType string
		var colDefault sql.NullString

		if err := rows.Scan(&tableName, &colName, &dataType, &isNullable, &colDefault, &constraintType); err != nil {
			return err
		}

		ct := strings.ToUpper(constraintType)
		col := SchemaColumnInfo{
			ColumnName:   colName,
			DataType:     dataType,
			IsNullable:   strings.ToUpper(isNullable) == "YES",
			IsPrimaryKey: ct == "PRIMARY KEY",
			Unique:       ct == "PRIMARY KEY" || ct == "UNIQUE",
			Indexed:      ct != "",
		}
		if colDefault.Valid {
			col.DefaultValue = colDefault.String
		}

		result[tableName] = append(result[tableName], col)
	}

	return rows.Err()
}
