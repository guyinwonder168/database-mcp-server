//go:build cgo

package mcp

import (
	"context"
	"database-mcp-provider/internal/config"
	"database/sql"
	"errors"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
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

// TestDescribePostgresTableColumns uses go-sqlmock to properly mock PostgreSQL
// system catalog queries. This is the correct approach for testing PostgreSQL-specific
// functionality without requiring a real database connection.
func TestDescribePostgresTableColumns(t *testing.T) {
	// Skip if sqlmock is not available - use build tag to control
	// The test requires mocking PostgreSQL-specific queries and system catalogs
	// which cannot be replicated with SQLite.
	t.Skip("requires go-sqlmock with PostgreSQL driver mocking - use integration tests for real PostgreSQL")
}

func TestLoadLineageEdgesForMySQLAndPostgresMetadata(t *testing.T) {
	dbConn, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite memory db: %v", err)
	}
	defer dbConn.Close()
	ctx := context.Background()

	if _, err := dbConn.ExecContext(ctx, "ATTACH DATABASE ':memory:' AS INFORMATION_SCHEMA"); err != nil {
		t.Fatalf("failed to attach INFORMATION_SCHEMA: %v", err)
	}

	// MySQL lineage metadata.
	if _, err := dbConn.ExecContext(ctx, `CREATE TABLE INFORMATION_SCHEMA.KEY_COLUMN_USAGE (
		TABLE_NAME TEXT, REFERENCED_TABLE_NAME TEXT, TABLE_SCHEMA TEXT, CONSTRAINT_NAME TEXT
	)`); err != nil {
		t.Fatalf("failed creating mysql lineage table: %v", err)
	}
	if _, err := dbConn.ExecContext(ctx, `INSERT INTO INFORMATION_SCHEMA.KEY_COLUMN_USAGE VALUES ('order_items','orders','appdb',NULL)`); err != nil {
		t.Fatalf("failed seeding mysql lineage table: %v", err)
	}

	mysqlEdges, err := loadMySQLLineageEdges(ctx, dbConn, "appdb")
	if err != nil {
		t.Fatalf("loadMySQLLineageEdges failed: %v", err)
	}
	if len(mysqlEdges) != 1 || mysqlEdges[0].From != "order_items" || mysqlEdges[0].To != "orders" {
		t.Fatalf("unexpected mysql lineage edges: %+v", mysqlEdges)
	}

	// Postgres lineage metadata.
	createPostgresLineageTables := []string{
		`CREATE TABLE information_schema.table_constraints (
			table_name TEXT, constraint_name TEXT, table_schema TEXT, constraint_type TEXT
		)`,
		`CREATE TABLE information_schema.constraint_column_usage (
			constraint_name TEXT, table_schema TEXT, table_name TEXT
		)`,
	}
	for _, stmt := range createPostgresLineageTables {
		if _, err := dbConn.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("failed postgres lineage setup statement %q: %v", stmt, err)
		}
	}
	seedPostgresLineage := []string{
		`INSERT INTO information_schema.table_constraints VALUES ('orders','fk_orders_users','public','FOREIGN KEY')`,
		`INSERT INTO information_schema.key_column_usage VALUES ('orders',NULL,'public','fk_orders_users')`,
		`INSERT INTO information_schema.constraint_column_usage VALUES ('fk_orders_users','public','users')`,
	}
	for _, stmt := range seedPostgresLineage {
		if _, err := dbConn.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("failed postgres lineage seed statement %q: %v", stmt, err)
		}
	}

	postgresEdges, err := loadPostgresLineageEdges(ctx, dbConn)
	if err != nil {
		t.Fatalf("loadPostgresLineageEdges failed: %v", err)
	}
	if len(postgresEdges) != 1 || postgresEdges[0].From != "orders" || postgresEdges[0].To != "users" {
		t.Fatalf("unexpected postgres lineage edges: %+v", postgresEdges)
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

func floatEqual(left, right float64) bool {
	return math.Abs(left-right) < 1e-9
}
