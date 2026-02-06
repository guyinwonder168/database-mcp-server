package mcp

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"os"
	"strings"
	"testing"
	"time"
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
	result := buildAnalyzeSchemaResult(
		startTime,
		AnalyzeSchemaParams{AnalysisLevel: AnalysisLevelDetailed},
		"sqlite",
		[]string{"users"},
		3,
		TableCatalog{},
		map[string]TableInfo{"users": {ColumnCount: 3}},
		RelationshipGraph{},
		map[string]interface{}{"nodes": []string{}},
		AIQuerySuggestions{},
		map[string]QualityMetrics{"users.id": {OverallScore: 0.9}},
		"finance",
		0.8,
	)

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

func TestDescribeMySQLTableColumnsWithAttachedSQLiteSchema(t *testing.T) {
	dbConn, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite memory db: %v", err)
	}
	defer dbConn.Close()

	ctx := context.Background()
	if _, err := dbConn.ExecContext(ctx, "ATTACH DATABASE ':memory:' AS INFORMATION_SCHEMA"); err != nil {
		t.Fatalf("failed to attach INFORMATION_SCHEMA db: %v", err)
	}

	createSQL := `CREATE TABLE INFORMATION_SCHEMA.COLUMNS (
		COLUMN_NAME TEXT,
		COLUMN_TYPE TEXT,
		IS_NULLABLE TEXT,
		COLUMN_KEY TEXT,
		COLUMN_DEFAULT TEXT,
		COLUMN_COMMENT TEXT,
		EXTRA TEXT,
		CHARACTER_SET_NAME TEXT,
		COLLATION_NAME TEXT,
		CHARACTER_MAXIMUM_LENGTH INTEGER,
		NUMERIC_PRECISION INTEGER,
		NUMERIC_SCALE INTEGER,
		TABLE_SCHEMA TEXT,
		TABLE_NAME TEXT,
		ORDINAL_POSITION INTEGER
	)`
	if _, err := dbConn.ExecContext(ctx, createSQL); err != nil {
		t.Fatalf("failed to create INFORMATION_SCHEMA.COLUMNS: %v", err)
	}

	insertSQL := `INSERT INTO INFORMATION_SCHEMA.COLUMNS (
		COLUMN_NAME, COLUMN_TYPE, IS_NULLABLE, COLUMN_KEY, COLUMN_DEFAULT, COLUMN_COMMENT, EXTRA,
		CHARACTER_SET_NAME, COLLATION_NAME, CHARACTER_MAXIMUM_LENGTH, NUMERIC_PRECISION, NUMERIC_SCALE,
		TABLE_SCHEMA, TABLE_NAME, ORDINAL_POSITION
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err = dbConn.ExecContext(
		ctx,
		insertSQL,
		"id", "bigint", "NO", "PRI", "0", "identifier", "auto_increment",
		"utf8mb4", "utf8mb4_general_ci", 255, 20, 0,
		"appdb", "users", 1,
	)
	if err != nil {
		t.Fatalf("failed to insert INFORMATION_SCHEMA row: %v", err)
	}

	columns, err := describeMySQLTableColumns(ctx, dbConn, "appdb", "users")
	if err != nil {
		t.Fatalf("describeMySQLTableColumns failed: %v", err)
	}
	if len(columns) != 1 {
		t.Fatalf("expected one column, got %d", len(columns))
	}
	if columns[0].Name != "id" || !columns[0].AutoIncrement {
		t.Fatalf("unexpected described column: %+v", columns[0])
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

func floatEqual(left, right float64) bool {
	return math.Abs(left-right) < 1e-9
}
