package analyze

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
)

// rows.go handles row count queries and sample row fetching for all backends.
// BUG-005 fix: populates TableInfo.RowCount from information_schema.TABLES / pg_stat_user_tables / COUNT(*).

// --- Sample Row Fetching ---

// NormalizeSampleSize returns a valid sample size, defaulting to 10 if <= 0.
func NormalizeSampleSize(sampleSize int) int {
	if sampleSize <= 0 {
		return 10
	}
	return sampleSize
}

// FetchSampleRows retrieves sample rows from a table for the given database type.
// Returns empty slice on error (never fails the caller).
func FetchSampleRows(ctx context.Context, db *sql.DB, tableName, dbType string, sampleSize int) []map[string]interface{} {
	sampleQuery, ok := SampleQueryForDB(dbType, tableName, sampleSize)
	if !ok {
		return []map[string]interface{}{}
	}
	rows, err := db.QueryContext(ctx, sampleQuery)
	if err != nil {
		return []map[string]interface{}{}
	}
	defer rows.Close() //nolint:errcheck
	return ScanSampleRows(rows, tableName)
}

// SampleQueryForDB returns the SQL query to fetch sample rows and whether the dbType is supported.
func SampleQueryForDB(dbType, tableName string, sampleSize int) (string, bool) {
	if err := sanitizeIdentifier(tableName); err != nil {
		return "", false
	}
	quoted, ok := quoteForDB(dbType, tableName)
	if !ok {
		return "", false
	}
	return "SELECT * FROM " + quoted + " LIMIT " + strconv.Itoa(sampleSize), true
}

// ScanSampleRows scans sql.Rows into a slice of row maps.
func ScanSampleRows(rows *sql.Rows, tableName string) []map[string]interface{} {
	sampleRows := make([]map[string]interface{}, 0)
	columns, err := rows.Columns()
	if err != nil {
		return sampleRows
	}
	for rows.Next() {
		row := make([]interface{}, len(columns))
		ptrs := make([]interface{}, len(columns))
		for idx := range row {
			ptrs[idx] = &row[idx]
		}
		if err := rows.Scan(ptrs...); err != nil {
			continue
		}
		sampleRows = append(sampleRows, NormalizeSampleRow(columns, row))
	}
	return sampleRows
}

// NormalizeSampleRow converts a scanned row into a map, converting []byte to string.
func NormalizeSampleRow(columns []string, row []interface{}) map[string]interface{} {
	rowMap := make(map[string]interface{}, len(columns))
	for idx, value := range row {
		if bytes, ok := value.([]byte); ok {
			rowMap[columns[idx]] = string(bytes)
			continue
		}
		rowMap[columns[idx]] = value
	}
	return rowMap
}

// --- Row Count Fetching ---

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
	defer func() { _ = rows.Close() }()

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
	defer func() { _ = rows.Close() }()

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
		if err := sanitizeIdentifier(table); err != nil {
			continue
		}
		query := "SELECT COUNT(*) AS cnt FROM " + quoteSQLite(table)
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
