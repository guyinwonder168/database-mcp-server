package analyze

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
)

// performance.go handles index analysis and performance optimization recommendations.
// BUG-009 fix: populates PerformanceOptimization from information_schema.STATISTICS / pg_indexes / PRAGMA.

// FetchIndexes retrieves index metadata for all tables in the given schema/database.
// MySQL/MariaDB: uses information_schema.STATISTICS (one row per column per index).
// PostgreSQL: uses pg_indexes (one row per index, indexdef parsed for columns).
// SQLite: uses PRAGMA index_list + PRAGMA index_info per table.
func FetchIndexes(ctx context.Context, db *sql.DB, dbType, schema string, sqliteTables ...[]string) ([]IndexInfo, error) {
	switch dbType {
	case "mysql", "mariadb":
		return fetchIndexesMySQL(ctx, db, schema)
	case "postgres", "postgresql":
		return fetchIndexesPostgres(ctx, db, schema)
	case "sqlite":
		tables := optionalStrings(sqliteTables)
		return fetchIndexesSQLite(ctx, db, tables)
	default:
		return nil, fmt.Errorf("unsupported db type for index fetching: %s", dbType)
	}
}

func fetchIndexesMySQL(ctx context.Context, db *sql.DB, schema string) ([]IndexInfo, error) {
	query := `SELECT TABLE_NAME, INDEX_NAME, COLUMN_NAME, NON_UNIQUE, SEQ_IN_INDEX
		FROM information_schema.STATISTICS
		WHERE TABLE_SCHEMA = ?
		ORDER BY TABLE_NAME, INDEX_NAME, SEQ_IN_INDEX`

	rows, err := db.QueryContext(ctx, query, schema)
	if err != nil {
		return nil, fmt.Errorf("mysql index fetch: %w", err)
	}
	defer func() { _ = rows.Close() }()

	// MySQL returns one row per column in an index; group by table+index
	type indexKey struct {
		TableName string
		IndexName string
	}
	indexMap := make(map[indexKey]*IndexInfo)
	var order []indexKey // preserve order

	for rows.Next() {
		var tableName, indexName, columnName string
		var nonUnique int
		var seqInIndex int

		if err := rows.Scan(&tableName, &indexName, &columnName, &nonUnique, &seqInIndex); err != nil {
			continue
		}

		key := indexKey{TableName: tableName, IndexName: indexName}
		idx, exists := indexMap[key]
		if !exists {
			idx = &IndexInfo{
				TableName: tableName,
				IndexName: indexName,
				IsUnique:  nonUnique == 0,
				IsPrimary: indexName == "PRIMARY",
			}
			indexMap[key] = idx
			order = append(order, key)
		}
		// Append columns in sequence order
		for len(idx.Columns) < seqInIndex {
			idx.Columns = append(idx.Columns, "")
		}
		idx.Columns[seqInIndex-1] = columnName
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Build result in order
	var result []IndexInfo
	for _, key := range order {
		idx := indexMap[key]
		// Trim any trailing empty slots
		for len(idx.Columns) > 0 && idx.Columns[len(idx.Columns)-1] == "" {
			idx.Columns = idx.Columns[:len(idx.Columns)-1]
		}
		result = append(result, *idx)
	}

	return result, nil
}

func fetchIndexesPostgres(ctx context.Context, db *sql.DB, schema string) ([]IndexInfo, error) {
	query := `SELECT tablename, indexname, indexdef
		FROM pg_indexes
		WHERE schemaname = $1
		ORDER BY tablename, indexname`

	rows, err := db.QueryContext(ctx, query, schema)
	if err != nil {
		return nil, fmt.Errorf("postgres index fetch: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []IndexInfo
	for rows.Next() {
		var tableName, indexName, indexdef string
		if err := rows.Scan(&tableName, &indexName, &indexdef); err != nil {
			continue
		}
		columns := parseIndexColumns(indexdef)
		isPrimary := strings.HasSuffix(indexName, "_pkey")
		isUnique := strings.Contains(strings.ToUpper(indexdef), "UNIQUE")

		result = append(result, IndexInfo{
			TableName: tableName,
			IndexName: indexName,
			Columns:   columns,
			IsUnique:  isUnique,
			IsPrimary: isPrimary,
		})
	}

	return result, rows.Err()
}

// colRegex extracts column names from indexdef like:
// CREATE UNIQUE INDEX idx_name ON public.table USING btree (col1, col2)
var colRegex = regexp.MustCompile(`\(([^)]+)\)`)

// parseIndexColumns extracts the column list from a PostgreSQL indexdef string.
func parseIndexColumns(indexdef string) []string {
	matches := colRegex.FindStringSubmatch(indexdef)
	if len(matches) <= 1 {
		return nil
	}
	var columns []string
	for _, col := range strings.Split(matches[1], ",") {
		col = strings.TrimSpace(col)
		if idx := strings.Index(col, "::"); idx != -1 {
			col = col[:idx]
		}
		col = strings.TrimSpace(col)
		if col != "" {
			columns = append(columns, col)
		}
	}
	return columns
}

func fetchIndexesSQLite(ctx context.Context, db *sql.DB, tableNames []string) ([]IndexInfo, error) {
	var result []IndexInfo

	for _, table := range tableNames {
		indexes := fetchSQLiteIndexesForTable(ctx, db, table)
		result = append(result, indexes...)
	}

	return result, nil
}

// fetchSQLiteIndexesForTable retrieves all indexes for a single SQLite table.
func fetchSQLiteIndexesForTable(ctx context.Context, db *sql.DB, table string) []IndexInfo {
	listRows, err := db.QueryContext(ctx,
		`SELECT seq, name, "unique" FROM pragma_index_list WHERE arg = ?`,
		table)
	if err != nil {
		return nil
	}

	var indexes []IndexInfo
	for listRows.Next() {
		var seq int
		var indexName string
		var unique int
		if err := listRows.Scan(&seq, &indexName, &unique); err != nil {
			continue
		}
		columns := fetchSQLiteIndexColumns(ctx, db, indexName)
		isPrimary := strings.HasPrefix(indexName, "sqlite_autoindex_")

		indexes = append(indexes, IndexInfo{
			TableName: table,
			IndexName: indexName,
			Columns:   columns,
			IsUnique:  unique == 1,
			IsPrimary: isPrimary,
		})
	}
	_ = listRows.Close()
	return indexes
}

// fetchSQLiteIndexColumns retrieves the column list for a single SQLite index.
func fetchSQLiteIndexColumns(ctx context.Context, db *sql.DB, indexName string) []string {
	infoRows, err := db.QueryContext(ctx,
		`SELECT seqno, cid, name FROM pragma_index_info WHERE arg = ?`,
		indexName)
	if err != nil {
		return nil
	}

	var columns []string
	for infoRows.Next() {
		var seqno, cid int
		var colName string
		if err := infoRows.Scan(&seqno, &cid, &colName); err != nil {
			continue
		}
		for len(columns) <= seqno {
			columns = append(columns, "")
		}
		columns[seqno] = colName
	}
	_ = infoRows.Close()
	return columns
}

// BuildPerformanceOptimization analyzes existing indexes, foreign keys, and table columns
// to generate index recommendations and query pattern hints.
// BUG-009 fix: populates PerformanceOptimization from real index data.
func BuildPerformanceOptimization(
	tableColumns map[string][]SchemaColumnInfo,
	fks []ForeignKeyRelationship,
	existingIndexes []IndexInfo,
) PerformanceOptimization {
	result := PerformanceOptimization{
		QueryPatterns: QueryPatterns{
			Prefer: []string{
				"Use indexed columns in WHERE clauses for better performance",
				"Prefer JOINs on foreign key columns (often indexed)",
				"Use LIMIT with ORDER BY for pagination queries",
				"Avoid SELECT * — specify only needed columns",
			},
			Avoid: []string{
				"Avoid LIKE 'prefix%' on unindexed columns (use full-text search for large datasets)",
				"Avoid functions on indexed columns in WHERE clauses (e.g., WHERE YEAR(date_col) = 2024)",
				"Avoid implicit type conversions in JOIN conditions",
			},
		},
	}

	if len(tableColumns) == 0 {
		return result
	}

	// Build a set of existing indexed columns per table
	indexedCols := make(map[string]map[string]bool) // table -> set of columns
	for _, idx := range existingIndexes {
		if indexedCols[idx.TableName] == nil {
			indexedCols[idx.TableName] = make(map[string]bool)
		}
		for _, col := range idx.Columns {
			indexedCols[idx.TableName][col] = true
		}
	}

	// Recommend indexes on foreign key columns that lack indexes
	seen := make(map[string]bool) // "table.column" -> seen
	for _, fk := range fks {
		key := fk.FromTable + "." + fk.FromColumn
		if seen[key] {
			continue
		}
		seen[key] = true

		// Check if this column is already indexed
		if cols, ok := indexedCols[fk.FromTable]; ok && cols[fk.FromColumn] {
			continue // already indexed
		}

		result.RecommendedIndexes = append(result.RecommendedIndexes, RecommendedIndex{
			Table:   fk.FromTable,
			Columns: []string{fk.FromColumn},
			Reason:  fmt.Sprintf("Foreign key column referencing %s.%s — index improves JOIN performance", fk.ToTable, fk.ToColumn),
		})
	}

	return result
}
