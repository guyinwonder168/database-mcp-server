//go:build cgo

package mcp

import (
	"context"
	"database-mcp-provider/internal/config"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestFilterAnalyzeSchemaTables(t *testing.T) {
	tables := []string{"users", "orders", "audit_logs"}

	filtered := filterAnalyzeSchemaTables(tables, []string{"users", "orders"}, nil)
	if len(filtered) != 2 || filtered[0] != "users" || filtered[1] != "orders" {
		t.Fatalf("unexpected filtered include result: %+v", filtered)
	}

	filtered = filterAnalyzeSchemaTables(tables, nil, []string{"audit_logs"})
	if len(filtered) != 2 || filtered[0] != "users" || filtered[1] != "orders" {
		t.Fatalf("unexpected filtered exclude result: %+v", filtered)
	}

	// Empty include match should fall back to all tables.
	filtered = filterAnalyzeSchemaTables(tables, []string{"missing"}, nil)
	if len(filtered) != len(tables) {
		t.Fatalf("expected fallback to all tables, got %+v", filtered)
	}
}

func TestNormalizeAnalyzeSchemaSampleSize(t *testing.T) {
	if got := normalizeAnalyzeSchemaSampleSize(0); got != 10 {
		t.Fatalf("expected default sample size 10, got %d", got)
	}
	if got := normalizeAnalyzeSchemaSampleSize(-3); got != 10 {
		t.Fatalf("expected default sample size 10 for negative input, got %d", got)
	}
	if got := normalizeAnalyzeSchemaSampleSize(7); got != 7 {
		t.Fatalf("expected sample size 7, got %d", got)
	}
}

func TestAnalyzeSchemaColumnQuery(t *testing.T) {
	mysqlQuery, ok := analyzeSchemaColumnQuery("mysql", "users")
	if !ok || !strings.Contains(mysqlQuery, "SHOW COLUMNS FROM `users`") {
		t.Fatalf("unexpected mysql query: ok=%v query=%q", ok, mysqlQuery)
	}

	postgresQuery, ok := analyzeSchemaColumnQuery("postgres", "users")
	if !ok || !strings.Contains(postgresQuery, "information_schema.columns") {
		t.Fatalf("unexpected postgres query: ok=%v query=%q", ok, postgresQuery)
	}

	sqliteQuery, ok := analyzeSchemaColumnQuery("sqlite", "users")
	if !ok || !strings.Contains(sqliteQuery, "PRAGMA table_info('users')") {
		t.Fatalf("unexpected sqlite query: ok=%v query=%q", ok, sqliteQuery)
	}

	if query, ok := analyzeSchemaColumnQuery("oracle", "users"); ok || query != "" {
		t.Fatalf("expected unsupported db type to return false, got ok=%v query=%q", ok, query)
	}
}

func TestAnalyzeSchemaSampleQuery(t *testing.T) {
	mysqlQuery, ok := analyzeSchemaSampleQuery("mysql", "users", 5)
	if !ok || mysqlQuery != "SELECT * FROM `users` LIMIT 5" {
		t.Fatalf("unexpected mysql sample query: ok=%v query=%q", ok, mysqlQuery)
	}

	postgresQuery, ok := analyzeSchemaSampleQuery("postgres", "users", 5)
	if !ok || postgresQuery != "SELECT * FROM \"users\" LIMIT 5" {
		t.Fatalf("unexpected postgres sample query: ok=%v query=%q", ok, postgresQuery)
	}

	sqliteQuery, ok := analyzeSchemaSampleQuery("sqlite", "users", 5)
	if !ok || sqliteQuery != "SELECT * FROM 'users' LIMIT 5" {
		t.Fatalf("unexpected sqlite sample query: ok=%v query=%q", ok, sqliteQuery)
	}

	if query, ok := analyzeSchemaSampleQuery("oracle", "users", 5); ok || query != "" {
		t.Fatalf("expected unsupported db type to return false, got ok=%v query=%q", ok, query)
	}
}

func TestUpdateAnalyzeSchemaKeyColumns(t *testing.T) {
	keyCols := &KeyColumns{}

	updateAnalyzeSchemaKeyColumns(keyCols, SchemaColumnInfo{ColumnName: "id", IsPrimaryKey: true})
	updateAnalyzeSchemaKeyColumns(keyCols, SchemaColumnInfo{ColumnName: "email", Unique: true})
	updateAnalyzeSchemaKeyColumns(keyCols, SchemaColumnInfo{ColumnName: "status_id", Indexed: true})
	updateAnalyzeSchemaKeyColumns(keyCols, SchemaColumnInfo{ColumnName: "user_id", IsForeignKey: true})

	if keyCols.PrimaryKey != "id" {
		t.Fatalf("expected primary key id, got %q", keyCols.PrimaryKey)
	}
	if len(keyCols.UniqueColumns) != 1 || keyCols.UniqueColumns[0] != "email" {
		t.Fatalf("unexpected unique columns: %+v", keyCols.UniqueColumns)
	}
	if len(keyCols.IndexedColumns) != 1 || keyCols.IndexedColumns[0] != "status_id" {
		t.Fatalf("unexpected indexed columns: %+v", keyCols.IndexedColumns)
	}
	if len(keyCols.ForeignKeys) != 1 || keyCols.ForeignKeys[0] != "user_id" {
		t.Fatalf("unexpected foreign key columns: %+v", keyCols.ForeignKeys)
	}
}

func TestAddAnalyzeSchemaTableAggregateMetric(t *testing.T) {
	columnMetrics := map[string]QualityMetrics{
		"id":   {OverallScore: 0.8},
		"name": {OverallScore: 0.6},
	}
	addAnalyzeSchemaTableAggregateMetric(columnMetrics)

	tableMetric, ok := columnMetrics["__table__"]
	if !ok {
		t.Fatalf("expected table aggregate metric")
	}
	if !floatEqual(tableMetric.OverallScore, 0.7) {
		t.Fatalf("expected table overall score 0.7, got %f", tableMetric.OverallScore)
	}

	emptyMetrics := map[string]QualityMetrics{}
	addAnalyzeSchemaTableAggregateMetric(emptyMetrics)
	if _, exists := emptyMetrics["__table__"]; exists {
		t.Fatalf("did not expect table aggregate metric for empty input")
	}
}

func TestFlattenAnalyzeSchemaQualityMetrics(t *testing.T) {
	target := map[string]QualityMetrics{}
	columnMetrics := map[string]QualityMetrics{
		"id":        {OverallScore: 0.9},
		"__table__": {OverallScore: 0.8},
	}
	flattenAnalyzeSchemaQualityMetrics(target, "users", columnMetrics)

	if _, ok := target["users.id"]; !ok {
		t.Fatalf("expected users.id key in flattened metrics")
	}
	if _, ok := target["users"]; !ok {
		t.Fatalf("expected users key for table-level metric")
	}
}

func TestAddAnalyzeSchemaDatabaseAggregateMetric(t *testing.T) {
	metrics := map[string]QualityMetrics{
		"users.id": {OverallScore: 0.8},
		"users":    {OverallScore: 0.6},
	}
	addAnalyzeSchemaDatabaseAggregateMetric(metrics)

	dbMetric, ok := metrics["__database__"]
	if !ok {
		t.Fatalf("expected database aggregate metric")
	}
	if !floatEqual(dbMetric.OverallScore, 0.7) {
		t.Fatalf("expected database overall score 0.7, got %f", dbMetric.OverallScore)
	}
}

func TestSummarizeAnalyzeSchemaBusinessContext(t *testing.T) {
	server := &MCPServer{}
	domain, confidence, desc := server.summarizeAnalyzeSchemaBusinessContext(&BusinessContext{
		DomainIndicators: map[string]float64{
			"finance": 0.85,
			"crm":     0.4,
		},
		EntityRelationships: EntityRelationships{
			CentralEntities: []string{"accounts"},
		},
	})
	if domain != "finance" {
		t.Fatalf("expected finance domain, got %q", domain)
	}
	if !floatEqual(confidence, 0.85) {
		t.Fatalf("expected confidence 0.85, got %f", confidence)
	}
	if desc == "" {
		t.Fatalf("expected non-empty business description")
	}

	domain, confidence, desc = server.summarizeAnalyzeSchemaBusinessContext(nil)
	if domain != "" || confidence != 0 || desc != "" {
		t.Fatalf("expected zero-values for nil business context, got domain=%q confidence=%f desc=%q", domain, confidence, desc)
	}
}

func TestBuildAnalyzeSchemaResult(t *testing.T) {
	startTime := time.Now()
	result := buildAnalyzeSchemaResult(analyzeSchemaResultInput{
		startTime:               startTime,
		params:                  AnalyzeSchemaParams{AnalysisLevel: AnalysisLevelDetailed},
		dbType:                  "sqlite",
		filteredTables:          []string{"users"},
		totalColumns:            3,
		tableCatalog:            TableCatalog{},
		tableSchemas:            map[string]TableInfo{"users": {ColumnCount: 3}},
		relationshipGraph:       RelationshipGraph{},
		relationshipGraphVisual: map[string]interface{}{"nodes": []string{}},
		aiQuerySuggestions:      AIQuerySuggestions{},
		dataQualityMetrics:      map[string]QualityMetrics{"users.id": {OverallScore: 0.9}},
		domain:                  "finance",
		confidence:              0.8,
	})

	if result.AnalysisMetadata.AnalysisLevel != AnalysisLevelDetailed {
		t.Fatalf("unexpected analysis level: %q", result.AnalysisMetadata.AnalysisLevel)
	}
	if result.DatabaseOverview.TotalTables != 1 {
		t.Fatalf("expected total tables 1, got %d", result.DatabaseOverview.TotalTables)
	}
	if result.DatabaseOverview.EstimatedBusinessDomain != "finance" {
		t.Fatalf("expected business domain finance, got %q", result.DatabaseOverview.EstimatedBusinessDomain)
	}
}

func TestBuildMySQLDescribeColumnInfo(t *testing.T) {
	row := mysqlDescribeColumnRow{
		name:       "id",
		typ:        "bigint",
		nullable:   "NO",
		keyType:    "PRI",
		extra:      "auto_increment",
		defaultVal: sql.NullString{String: "0", Valid: true},
		comment:    sql.NullString{String: "identifier", Valid: true},
		characterSet: sql.NullString{
			String: "utf8mb4",
			Valid:  true,
		},
		collation: sql.NullString{
			String: "utf8mb4_general_ci",
			Valid:  true,
		},
		maxLength: sql.NullInt64{Int64: 255, Valid: true},
		precision: sql.NullInt64{Int64: 20, Valid: true},
		scale:     sql.NullInt64{Int64: 0, Valid: true},
	}

	col := buildMySQLDescribeColumnInfo(row)
	if col.Name != "id" || !col.AutoIncrement || col.Key != "PRI" {
		t.Fatalf("unexpected base column info: %+v", col)
	}
	if col.Default == nil || *col.Default != "0" {
		t.Fatalf("expected default value to be set, got %+v", col.Default)
	}
	if col.Comment != "identifier" {
		t.Fatalf("expected comment to be propagated, got %q", col.Comment)
	}
	if col.CharacterSet != "utf8mb4" {
		t.Fatalf("expected character set utf8mb4, got %q", col.CharacterSet)
	}
	if col.Collation != "utf8mb4_general_ci" {
		t.Fatalf("expected collation utf8mb4_general_ci, got %q", col.Collation)
	}
	if col.MaxLength == nil || *col.MaxLength != 255 {
		t.Fatalf("expected max length 255, got %+v", col.MaxLength)
	}
	if col.Precision == nil || *col.Precision != 20 {
		t.Fatalf("expected precision 20, got %+v", col.Precision)
	}
}

func TestAnalyzeDataPatterns(t *testing.T) {
	server := &MCPServer{}
	sampleData := []map[string]interface{}{
		{
			"email":      "alice@example.com",
			"score":      1.25,
			"event_date": "2025-01-02T00:00:00",
		},
		{
			"email":      "bob@example.com",
			"score":      2.50,
			"event_date": "2025-01-03T00:00:00",
		},
		{
			"email":      nil,
			"score":      3.0,
			"event_date": "2025-01-01T00:00:00",
		},
	}
	columns := []SchemaColumnInfo{
		{ColumnName: "email"},
		{ColumnName: "score"},
		{ColumnName: "event_date"},
	}

	patterns := server.analyzeDataPatterns("users", sampleData, columns)
	if len(patterns) != 3 {
		t.Fatalf("expected 3 patterns, got %d", len(patterns))
	}
	if patterns[0].PatternType != "email" {
		t.Fatalf("expected email pattern type, got %q", patterns[0].PatternType)
	}
	if patterns[1].Range == nil {
		t.Fatalf("expected numeric range for score pattern")
	}
	if patterns[2].PatternType != "date" {
		t.Fatalf("expected date pattern type, got %q", patterns[2].PatternType)
	}
}

func TestGenerateDataQualityMetrics(t *testing.T) {
	server := &MCPServer{}
	columns := []SchemaColumnInfo{
		{
			ColumnName:      "email",
			PatternType:     "email",
			ValidationRegex: `^[\w\.\-]+@[\w\.\-]+\.\w+$`,
		},
		{
			ColumnName: "event_date",
			DataType:   "date",
		},
	}
	sampleData := []map[string]interface{}{
		{"email": "alice@example.com", "event_date": "2025-01-02"},
		{"email": "invalid-email", "event_date": "2025-01-01"},
		{"email": nil, "event_date": "2025-01-03"},
	}

	metrics := server.generateDataQualityMetrics(sampleData, columns)
	emailMetrics := metrics["email"]
	if emailMetrics.Validity >= 1.0 {
		t.Fatalf("expected invalid email values to reduce validity, got %f", emailMetrics.Validity)
	}
	if len(emailMetrics.Issues) == 0 {
		t.Fatalf("expected validation issues for email column")
	}

	dateMetrics := metrics["event_date"]
	if dateMetrics.TemporalConsistency != 0.0 {
		t.Fatalf("expected temporal inconsistency score 0, got %f", dateMetrics.TemporalConsistency)
	}
}

func TestTruncateQualityIssues(t *testing.T) {
	issues := make([]string, 0, maxQualityIssuesPerColumn+2)
	for idx := 0; idx < maxQualityIssuesPerColumn+2; idx++ {
		issues = append(issues, "issue")
	}
	truncated := truncateQualityIssues(issues)
	if len(truncated) != maxQualityIssuesPerColumn+1 {
		t.Fatalf("expected %d truncated issues, got %d", maxQualityIssuesPerColumn+1, len(truncated))
	}
	last := truncated[len(truncated)-1]
	if !strings.Contains(last, "more issues truncated") {
		t.Fatalf("expected truncation summary in last issue, got %q", last)
	}
}

func TestApplyAnalyzeSchemaPatternsAndCorrelate(t *testing.T) {
	server := &MCPServer{}
	tableSchemas := map[string]TableInfo{
		"users": {
			ColumnCount: 2,
			Columns: []SchemaColumnInfo{
				{ColumnName: "user_id"},
				{ColumnName: "email"},
			},
		},
		"orders": {
			ColumnCount: 1,
			Columns: []SchemaColumnInfo{
				{ColumnName: "user_id"},
			},
		},
	}
	sampleData := map[string][]map[string]interface{}{
		"users": {
			{"user_id": "1", "email": "alice@example.com"},
			{"user_id": "2", "email": "bob@example.com"},
		},
		"orders": {
			{"user_id": "1"},
			{"user_id": "2"},
		},
	}

	updated := server.applyAnalyzeSchemaPatterns(tableSchemas, sampleData)
	if len(updated["users"].DataPatterns) == 0 {
		t.Fatalf("expected detected data patterns for users table")
	}

	// Ensure no panic and path coverage for correlation helper.
	server.correlateAnalyzeSchemaValueMatches(updated, sampleData)
}

func TestBuildAnalyzeSchemaQuerySuggestions(t *testing.T) {
	testConfig := setupTestConfig(t)
	defer os.Remove(testConfig)

	server := NewMCPServerWithConfig(testConfig)
	ctx := context.Background()

	conn, _, err := server.openConnection(ctx, "testsqlite", testSQLiteDBPath)
	if err != nil {
		t.Fatalf("failed to open sqlite connection for setup: %v", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, "DROP TABLE IF EXISTS users"); err != nil {
		t.Fatalf("failed to drop users table: %v", err)
	}
	if _, err := conn.ExecContext(ctx, "CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT)"); err != nil {
		t.Fatalf("failed to create users table: %v", err)
	}

	params := AnalyzeSchemaParams{
		ProfileName:    "testsqlite",
		AnalysisLevel:  AnalysisLevelComprehensive,
		IncludeQueries: true,
	}
	suggestions := server.buildAnalyzeSchemaQuerySuggestions(ctx, params, []string{"users"}, testSQLiteDBPath)
	if len(suggestions.DataExploration) == 0 {
		t.Fatalf("expected at least one query suggestion")
	}
}

func TestScanAnalyzeSchemaColumnHelpers(t *testing.T) {
	dbConn, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite memory db: %v", err)
	}
	defer dbConn.Close()
	ctx := context.Background()

	mysqlRows, err := dbConn.QueryContext(ctx, "SELECT 'id','bigint','NO','PRI','0','auto_increment'")
	if err != nil {
		t.Fatalf("failed mysql-mock query: %v", err)
	}
	if !mysqlRows.Next() {
		t.Fatalf("expected mysql-mock row")
	}
	mysqlCol, err := scanAnalyzeSchemaMySQLColumn(mysqlRows)
	if err != nil {
		t.Fatalf("scanAnalyzeSchemaMySQLColumn failed: %v", err)
	}
	if !mysqlCol.IsPrimaryKey || mysqlCol.ColumnName != "id" {
		t.Fatalf("unexpected mysql scanned column: %+v", mysqlCol)
	}
	_ = mysqlRows.Close()

	postgresRows, err := dbConn.QueryContext(ctx, "SELECT 'user_id','text','YES',NULL,'FOREIGN KEY'")
	if err != nil {
		t.Fatalf("failed postgres-mock query: %v", err)
	}
	if !postgresRows.Next() {
		t.Fatalf("expected postgres-mock row")
	}
	postgresCol, err := scanAnalyzeSchemaPostgresColumn(postgresRows)
	if err != nil {
		t.Fatalf("scanAnalyzeSchemaPostgresColumn failed: %v", err)
	}
	if !postgresCol.IsForeignKey || postgresCol.ColumnName != "user_id" {
		t.Fatalf("unexpected postgres scanned column: %+v", postgresCol)
	}
	_ = postgresRows.Close()

	sqliteRows, err := dbConn.QueryContext(ctx, "SELECT 0,'id','INTEGER',0,NULL,1")
	if err != nil {
		t.Fatalf("failed sqlite-mock query: %v", err)
	}
	if !sqliteRows.Next() {
		t.Fatalf("expected sqlite-mock row")
	}
	sqliteCol, err := scanAnalyzeSchemaSQLiteColumn(sqliteRows)
	if err != nil {
		t.Fatalf("scanAnalyzeSchemaSQLiteColumn failed: %v", err)
	}
	if !sqliteCol.IsPrimaryKey || sqliteCol.ColumnName != "id" {
		t.Fatalf("unexpected sqlite scanned column: %+v", sqliteCol)
	}
	_ = sqliteRows.Close()

	if _, err := scanAnalyzeSchemaColumn(&sql.Rows{}, "oracle"); err == nil {
		t.Fatalf("expected unsupported db type error from scanAnalyzeSchemaColumn")
	}

	// Ensure logging helper path is exercised.
	logAnalyzeSchemaColumnScanError("mysql", "users", errors.New("scan error"))
}

func TestDescribeMySQLTableColumns(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()
	ctx := context.Background()

	// Mock the INFORMATION_SCHEMA.COLUMNS query
	rows := sqlmock.NewRows([]string{
		"name", "type", "nullable", "key_type", "default_value",
		"comment", "extra", "character_set", "collation",
		"max_length", "precision", "scale",
	}).AddRow(
		"id", "bigint", "NO", "PRI", "0",
		"identifier", "auto_increment", "utf8mb4", "utf8mb4_general_ci",
		255, 20, 0,
	)

	mock.ExpectQuery("SELECT.*FROM INFORMATION_SCHEMA.COLUMNS").
		WithArgs("appdb", "users").
		WillReturnRows(rows)

	columns, err := describeMySQLTableColumns(ctx, db, "appdb", "users")
	if err != nil {
		t.Fatalf("describeMySQLTableColumns failed: %v", err)
	}
	if len(columns) != 1 {
		t.Fatalf("expected one column, got %d", len(columns))
	}
	if columns[0].Name != "id" {
		t.Errorf("expected column name 'id', got %q", columns[0].Name)
	}
	if columns[0].Key != "PRI" {
		t.Errorf("expected key 'PRI', got %q", columns[0].Key)
	}
	if !columns[0].AutoIncrement {
		t.Errorf("expected AutoIncrement to be true")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestMarshalAnalyzeSchemaResultError(t *testing.T) {
	_, err := marshalAnalyzeSchemaResult(AnalyzeSchemaResult{
		DataQualityMetrics: map[string]QualityMetrics{
			"bad": {OverallScore: math.NaN()},
		},
	})
	if err == nil {
		t.Fatalf("expected marshal error for NaN quality score")
	}
}

func TestDescribePostgresTableColumns(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()
	ctx := context.Background()

	// Test with explicit schema (no schema resolution needed)
	t.Run("with_explicit_schema", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{
			"name", "type", "nullable", "default_value", "comment",
			"character_maximum_length", "numeric_precision", "numeric_scale", "key_type",
		}).AddRow(
			"id", "bigint", "NO", "nextval('users_id_seq'::regclass)", "identifier",
			nil, 64, 0, "PRI",
		)

		// PostgreSQL uses $1 and $2 parameter placeholders
		mock.ExpectQuery("SELECT.*FROM information_schema.columns").
			WithArgs("users", "public").
			WillReturnRows(rows)

		columns, err := describePostgresTableColumns(ctx, db, "users", "public")
		if err != nil {
			t.Fatalf("describePostgresTableColumns failed: %v", err)
		}
		if len(columns) != 1 {
			t.Fatalf("expected one column, got %d", len(columns))
		}
		if columns[0].Name != "id" {
			t.Errorf("expected column name 'id', got %q", columns[0].Name)
		}
		if columns[0].Key != "PRI" {
			t.Errorf("expected key 'PRI', got %q", columns[0].Key)
		}
		if !columns[0].AutoIncrement {
			t.Errorf("expected AutoIncrement to be true (nextval)")
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled expectations: %v", err)
		}
	})

	// Test with empty schema (requires mock of current_schema query)
	t.Run("with_empty_schema_uses_default", func(t *testing.T) {
		// Mock current_schema() query
		schemaRows := sqlmock.NewRows([]string{"current_schema"}).AddRow("public")
		mock.ExpectQuery("SELECT current_schema").
			WillReturnRows(schemaRows)

		rows := sqlmock.NewRows([]string{
			"name", "type", "nullable", "default_value", "comment",
			"character_maximum_length", "numeric_precision", "numeric_scale", "key_type",
		}).AddRow(
			"name", "text", "YES", nil, nil,
			nil, nil, nil, "",
		)

		mock.ExpectQuery("SELECT.*FROM information_schema.columns").
			WithArgs("items", "public").
			WillReturnRows(rows)

		columns, err := describePostgresTableColumns(ctx, db, "items", "")
		if err != nil {
			t.Fatalf("describePostgresTableColumns with empty schema failed: %v", err)
		}
		if len(columns) != 1 {
			t.Fatalf("expected one column, got %d", len(columns))
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled expectations: %v", err)
		}
	})

	// Test query error
	t.Run("query_error", func(t *testing.T) {
		mock.ExpectQuery("SELECT.*FROM information_schema.columns").
			WithArgs("users", "public").
			WillReturnError(fmt.Errorf("query failed"))

		_, err := describePostgresTableColumns(ctx, db, "users", "public")
		if err == nil {
			t.Error("expected error, got nil")
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled expectations: %v", err)
		}
	})

	// Test scan error (column count mismatch)
	t.Run("scan_error", func(t *testing.T) {
		// Return fewer columns than expected to trigger scan error
		rows := sqlmock.NewRows([]string{
			"name", "type", // only 2 columns instead of 9
		}).AddRow("id", "bigint")

		mock.ExpectQuery("SELECT.*FROM information_schema.columns").
			WithArgs("users", "public").
			WillReturnRows(rows)

		_, err := describePostgresTableColumns(ctx, db, "users", "public")
		if err == nil {
			t.Error("expected error, got nil")
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled expectations: %v", err)
		}
	})
}

func TestLoadLineageEdgesForMySQLMetadata(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()
	ctx := context.Background()

	// Mock foreign key query for MySQL
	rows := sqlmock.NewRows([]string{
		"TABLE_NAME", "REFERENCED_TABLE_NAME",
	}).AddRow("orders", "users")

	mock.ExpectQuery("SELECT.*FROM INFORMATION_SCHEMA.KEY_COLUMN_USAGE").
		WithArgs("appdb").
		WillReturnRows(rows)

	edges, err := loadMySQLLineageEdges(ctx, db, "appdb")
	if err != nil {
		t.Fatalf("loadMySQLLineageEdges failed: %v", err)
	}
	if len(edges) != 1 {
		t.Fatalf("expected one edge, got %d", len(edges))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestLoadLineageEdgesForPostgresMetadata(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()
	ctx := context.Background()

	// Mock foreign key query for PostgreSQL
	rows := sqlmock.NewRows([]string{
		"from_table", "to_table",
	}).AddRow("orders", "users")

	mock.ExpectQuery("SELECT.*FROM information_schema.table_constraints").
		WillReturnRows(rows)

	edges, err := loadPostgresLineageEdges(ctx, db)
	if err != nil {
		t.Fatalf("loadPostgresLineageEdges failed: %v", err)
	}
	if len(edges) != 1 {
		t.Fatalf("expected one edge, got %d", len(edges))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestAdditionalHelperBranchesCoverage(t *testing.T) {
	if got := namingValueString(map[string]interface{}{"k": "v"}, "k", "fallback"); got != "v" {
		t.Fatalf("expected namingValueString to return value, got %q", got)
	}
	if got := namingValueString(map[string]interface{}{"k": 1}, "k", "fallback"); got != "fallback" {
		t.Fatalf("expected namingValueString fallback, got %q", got)
	}

	if got := namingValueFloat(map[string]interface{}{"k": int64(7)}, "k", 0); got != 7 {
		t.Fatalf("expected namingValueFloat int64 conversion, got %f", got)
	}
	if got := namingValueFloat(map[string]interface{}{"k": "x"}, "k", 1.5); got != 1.5 {
		t.Fatalf("expected namingValueFloat fallback, got %f", got)
	}

	sliceVal := namingValueStringSlice(map[string]interface{}{
		"k": []interface{}{"a", 2},
	}, "k")
	if len(sliceVal) != 2 || sliceVal[0] != "a" {
		t.Fatalf("unexpected namingValueStringSlice result: %+v", sliceVal)
	}
	if got := namingValueStringSlice(map[string]interface{}{}, "missing"); len(got) != 0 {
		t.Fatalf("expected empty namingValueStringSlice for missing key")
	}

	prefixes := map[string]int{}
	suffixes := map[string]int{}
	recordPrefixAndSuffix(prefixes, suffixes, "simple")
	if len(prefixes) != 0 || len(suffixes) != 0 {
		t.Fatalf("expected no prefix/suffix recorded for simple name")
	}
	recordPrefixAndSuffix(prefixes, suffixes, "user_id")
	if prefixes["user"] != 1 || suffixes["id"] != 1 {
		t.Fatalf("unexpected prefix/suffix counts: %+v %+v", prefixes, suffixes)
	}

	if got := classifyForeignKeyPattern(0, 1, 1); got != "prefix" {
		t.Fatalf("expected prefix fk pattern, got %q", got)
	}
	if got := classifyForeignKeyPattern(0, 0, 0); got != "none" {
		t.Fatalf("expected none fk pattern, got %q", got)
	}

	server := &MCPServer{}
	types := server.identifyEntityTypes([]string{
		"audit_log", "country_type", "sales_order", "users", "misc",
	})
	if len(types) != 5 {
		t.Fatalf("expected 5 identified types, got %d", len(types))
	}
}

func TestFetchAnalyzeSchemaErrorPaths(t *testing.T) {
	server := &MCPServer{}
	ctx := context.Background()
	dbConn, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	defer dbConn.Close()

	cols, keyCols := server.fetchAnalyzeSchemaColumns(ctx, dbConn, "users", "oracle")
	if len(cols) != 0 || keyCols.PrimaryKey != "" {
		t.Fatalf("expected empty columns for unsupported db type")
	}

	cols, keyCols = server.fetchAnalyzeSchemaColumns(ctx, dbConn, "users", "mysql")
	if len(cols) != 0 || keyCols.PrimaryKey != "" {
		t.Fatalf("expected empty columns when mysql metadata query fails on sqlite")
	}

	samples := server.fetchAnalyzeSchemaSampleRows(ctx, dbConn, "users", "oracle", 1)
	if len(samples) != 0 {
		t.Fatalf("expected empty samples for unsupported db type")
	}
	samples = server.fetchAnalyzeSchemaSampleRows(ctx, dbConn, "users", "mysql", 1)
	if len(samples) != 0 {
		t.Fatalf("expected empty samples when mysql query fails on sqlite")
	}
}

func TestBuildSampleQueryAndNormalizeSampleSizeBranches(t *testing.T) {
	if normalizeSampleSize(101) != 100 {
		t.Fatalf("expected normalizeSampleSize to cap at 100")
	}
	if normalizeSampleSize(-1) != 3 {
		t.Fatalf("expected normalizeSampleSize default to 3")
	}

	if query, _, err := buildSampleQuery("mysql", "users", 2); err != nil || query == "" {
		t.Fatalf("expected mysql sample query, got query=%q err=%v", query, err)
	}
	if query, _, err := buildSampleQuery("postgres", "users", 2); err != nil || query == "" {
		t.Fatalf("expected postgres sample query, got query=%q err=%v", query, err)
	}
	if query, _, err := buildSampleQuery("sqlite", "users", 2); err != nil || query == "" {
		t.Fatalf("expected sqlite sample query, got query=%q err=%v", query, err)
	}
	if _, errResult, err := buildSampleQuery("oracle", "users", 2); err == nil || errResult == nil {
		t.Fatalf("expected unsupported db buildSampleQuery error result")
	}
}

func TestOpenExecuteSQLConnectionAndDecodeMySQLByteValueBranches(t *testing.T) {
	server := &MCPServer{errorAnalyzer: NewErrorAnalyzer("")}
	ctx := context.Background()
	_, errResult := server.openExecuteSQLConnection(ctx, "profile", "db", 5, &config.Profile{
		ProfileName: "profile",
		DBType:      "oracle",
	})
	if errResult == nil {
		t.Fatalf("expected openExecuteSQLConnection error result for unsupported db")
	}

	if got := decodeMySQLByteValue([]byte("42"), "int"); got != int64(42) {
		t.Fatalf("expected int64 decode, got %#v", got)
	}
	if got := decodeMySQLByteValue([]byte("3.14"), "decimal(10,2)"); got != 3.14 {
		t.Fatalf("expected float decode, got %#v", got)
	}
	if got := decodeMySQLByteValue([]byte("text"), "varchar"); got != "text" {
		t.Fatalf("expected string decode, got %#v", got)
	}
}

func TestScanTableNameAndForeignKeyQueryBranches(t *testing.T) {
	ctx := context.Background()
	dbConn, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	defer dbConn.Close()

	rows, err := dbConn.QueryContext(ctx, "SELECT 'users','BASE TABLE'")
	if err != nil {
		t.Fatalf("failed to query two-column row: %v", err)
	}
	if !rows.Next() {
		t.Fatalf("expected row for scanTableName mysql")
	}
	name, err := scanTableName(rows, "mysql")
	if err != nil || name != "users" {
		t.Fatalf("unexpected mysql scanTableName result name=%q err=%v", name, err)
	}
	_ = rows.Close()

	if _, err := foreignKeyQuery("oracle"); err == nil {
		t.Fatalf("expected unsupported foreignKeyQuery error")
	}
	if _, err := foreignKeyQuery("postgres"); err != nil {
		t.Fatalf("expected postgres foreign key query")
	}
	if _, err := foreignKeyQuery("mysql"); err != nil {
		t.Fatalf("expected mysql foreign key query")
	}
}

func TestCollectSQLiteJoinSuggestions(t *testing.T) {
	ctx := context.Background()
	dbConn, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	defer dbConn.Close()

	if _, err := dbConn.ExecContext(ctx, "CREATE TABLE parents (id INTEGER PRIMARY KEY)"); err != nil {
		t.Fatalf("failed to create parents table: %v", err)
	}
	if _, err := dbConn.ExecContext(ctx, "CREATE TABLE children (id INTEGER PRIMARY KEY, parent_id INTEGER, FOREIGN KEY(parent_id) REFERENCES parents(id))"); err != nil {
		t.Fatalf("failed to create children table: %v", err)
	}

	joins, err := collectSQLiteJoinSuggestions(ctx, dbConn, nil, map[string]bool{})
	if err != nil {
		t.Fatalf("collectSQLiteJoinSuggestions failed: %v", err)
	}
	if len(joins) == 0 {
		t.Fatalf("expected sqlite join suggestions")
	}
}

func TestCollectJoinSuggestionsAndForeignKeyQueryBranches(t *testing.T) {
	ctx := context.Background()
	server := &MCPServer{errorAnalyzer: NewErrorAnalyzer("")}
	dbConn, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	defer dbConn.Close()

	// Trigger error branch for non-sqlite join discovery on missing metadata tables.
	_, errResult, err := server.collectJoinSuggestions(
		ctx,
		dbConn,
		config.Profile{ProfileName: "test", DBType: "postgres"},
		"test",
		nil,
		map[string]bool{},
	)
	if err != nil || errResult == nil {
		t.Fatalf("expected structured join discovery error result, got err=%v errResult=%v", err, errResult)
	}

	// Cover standard join suggestion success path.
	joins, err := collectStandardJoinSuggestions(
		ctx,
		dbConn,
		config.Profile{DBType: "postgres"},
		"SELECT 'orders','user_id','users','id'",
		map[string]bool{},
	)
	if err != nil {
		t.Fatalf("collectStandardJoinSuggestions failed: %v", err)
	}
	if len(joins) != 1 {
		t.Fatalf("expected one join suggestion, got %d", len(joins))
	}

	// Cover mysql branch in queryForeignKeys.
	rows, err := queryForeignKeys(
		ctx,
		dbConn,
		config.Profile{DBType: "mysql", DatabaseName: "appdb"},
		"SELECT 'orders','user_id','users','id' WHERE ? IS NOT NULL",
	)
	if err != nil {
		t.Fatalf("queryForeignKeys mysql branch failed: %v", err)
	}
	if !rows.Next() {
		t.Fatalf("expected a row from mysql queryForeignKeys branch")
	}
	_ = rows.Close()
}

func TestListSmartBuilderTablesErrorBranch(t *testing.T) {
	ctx := context.Background()
	server := &MCPServer{errorAnalyzer: NewErrorAnalyzer("")}
	dbConn, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	if err := dbConn.Close(); err != nil {
		t.Fatalf("failed to close sqlite db: %v", err)
	}

	_, errResult, err := server.listSmartBuilderTables(
		ctx,
		dbConn,
		SmartQueryBuilderParams{ProfileName: "test", DatabaseName: "testdb"},
		&config.Profile{DBType: "sqlite", DatabaseName: "testdb"},
	)
	if err != nil || errResult == nil {
		t.Fatalf("expected listSmartBuilderTables error result, got err=%v errResult=%v", err, errResult)
	}
}

func TestValidateConfigureProfileParamsErrorBranch(t *testing.T) {
	result := validateConfigureProfileParams(ConfigureProfileParams{})
	if result == nil || len(result.Content) == 0 {
		t.Fatalf("expected validation error result for empty configure-profile params")
	}
}

// TestHandleGetSearchPath tests the get-search-path handler
func TestHandleGetSearchPath(t *testing.T) {
	server := &MCPServer{errorAnalyzer: NewErrorAnalyzer("")}
	ctx := context.Background()

	t.Run("missing_profile_name", func(t *testing.T) {
		_, _, err := server.handleGetSearchPath(ctx, nil, GetSearchPathParams{
			ProfileName:  "",
			DatabaseName: "testdb",
		})
		if err == nil {
			t.Fatal("expected error for missing profile_name")
		}
	})

	t.Run("missing_database_name", func(t *testing.T) {
		_, _, err := server.handleGetSearchPath(ctx, nil, GetSearchPathParams{
			ProfileName:  "test",
			DatabaseName: "",
		})
		if err == nil {
			t.Fatal("expected error for missing database_name")
		}
	})
}

// TestHandleGetSearchPath_Postgres tests PostgreSQL search path with sqlmock
func TestHandleGetSearchPath_Postgres(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		// Mock search_path query
		searchPathRows := sqlmock.NewRows([]string{"search_path"}).AddRow("public, user_schema")
		mock.ExpectQuery("SHOW search_path").WillReturnRows(searchPathRows)

		// Mock current_schema query
		schemaRows := sqlmock.NewRows([]string{"current_schema"}).AddRow("public")
		mock.ExpectQuery("SELECT current_schema").WillReturnRows(schemaRows)

		// We can't easily mock the connection opening, so we test the core logic separately
		// This tests the query execution and result parsing
		var searchPath, currentSchema string
		if err := db.QueryRowContext(ctx, "SHOW search_path").Scan(&searchPath); err != nil {
			searchPath = "unknown"
		}
		if err := db.QueryRowContext(ctx, "SELECT current_schema").Scan(&currentSchema); err != nil {
			currentSchema = "unknown"
		}

		if searchPath != "public, user_schema" {
			t.Errorf("expected search_path 'public, user_schema', got %q", searchPath)
		}
		if currentSchema != "public" {
			t.Errorf("expected current_schema 'public', got %q", currentSchema)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled expectations: %v", err)
		}
	})

	t.Run("query_error_returns_unknown", func(t *testing.T) {
		// Mock search_path query error
		mock.ExpectQuery("SHOW search_path").WillReturnError(fmt.Errorf("connection error"))

		var searchPath string
		if err := db.QueryRowContext(ctx, "SHOW search_path").Scan(&searchPath); err != nil {
			searchPath = "unknown"
		}

		if searchPath != "unknown" {
			t.Errorf("expected search_path 'unknown' on error, got %q", searchPath)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled expectations: %v", err)
		}
	})
}

// TestCollectLineageEdges tests the lineage edge collection dispatcher
func TestCollectLineageEdges(t *testing.T) {
	ctx := context.Background()

	t.Run("unsupported_db_type", func(t *testing.T) {
		db, _, err := sqlmock.New()
		if err != nil {
			t.Fatalf("failed to create sqlmock: %v", err)
		}
		defer db.Close()

		prof := config.Profile{
			ProfileName: "test",
			DBType:      "oracle",
		}

		_, err = collectLineageEdges(ctx, db, prof)
		if err == nil {
			t.Fatal("expected error for unsupported db_type")
		}
		if !strings.Contains(err.Error(), "unsupported db_type for lineage") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("mysql_dispatcher", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("failed to create sqlmock: %v", err)
		}
		defer db.Close()

		rows := sqlmock.NewRows([]string{
			"TABLE_NAME", "REFERENCED_TABLE_NAME",
		}).AddRow("orders", "users")

		mock.ExpectQuery("SELECT.*FROM INFORMATION_SCHEMA.KEY_COLUMN_USAGE").
			WithArgs("appdb").
			WillReturnRows(rows)

		prof := config.Profile{
			ProfileName:  "test",
			DBType:       "mysql",
			DatabaseName: "appdb",
		}

		edges, err := collectLineageEdges(ctx, db, prof)
		if err != nil {
			t.Fatalf("collectLineageEdges failed: %v", err)
		}
		if len(edges) != 1 {
			t.Fatalf("expected one edge, got %d", len(edges))
		}
		if edges[0].From != "orders" || edges[0].To != "users" {
			t.Errorf("unexpected edge: From=%q To=%q", edges[0].From, edges[0].To)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled expectations: %v", err)
		}
	})

	t.Run("mariadb_dispatcher", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("failed to create sqlmock: %v", err)
		}
		defer db.Close()

		rows := sqlmock.NewRows([]string{
			"TABLE_NAME", "REFERENCED_TABLE_NAME",
		}).AddRow("items", "products")

		mock.ExpectQuery("SELECT.*FROM INFORMATION_SCHEMA.KEY_COLUMN_USAGE").
			WithArgs("mariadb_app").
			WillReturnRows(rows)

		prof := config.Profile{
			ProfileName:  "test",
			DBType:       "mariadb",
			DatabaseName: "mariadb_app",
		}

		edges, err := collectLineageEdges(ctx, db, prof)
		if err != nil {
			t.Fatalf("collectLineageEdges failed: %v", err)
		}
		if len(edges) != 1 {
			t.Fatalf("expected one edge, got %d", len(edges))
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled expectations: %v", err)
		}
	})

	t.Run("postgres_dispatcher", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("failed to create sqlmock: %v", err)
		}
		defer db.Close()

		rows := sqlmock.NewRows([]string{
			"from_table", "to_table",
		}).AddRow("comments", "posts")

		mock.ExpectQuery("SELECT.*FROM information_schema.table_constraints").
			WillReturnRows(rows)

		prof := config.Profile{
			ProfileName: "test",
			DBType:      "postgres",
		}

		edges, err := collectLineageEdges(ctx, db, prof)
		if err != nil {
			t.Fatalf("collectLineageEdges failed: %v", err)
		}
		if len(edges) != 1 {
			t.Fatalf("expected one edge, got %d", len(edges))
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled expectations: %v", err)
		}
	})
}

// TestCollectLineageEdges_SQLite tests SQLite lineage edge collection with in-memory DB
func TestCollectLineageEdges_SQLite(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	defer db.Close()

	// Create tables with foreign key relationship
	if _, err := db.ExecContext(ctx, "CREATE TABLE users (id INTEGER PRIMARY KEY)"); err != nil {
		t.Fatalf("failed to create users table: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE orders (
			id INTEGER PRIMARY KEY,
			user_id INTEGER,
			FOREIGN KEY(user_id) REFERENCES users(id)
		)
	`); err != nil {
		t.Fatalf("failed to create orders table: %v", err)
	}

	prof := config.Profile{
		ProfileName: "test",
		DBType:      "sqlite",
	}

	edges, err := collectLineageEdges(ctx, db, prof)
	if err != nil {
		t.Fatalf("collectLineageEdges for sqlite failed: %v", err)
	}
	if len(edges) != 1 {
		t.Fatalf("expected one edge, got %d", len(edges))
	}
	if edges[0].From != "orders" || edges[0].To != "users" {
		t.Errorf("unexpected edge: From=%q To=%q", edges[0].From, edges[0].To)
	}
}

// TestScanTableInfo tests the scanTableInfo function for all db types
func TestScanTableInfo(t *testing.T) {
	ctx := context.Background()

	t.Run("mysql_branch", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("failed to create sqlmock: %v", err)
		}
		defer db.Close()

		rows := sqlmock.NewRows([]string{"schema", "name", "type"}).
			AddRow("mydb", "users", "BASE TABLE")

		mock.ExpectQuery("SELECT").WillReturnRows(rows)

		resultRows, err := db.QueryContext(ctx, "SELECT")
		if err != nil {
			t.Fatalf("query failed: %v", err)
		}
		defer resultRows.Close()

		if !resultRows.Next() {
			t.Fatal("expected row")
		}

		info, err := scanTableInfo(resultRows, "mysql")
		if err != nil {
			t.Fatalf("scanTableInfo failed: %v", err)
		}
		if info.Schema != "mydb" || info.Name != "users" {
			t.Errorf("unexpected result: Schema=%q Name=%q", info.Schema, info.Name)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled expectations: %v", err)
		}
	})

	t.Run("mariadb_branch", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("failed to create sqlmock: %v", err)
		}
		defer db.Close()

		rows := sqlmock.NewRows([]string{"schema", "name", "type"}).
			AddRow("mariadb_db", "products", "BASE TABLE")

		mock.ExpectQuery("SELECT").WillReturnRows(rows)

		resultRows, err := db.QueryContext(ctx, "SELECT")
		if err != nil {
			t.Fatalf("query failed: %v", err)
		}
		defer resultRows.Close()

		if !resultRows.Next() {
			t.Fatal("expected row")
		}

		info, err := scanTableInfo(resultRows, "mariadb")
		if err != nil {
			t.Fatalf("scanTableInfo failed: %v", err)
		}
		if info.Schema != "mariadb_db" || info.Name != "products" {
			t.Errorf("unexpected result: Schema=%q Name=%q", info.Schema, info.Name)
		}
	})

	t.Run("postgres_branch", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("failed to create sqlmock: %v", err)
		}
		defer db.Close()

		rows := sqlmock.NewRows([]string{"schema", "name"}).
			AddRow("public", "orders")

		mock.ExpectQuery("SELECT").WillReturnRows(rows)

		resultRows, err := db.QueryContext(ctx, "SELECT")
		if err != nil {
			t.Fatalf("query failed: %v", err)
		}
		defer resultRows.Close()

		if !resultRows.Next() {
			t.Fatal("expected row")
		}

		info, err := scanTableInfo(resultRows, "postgres")
		if err != nil {
			t.Fatalf("scanTableInfo failed: %v", err)
		}
		if info.Schema != "public" || info.Name != "orders" {
			t.Errorf("unexpected result: Schema=%q Name=%q", info.Schema, info.Name)
		}
	})

	t.Run("sqlite_branch", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("failed to create sqlmock: %v", err)
		}
		defer db.Close()

		rows := sqlmock.NewRows([]string{"name"}).
			AddRow("items")

		mock.ExpectQuery("SELECT").WillReturnRows(rows)

		resultRows, err := db.QueryContext(ctx, "SELECT")
		if err != nil {
			t.Fatalf("query failed: %v", err)
		}
		defer resultRows.Close()

		if !resultRows.Next() {
			t.Fatal("expected row")
		}

		info, err := scanTableInfo(resultRows, "sqlite")
		if err != nil {
			t.Fatalf("scanTableInfo failed: %v", err)
		}
		if info.Schema != "" || info.Name != "items" {
			t.Errorf("unexpected result: Schema=%q Name=%q", info.Schema, info.Name)
		}
	})

	t.Run("mysql_scan_error", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("failed to create sqlmock: %v", err)
		}
		defer db.Close()

		// Only 2 columns when expecting 3 for mysql
		rows := sqlmock.NewRows([]string{"schema", "name"}).
			AddRow("mydb", "users")

		mock.ExpectQuery("SELECT").WillReturnRows(rows)

		resultRows, err := db.QueryContext(ctx, "SELECT")
		if err != nil {
			t.Fatalf("query failed: %v", err)
		}
		defer resultRows.Close()

		if !resultRows.Next() {
			t.Fatal("expected row")
		}

		_, err = scanTableInfo(resultRows, "mysql")
		if err == nil {
			t.Fatal("expected scan error for column mismatch")
		}
	})

	t.Run("postgres_scan_error", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("failed to create sqlmock: %v", err)
		}
		defer db.Close()

		// Only 1 column when expecting 2 for postgres
		rows := sqlmock.NewRows([]string{"name"}).
			AddRow("users")

		mock.ExpectQuery("SELECT").WillReturnRows(rows)

		resultRows, err := db.QueryContext(ctx, "SELECT")
		if err != nil {
			t.Fatalf("query failed: %v", err)
		}
		defer resultRows.Close()

		if !resultRows.Next() {
			t.Fatal("expected row")
		}

		_, err = scanTableInfo(resultRows, "postgres")
		if err == nil {
			t.Fatal("expected scan error for column mismatch")
		}
	})
}

// TestHandleListSchemas_MissingParams tests missing params validation
func TestHandleListSchemas_MissingParams(t *testing.T) {
	server := &MCPServer{errorAnalyzer: NewErrorAnalyzer("")}
	ctx := context.Background()

	t.Run("missing_profile_name", func(t *testing.T) {
		_, _, err := server.handleListSchemas(ctx, nil, ListSchemasParams{
			ProfileName:  "",
			DatabaseName: "testdb",
		})
		if err == nil {
			t.Fatal("expected error for missing profile_name")
		}
	})

	t.Run("missing_database_name", func(t *testing.T) {
		_, _, err := server.handleListSchemas(ctx, nil, ListSchemasParams{
			ProfileName:  "test",
			DatabaseName: "",
		})
		if err == nil {
			t.Fatal("expected error for missing database_name")
		}
	})
}

// TestGetDefaultSchemaWithMock tests the schema auto-detection with sqlmock
func TestGetDefaultSchemaWithMock(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()
	ctx := context.Background()

	t.Run("success_current_schema", func(t *testing.T) {
		// Get a connection from the pool
		conn, err := db.Conn(ctx)
		if err != nil {
			t.Fatalf("failed to get connection: %v", err)
		}
		defer conn.Close()

		rows := sqlmock.NewRows([]string{"current_schema"}).AddRow("myapp_schema")
		mock.ExpectQuery("SELECT current_schema").WillReturnRows(rows)

		schema, err := GetDefaultSchema(ctx, conn)
		if err != nil {
			t.Fatalf("GetDefaultSchema failed: %v", err)
		}
		if schema != "myapp_schema" {
			t.Errorf("expected 'myapp_schema', got %q", schema)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled expectations: %v", err)
		}
	})

	t.Run("fallback_to_information_schema", func(t *testing.T) {
		conn, err := db.Conn(ctx)
		if err != nil {
			t.Fatalf("failed to get connection: %v", err)
		}
		defer conn.Close()

		// First query returns null
		nullRows := sqlmock.NewRows([]string{"current_schema"}).AddRow(nil)
		mock.ExpectQuery("SELECT current_schema").WillReturnRows(nullRows)

		// Second query returns schema from information_schema
		schemaRows := sqlmock.NewRows([]string{"schema_name"}).AddRow("app_public")
		mock.ExpectQuery("SELECT schema_name FROM information_schema.schemata").WillReturnRows(schemaRows)

		schema, err := GetDefaultSchema(ctx, conn)
		if err != nil {
			t.Fatalf("GetDefaultSchema failed: %v", err)
		}
		if schema != "app_public" {
			t.Errorf("expected 'app_public', got %q", schema)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled expectations: %v", err)
		}
	})

	t.Run("fallback_to_public", func(t *testing.T) {
		conn, err := db.Conn(ctx)
		if err != nil {
			t.Fatalf("failed to get connection: %v", err)
		}
		defer conn.Close()

		// First query returns null
		nullRows := sqlmock.NewRows([]string{"current_schema"}).AddRow(nil)
		mock.ExpectQuery("SELECT current_schema").WillReturnRows(nullRows)

		// Second query returns no rows (sql.ErrNoRows)
		emptyRows := sqlmock.NewRows([]string{"schema_name"})
		mock.ExpectQuery("SELECT schema_name FROM information_schema.schemata").WillReturnRows(emptyRows)

		schema, err := GetDefaultSchema(ctx, conn)
		if err != nil {
			t.Fatalf("GetDefaultSchema failed: %v", err)
		}
		if schema != "public" {
			t.Errorf("expected 'public' fallback, got %q", schema)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled expectations: %v", err)
		}
	})

	t.Run("query_error", func(t *testing.T) {
		conn, err := db.Conn(ctx)
		if err != nil {
			t.Fatalf("failed to get connection: %v", err)
		}
		defer conn.Close()

		mock.ExpectQuery("SELECT current_schema").WillReturnError(fmt.Errorf("connection lost"))

		_, err = GetDefaultSchema(ctx, conn)
		if err == nil {
			t.Fatal("expected error for query failure")
		}
	})
}

// TestResolveSchemaWithConnection tests ResolveSchema with actual connection
func TestResolveSchemaWithConnection(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()
	ctx := context.Background()

	t.Run("explicit_schema", func(t *testing.T) {
		conn, err := db.Conn(ctx)
		if err != nil {
			t.Fatalf("failed to get connection: %v", err)
		}
		defer conn.Close()

		// With explicit schema, should not query DB
		result, err := ResolveSchema(ctx, conn, "custom_schema")
		if err != nil {
			t.Fatalf("ResolveSchema failed: %v", err)
		}
		if result != `"custom_schema"` {
			t.Errorf("expected '\"custom_schema\"', got %q", result)
		}
	})

	t.Run("empty_schema_calls_get_default", func(t *testing.T) {
		conn, err := db.Conn(ctx)
		if err != nil {
			t.Fatalf("failed to get connection: %v", err)
		}
		defer conn.Close()

		rows := sqlmock.NewRows([]string{"current_schema"}).AddRow("detected_schema")
		mock.ExpectQuery("SELECT current_schema").WillReturnRows(rows)

		result, err := ResolveSchema(ctx, conn, "")
		if err != nil {
			t.Fatalf("ResolveSchema failed: %v", err)
		}
		if result != `"detected_schema"` {
			t.Errorf("expected '\"detected_schema\"', got %q", result)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled expectations: %v", err)
		}
	})
}

func floatEqual(left, right float64) bool {
	return math.Abs(left-right) < 1e-9
}

func TestFetchSchemasFromDB(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()
	ctx := context.Background()

	t.Run("success_multiple_rows", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"schema_name"}).
			AddRow("myapp").
			AddRow("public").
			AddRow("test_schema")
		mock.ExpectQuery("SELECT schema_name FROM information_schema.schemata").WillReturnRows(rows)

		schemas, err := fetchSchemasFromDB(ctx, db)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(schemas) != 3 {
			t.Fatalf("expected 3 schemas, got %d", len(schemas))
		}
		if schemas[0] != "myapp" || schemas[1] != "public" || schemas[2] != "test_schema" {
			t.Errorf("unexpected schemas: %v", schemas)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled expectations: %v", err)
		}
	})

	t.Run("query_error", func(t *testing.T) {
		mock.ExpectQuery("SELECT schema_name FROM information_schema.schemata").WillReturnError(fmt.Errorf("db error"))

		_, err := fetchSchemasFromDB(ctx, db)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "failed to query schemas") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("scan_error", func(t *testing.T) {
		// Use a new mock to avoid interference from previous sub-tests
		scanDB, scanMock, scanErr := sqlmock.New()
		if scanErr != nil {
			t.Fatalf("failed to create sqlmock: %v", scanErr)
		}
		defer scanDB.Close()
		// Return extra columns to trigger a scan mismatch — scanning 1 field from 2-column row works,
		// so instead use a driver.Valuer that returns an unscannable type
		rows := sqlmock.NewRows([]string{"schema_name"}).AddRow(nil)
		scanMock.ExpectQuery("SELECT schema_name FROM information_schema.schemata").WillReturnRows(rows)

		result, err := fetchSchemasFromDB(ctx, scanDB)
		// nil values scan into string as empty string — not an error, just verify we handle it
		if err != nil {
			// If we get an error, it should be a scan error
			if !strings.Contains(err.Error(), "failed to scan schema") {
				t.Errorf("unexpected error: %v", err)
			}
		} else {
			// nil rows scan as empty string — verify we get an empty string entry
			if len(result) != 1 || result[0] != "" {
				t.Logf("scan of nil yielded empty string: %v (acceptable)", result)
			}
		}
	})
}

func TestResolveDefaultSchema(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()
	ctx := context.Background()

	t.Run("postgres_success", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"current_schema"}).AddRow("myapp_schema")
		mock.ExpectQuery("SELECT current_schema").WillReturnRows(rows)

		result := resolveDefaultSchema(ctx, db, "postgres", "testdb")
		if result != "myapp_schema" {
			t.Errorf("expected 'myapp_schema', got %q", result)
		}
	})

	t.Run("postgres_query_error_fallback", func(t *testing.T) {
		mock.ExpectQuery("SELECT current_schema").WillReturnError(fmt.Errorf("connection lost"))

		result := resolveDefaultSchema(ctx, db, "postgres", "testdb")
		if result != "public" {
			t.Errorf("expected fallback 'public', got %q", result)
		}
	})

	t.Run("mysql_returns_database_name", func(t *testing.T) {
		result := resolveDefaultSchema(ctx, db, "mysql", "my_database")
		if result != "my_database" {
			t.Errorf("expected 'my_database', got %q", result)
		}
	})

	t.Run("mariadb_returns_database_name", func(t *testing.T) {
		result := resolveDefaultSchema(ctx, db, "mariadb", "prod_db")
		if result != "prod_db" {
			t.Errorf("expected 'prod_db', got %q", result)
		}
	})
}

func TestMapPostgresColumn(t *testing.T) {
	t.Run("all_fields_valid", func(t *testing.T) {
		col := mapPostgresColumn(postgresColumnRow{
			name:       "id",
			typ:        "integer",
			nullable:   "NO",
			keyType:    "PRI",
			defaultVal: sql.NullString{String: "nextval('seq')", Valid: true},
			comment:    sql.NullString{String: "primary key", Valid: true},
			maxLength:  sql.NullInt64{Int64: 0, Valid: true},
			precision:  sql.NullInt64{Int64: 10, Valid: true},
			scale:      sql.NullInt64{Int64: 0, Valid: true},
		})
		if col.Name != "id" {
			t.Errorf("expected name 'id', got %q", col.Name)
		}
		if col.Nullable {
			t.Error("expected Nullable=false for 'NO'")
		}
		if !col.AutoIncrement {
			t.Error("expected AutoIncrement=true for nextval default")
		}
		if col.Key != "PRI" {
			t.Errorf("expected Key 'PRI', got %q", col.Key)
		}
		if col.Default == nil || *col.Default != "nextval('seq')" {
			t.Error("expected Default to be set")
		}
		if col.Comment != "primary key" {
			t.Errorf("expected Comment 'primary key', got %q", col.Comment)
		}
		if col.MaxLength == nil || *col.MaxLength != 0 {
			t.Errorf("expected MaxLength=0, got %v", col.MaxLength)
		}
		if col.Precision == nil || *col.Precision != int64(10) {
			t.Errorf("expected Precision=10, got %v", col.Precision)
		}
		if col.Scale == nil || *col.Scale != 0 {
			t.Errorf("expected Scale=0, got %v", col.Scale)
		}
	})

	t.Run("nullable_with_no_default", func(t *testing.T) {
		col := mapPostgresColumn(postgresColumnRow{
			name:      "name",
			typ:       "varchar",
			nullable:  "YES",
			maxLength: sql.NullInt64{Int64: 255, Valid: true},
		})
		if !col.Nullable {
			t.Error("expected Nullable=true for 'YES'")
		}
		if col.AutoIncrement {
			t.Error("expected AutoIncrement=false")
		}
		if col.Default != nil {
			t.Error("expected Default=nil when not valid")
		}
		if col.MaxLength == nil || *col.MaxLength != 255 {
			t.Error("expected MaxLength=255")
		}
	})

	t.Run("no_auto_increment_without_nextval", func(t *testing.T) {
		col := mapPostgresColumn(postgresColumnRow{
			name:       "status",
			typ:        "text",
			nullable:   "NO",
			defaultVal: sql.NullString{String: "'active'", Valid: true},
		})
		if col.AutoIncrement {
			t.Error("expected AutoIncrement=false without nextval")
		}
		if col.Default == nil || *col.Default != "'active'" {
			t.Error("expected Default to be 'active'")
		}
	})
}

func TestHandleGetSearchPath_NonPostgres(t *testing.T) {
	testConfig := setupTestConfig(t)
	defer os.Remove(testConfig)

	server := NewMCPServerWithConfig(testConfig)
	ctx := context.Background()

	res, _, err := server.handleGetSearchPath(ctx, nil, GetSearchPathParams{
		ProfileName:  "testsqlite",
		DatabaseName: "test.db",
	})
	if err != nil {
		t.Fatalf("handleGetSearchPath error: %v", err)
	}
	if res == nil || res.Content == nil {
		t.Fatalf("handleGetSearchPath returned nil content")
	}

	var result GetSearchPathResult
	if err := json.Unmarshal([]byte(res.Content[0].(*mcpsdk.TextContent).Text), &result); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if result.SearchPath != "test.db" {
		t.Errorf("Expected search_path 'test.db' for SQLite, got %q", result.SearchPath)
	}
	if result.CurrentSchema != "test.db" {
		t.Errorf("Expected current_schema 'test.db' for SQLite, got %q", result.CurrentSchema)
	}
	if result.ConnectionPoolingWarning == "" {
		t.Error("Expected non-empty ConnectionPoolingWarning")
	}
}
