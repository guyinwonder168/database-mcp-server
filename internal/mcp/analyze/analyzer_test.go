package analyze

import (
	"context"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestBuildTableSchemas(t *testing.T) {
	tableColumns := map[string][]SchemaColumnInfo{
		"users": {
			{ColumnName: "id", DataType: "int", IsPrimaryKey: true},
			{ColumnName: "name", DataType: "varchar", IsNullable: true},
			{ColumnName: "email", DataType: "varchar", Unique: true},
			{ColumnName: "role_id", DataType: "int", IsForeignKey: true},
			{ColumnName: "created_at", DataType: "timestamp", Indexed: true},
		},
	}

	schemas := buildTableSchemas(tableColumns)

	if len(schemas) != 1 {
		t.Fatalf("expected 1 schema, got %d", len(schemas))
	}

	users := schemas["users"]
	if users.ColumnCount != 5 {
		t.Errorf("expected 5 columns, got %d", users.ColumnCount)
	}
	if users.KeyColumns.PrimaryKey != "id" {
		t.Errorf("expected primary key 'id', got %q", users.KeyColumns.PrimaryKey)
	}
	if len(users.KeyColumns.ForeignKeys) != 1 || users.KeyColumns.ForeignKeys[0] != "role_id" {
		t.Errorf("expected foreign key 'role_id', got %v", users.KeyColumns.ForeignKeys)
	}
	if len(users.KeyColumns.UniqueColumns) != 1 || users.KeyColumns.UniqueColumns[0] != "email" {
		t.Errorf("expected unique column 'email', got %v", users.KeyColumns.UniqueColumns)
	}
	if len(users.KeyColumns.IndexedColumns) != 1 || users.KeyColumns.IndexedColumns[0] != "created_at" {
		t.Errorf("expected indexed column 'created_at', got %v", users.KeyColumns.IndexedColumns)
	}
}

func TestBuildTableSchemas_Empty(t *testing.T) {
	schemas := buildTableSchemas(map[string][]SchemaColumnInfo{})
	if len(schemas) != 0 {
		t.Errorf("expected 0 schemas, got %d", len(schemas))
	}
}

func TestApplyIndexesToSchemas(t *testing.T) {
	schemas := map[string]TableInfo{
		"users": {
			Columns: []SchemaColumnInfo{{ColumnName: "id"}, {ColumnName: "email"}},
		},
	}
	indexes := []IndexInfo{
		{TableName: "users", IndexName: "idx_email", Columns: []string{"email"}, IsUnique: true},
		{TableName: "users", IndexName: "PRIMARY", Columns: []string{"id"}, IsPrimary: true},
	}

	applyIndexesToSchemas(schemas, indexes)

	idxCols := schemas["users"].KeyColumns.IndexedColumns
	if len(idxCols) != 1 || idxCols[0] != "email" {
		t.Errorf("expected indexed column 'email', got %v", idxCols)
	}
}

func TestApplyIndexesToSchemas_SkipsExisting(t *testing.T) {
	schemas := map[string]TableInfo{
		"users": {
			KeyColumns: KeyColumns{IndexedColumns: []string{"email"}},
		},
	}
	indexes := []IndexInfo{
		{TableName: "users", IndexName: "idx_email", Columns: []string{"email"}, IsUnique: true},
	}

	applyIndexesToSchemas(schemas, indexes)

	// Should not duplicate
	idxCols := schemas["users"].KeyColumns.IndexedColumns
	if len(idxCols) != 1 {
		t.Errorf("expected 1 indexed column, got %d: %v", len(idxCols), idxCols)
	}
}

func TestBuildRelationshipGraph(t *testing.T) {
	fks := []ForeignKeyRelationship{
		{FromTable: "orders", FromColumn: "user_id", ToTable: "users", ToColumn: "id"},
	}
	implicitRels := []SemanticRelationship{
		{Tables: []string{"orders", "products"}, RelationshipType: "many_to_one", ConnectionBasis: "naming_convention", ConfidenceScore: 0.7, FromColumn: "product_id", ToColumn: "id"},
	}

	graph := buildRelationshipGraph(fks, implicitRels)

	if len(graph.ForeignKeys) != 1 {
		t.Fatalf("expected 1 FK, got %d", len(graph.ForeignKeys))
	}
	if graph.ForeignKeys[0].SuggestedJoin == "" {
		t.Error("expected suggested join for FK")
	}

	if len(graph.SemanticRelationships) != 1 {
		t.Fatalf("expected 1 semantic rel, got %d", len(graph.SemanticRelationships))
	}
	if graph.SemanticRelationships[0].SuggestedJoin == "" {
		t.Error("expected suggested join for semantic relationship")
	}
}

func TestBuildRelationshipGraph_Empty(t *testing.T) {
	graph := buildRelationshipGraph(nil, nil)
	if len(graph.ForeignKeys) != 0 || len(graph.SemanticRelationships) != 0 {
		t.Error("expected empty graph")
	}
}

func TestApplyRowCountToCatalog(t *testing.T) {
	catalog := TableCatalog{
		CoreEntities: []TableEntity{{TableName: "users"}, {TableName: "orders"}},
		LookupTables: []TableEntity{{TableName: "roles"}},
	}
	rowCounts := map[string]int64{
		"users":  50000,
		"orders": 120000,
		"roles":  10,
	}

	applyRowCountToCatalog(&catalog, rowCounts)

	if catalog.CoreEntities[0].EstimatedRows != "~50k" {
		t.Errorf("expected '~50k', got %q", catalog.CoreEntities[0].EstimatedRows)
	}
	if catalog.CoreEntities[1].EstimatedRows != "~120k" {
		t.Errorf("expected '~120k', got %q", catalog.CoreEntities[1].EstimatedRows)
	}
	if catalog.LookupTables[0].EstimatedRows != "10" {
		t.Errorf("expected '10', got %q", catalog.LookupTables[0].EstimatedRows)
	}
}

func TestFormatRowCount(t *testing.T) {
	tests := []struct {
		input int64
		want  string
	}{
		{0, "0"},
		{5, "5"},
		{999, "999"},
		{1000, "~1k"},
		{50000, "~50k"},
		{1000000, "~1M"},
		{5500000, "~5M"},
	}
	for _, tt := range tests {
		got := formatRowCount(tt.input)
		if got != tt.want {
			t.Errorf("formatRowCount(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSummarizeBusinessContext_Nil(t *testing.T) {
	domain, confidence, desc := summarizeBusinessContext(nil)
	if domain != "" || confidence != 0 || desc != "" {
		t.Errorf("expected empty result for nil context, got domain=%q confidence=%f desc=%q", domain, confidence, desc)
	}
}

func TestSummarizeBusinessContext_WithDomain(t *testing.T) {
	ctx := &BusinessContext{
		DomainIndicators: map[string]float64{
			"e-commerce": 0.85,
			"healthcare": 0.3,
		},
		EntityRelationships: EntityRelationships{
			CentralEntities: []string{"products", "orders"},
		},
	}

	domain, confidence, desc := summarizeBusinessContext(ctx)
	if domain != "e-commerce" {
		t.Errorf("expected domain 'e-commerce', got %q", domain)
	}
	if confidence != 0.85 {
		t.Errorf("expected confidence 0.85, got %f", confidence)
	}
	if desc == "" {
		t.Error("expected non-empty description")
	}
}

func TestBuildClassificationSignals(t *testing.T) {
	tableNames := []string{"users", "orders"}
	tableColumns := map[string][]SchemaColumnInfo{
		"users": {
			{ColumnName: "id"},
			{ColumnName: "caller_id"},
			{ColumnName: "duration"},
		},
		"orders": {
			{ColumnName: "id"},
			{ColumnName: "product_id"},
		},
	}
	fks := []ForeignKeyRelationship{
		{FromTable: "orders", FromColumn: "user_id", ToTable: "users", ToColumn: "id"},
	}

	signals := buildClassificationSignals(tableNames, tableColumns, fks)

	if signals.TotalTables != 2 {
		t.Errorf("expected 2 tables, got %d", signals.TotalTables)
	}
	if signals.TotalColumns != 5 {
		t.Errorf("expected 5 columns, got %d", signals.TotalColumns)
	}
	if len(signals.FKSummary) == 0 {
		t.Error("expected non-empty FK summary")
	}
}

func TestBuildDatabaseOverview(t *testing.T) {
	tableNames := []string{"users", "orders"}
	tableSchemas := map[string]TableInfo{
		"users":  {ColumnCount: 5},
		"orders": {ColumnCount: 8},
	}
	relGraph := RelationshipGraph{
		ForeignKeys:           []ForeignKeyRelationship{{}},
		SemanticRelationships: []SemanticRelationship{{}, {}},
	}

	overview := buildDatabaseOverview(tableNames, tableSchemas, relGraph, "e-commerce", 0.85, "This is an e-commerce database")

	if overview.TotalTables != 2 {
		t.Errorf("expected 2 tables, got %d", overview.TotalTables)
	}
	if overview.TotalColumns != 13 {
		t.Errorf("expected 13 columns, got %d", overview.TotalColumns)
	}
	if overview.TotalRelationships != 3 {
		t.Errorf("expected 3 relationships, got %d", overview.TotalRelationships)
	}
}

func TestRun_BasicLevel(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	// MySQL bulk columns
	colRows := sqlmock.NewRows([]string{"TABLE_NAME", "COLUMN_NAME", "DATA_TYPE", "IS_NULLABLE", "COLUMN_DEFAULT", "COLUMN_KEY", "EXTRA"}).
		AddRow("users", "id", "int", "NO", nil, "PRI", "auto_increment").
		AddRow("users", "name", "varchar", "YES", nil, "", "")

	mock.ExpectQuery(`SELECT TABLE_NAME, COLUMN_NAME.*FROM information_schema\.COLUMNS`).
		WithArgs("mydb").
		WillReturnRows(colRows)

	// Row counts
	rcRows := sqlmock.NewRows([]string{"TABLE_NAME", "TABLE_ROWS"}).
		AddRow("users", int64(100))
	mock.ExpectQuery(`SELECT TABLE_NAME, TABLE_ROWS FROM information_schema\.TABLES`).
		WithArgs("mydb").
		WillReturnRows(rcRows)

	// Indexes
	idxRows := sqlmock.NewRows([]string{"TABLE_NAME", "INDEX_NAME", "COLUMN_NAME", "NON_UNIQUE", "SEQ_IN_INDEX"}).
		AddRow("users", "PRIMARY", "id", 0, 1)
	mock.ExpectQuery(`SELECT TABLE_NAME, INDEX_NAME, COLUMN_NAME, NON_UNIQUE, SEQ_IN_INDEX FROM information_schema\.STATISTICS`).
		WithArgs("mydb").
		WillReturnRows(idxRows)

	// FKs
	fkRows := sqlmock.NewRows([]string{"TABLE_NAME", "COLUMN_NAME", "REFERENCED_TABLE_NAME", "REFERENCED_COLUMN_NAME"})
	mock.ExpectQuery(`SELECT TABLE_NAME, COLUMN_NAME, REFERENCED_TABLE_NAME, REFERENCED_COLUMN_NAME FROM information_schema\.KEY_COLUMN_USAGE`).
		WithArgs("mydb").
		WillReturnRows(fkRows)

	params := AnalyzeSchemaParams{
		ProfileName:   "test",
		AnalysisLevel: AnalysisLevelBasic,
	}

	result, err := Run(context.Background(), db, "mysql", "mydb", params, []string{"users"})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.AnalysisMetadata.AnalysisLevel != AnalysisLevelBasic {
		t.Errorf("expected analysis level 'basic', got %q", result.AnalysisMetadata.AnalysisLevel)
	}
	if result.DatabaseOverview.TotalTables != 1 {
		t.Errorf("expected 1 table, got %d", result.DatabaseOverview.TotalTables)
	}
	if len(result.TableSchemas) != 1 {
		t.Errorf("expected 1 schema, got %d", len(result.TableSchemas))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestRun_UnsupportedDB(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	params := AnalyzeSchemaParams{
		ProfileName:   "test",
		AnalysisLevel: AnalysisLevelBasic,
	}

	_, err = Run(context.Background(), db, "oracle", "mydb", params, []string{"users"})
	if err == nil {
		t.Fatal("expected error for unsupported db type")
	}
}
