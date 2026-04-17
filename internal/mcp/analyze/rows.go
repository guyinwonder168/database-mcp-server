package analyze

import (
	"context"
	"database/sql"
	"fmt"
)

// rows.go handles row count queries for all backends.
// BUG-005 fix: populates TableInfo.RowCount from information_schema.TABLES / pg_stat_user_tables / COUNT(*).

// FetchRowCounts retrieves estimated row counts for the given tables.
// MySQL/MariaDB: uses information_schema.TABLES.TABLE_ROWS (fast estimate).
// PostgreSQL: uses pg_stat_user_tables.n_live_tup (fast estimate).
// SQLite: uses SELECT COUNT(*) per table (exact but slower).
func FetchRowCounts(ctx context.Context, db *sql.DB, dbType, schema string, tableNames []string) (map[string]int64, error) {
	switch dbType {
	case "mysql", "mariadb":
		return fetchRowCountsMySQL(ctx, db, schema)
	case "postgres", "postgresql":
		return fetchRowCountsPostgres(ctx, db, schema)
	case "sqlite":
		return fetchRowCountsSQLite(ctx, db, tableNames)
	default:
		return nil, fmt.Errorf("unsupported db type for row counts: %s", dbType)
	}
}

func fetchRowCountsMySQL(ctx context.Context, db *sql.DB, schema string) (map[string]int64, error) {
	query := `SELECT TABLE_NAME, TABLE_ROWS
		FROM information_schema.TABLES
		WHERE TABLE_SCHEMA = ? AND TABLE_TYPE = 'BASE TABLE'`

	rows, err := db.QueryContext(ctx, query, schema)
	if err != nil {
		return nil, fmt.Errorf("mysql row counts: %w", err)
	}
	defer rows.Close()

	result := make(map[string]int64)
	for rows.Next() {
		var tableName string
		var tableRows int64
		if err := rows.Scan(&tableName, &tableRows); err != nil {
			continue
		}
		result[tableName] = tableRows
	}
	return result, rows.Err()
}

func fetchRowCountsPostgres(ctx context.Context, db *sql.DB, schema string) (map[string]int64, error) {
	query := `SELECT relname, n_live_tup
		FROM pg_stat_user_tables
		WHERE schemaname = $1`

	rows, err := db.QueryContext(ctx, query, schema)
	if err != nil {
		return nil, fmt.Errorf("postgres row counts: %w", err)
	}
	defer rows.Close()

	result := make(map[string]int64)
	for rows.Next() {
		var relname string
		var nLiveTup int64
		if err := rows.Scan(&relname, &nLiveTup); err != nil {
			continue
		}
		result[relname] = nLiveTup
	}
	return result, rows.Err()
}

func fetchRowCountsSQLite(ctx context.Context, db *sql.DB, tableNames []string) (map[string]int64, error) {
	result := make(map[string]int64)
	for _, table := range tableNames {
		query := fmt.Sprintf(`SELECT COUNT(*) AS cnt FROM "%s"`, table)
		var cnt int64
		err := db.QueryRowContext(ctx, query).Scan(&cnt)
		if err != nil {
			// Silently skip — individual table failures shouldn't abort entire analysis
			continue
		}
		result[table] = cnt
	}
	return result, nil
}
