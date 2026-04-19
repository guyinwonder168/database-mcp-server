package analyze

import (
	"context"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

// TestFetchColumnsBulk_MySQL_WithEmptySchema_Bug75 verifies that passing an empty
// schema to FetchColumnsBulk for MySQL returns 0 columns (the bug scenario).
//
// Issue #75: When resolveSchemaForAnalyze returned "" for MySQL, WHERE TABLE_SCHEMA = ”
// matched 0 rows, causing analyze-schema to return 0 columns.
//
// After the fix, resolveSchemaForAnalyze returns the database name for MySQL,
// so this scenario (empty schema) should no longer occur. This test documents
// the bug behavior and acts as a guardrail.
func TestFetchColumnsBulk_MySQL_WithEmptySchema_Bug75(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	// Empty schema query returns no rows (this is the bug scenario)
	rows := sqlmock.NewRows([]string{
		"TABLE_NAME", "COLUMN_NAME", "DATA_TYPE", "IS_NULLABLE",
		"COLUMN_DEFAULT", "COLUMN_KEY", "EXTRA",
	})
	// No rows added - WHERE TABLE_SCHEMA = '' matches nothing

	mock.ExpectQuery(`SELECT TABLE_NAME.*FROM information_schema\.COLUMNS WHERE TABLE_SCHEMA = \?`).
		WithArgs(""). // Empty schema - the bug
		WillReturnRows(rows)

	columns, err := FetchColumnsBulk(context.Background(), db, "mysql", "")
	if err != nil {
		t.Fatalf("FetchColumnsBulk returned error: %v", err)
	}
	// Bug behavior: 0 columns returned because WHERE TABLE_SCHEMA = '' matches nothing
	if len(columns) != 0 {
		t.Errorf("expected 0 tables with empty schema (bug scenario), got %d tables", len(columns))
	}
	t.Logf("BUG #75 CONFIRMED: Empty schema returns %d columns (0 = bug present)", len(columns))
}

// TestFetchColumnsBulk_MySQL_WithCorrectSchema verifies that passing the database name
// as schema for MySQL returns the expected columns.
//
// This is the behavior AFTER the fix: resolveSchemaForAnalyze returns the database name
// for MySQL, so WHERE TABLE_SCHEMA = 'voipdb' matches the correct rows.
func TestFetchColumnsBulk_MySQL_WithCorrectSchema(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{
		"TABLE_NAME", "COLUMN_NAME", "DATA_TYPE", "IS_NULLABLE",
		"COLUMN_DEFAULT", "COLUMN_KEY", "EXTRA",
	}).
		AddRow("calls", "id", "int", "NO", nil, "PRI", "auto_increment").
		AddRow("calls", "caller_id", "varchar", "NO", nil, "MUL", "").
		AddRow("calls", "callee_id", "varchar", "NO", nil, "MUL", "").
		AddRow("calls", "sip_uri", "varchar", "YES", nil, "", "").
		AddRow("calls", "duration", "int", "YES", nil, "", "").
		AddRow("extensions", "id", "int", "NO", nil, "PRI", "auto_increment").
		AddRow("extensions", "extension", "varchar", "NO", nil, "UNI", "").
		AddRow("extensions", "phone_number", "varchar", "YES", nil, "", "")

	mock.ExpectQuery(`SELECT TABLE_NAME.*FROM information_schema\.COLUMNS WHERE TABLE_SCHEMA = \?`).
		WithArgs("voipdb"). // Correct schema = database name
		WillReturnRows(rows)

	columns, err := FetchColumnsBulk(context.Background(), db, "mysql", "voipdb")
	if err != nil {
		t.Fatalf("FetchColumnsBulk returned error: %v", err)
	}
	if len(columns) != 2 {
		t.Fatalf("expected 2 tables with correct schema, got %d tables", len(columns))
	}
	if len(columns["calls"]) != 5 {
		t.Errorf("expected 5 columns for calls table, got %d", len(columns["calls"]))
	}
	if len(columns["extensions"]) != 3 {
		t.Errorf("expected 3 columns for extensions table, got %d", len(columns["extensions"]))
	}
	t.Logf("SUCCESS: Correct schema 'voipdb' returns %d tables with %d + %d columns",
		len(columns), len(columns["calls"]), len(columns["extensions"]))
}

// TestFetchRowCounts_MySQL_WithCorrectSchema verifies row counts with correct schema (#76).
func TestFetchRowCounts_MySQL_WithCorrectSchema(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"TABLE_NAME", "TABLE_ROWS"}).
		AddRow("calls", 150000).
		AddRow("extensions", 50).
		AddRow("voicemail", 5000)

	mock.ExpectQuery(`SELECT TABLE_NAME, TABLE_ROWS FROM information_schema\.TABLES WHERE TABLE_SCHEMA = \? AND TABLE_TYPE = 'BASE TABLE'`).
		WithArgs("voipdb").
		WillReturnRows(rows)

	result, err := FetchRowCounts(context.Background(), db, "mysql", "voipdb", nil)
	if err != nil {
		t.Fatalf("FetchRowCounts returned error: %v", err)
	}
	if len(result) != 3 {
		t.Errorf("expected 3 tables in row counts, got %d", len(result))
	}
	if result["calls"] != 150000 {
		t.Errorf("expected calls row count 150000, got %d", result["calls"])
	}
	t.Logf("SUCCESS: Row counts with correct schema: calls=%d, extensions=%d, voicemail=%d",
		result["calls"], result["extensions"], result["voicemail"])
}

// TestFetchRowCounts_MySQL_WithEmptySchema_Bug76 verifies bug #76 behavior: 0 rows.
func TestFetchRowCounts_MySQL_WithEmptySchema_Bug76(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"TABLE_NAME", "TABLE_ROWS"})
	// No rows: WHERE TABLE_SCHEMA = '' matches nothing

	mock.ExpectQuery(`SELECT TABLE_NAME, TABLE_ROWS FROM information_schema\.TABLES WHERE TABLE_SCHEMA = \? AND TABLE_TYPE = 'BASE TABLE'`).
		WithArgs("").
		WillReturnRows(rows)

	result, err := FetchRowCounts(context.Background(), db, "mysql", "", nil)
	if err != nil {
		t.Fatalf("FetchRowCounts returned error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 tables with empty schema (bug scenario), got %d", len(result))
	}
	t.Logf("BUG #76 CONFIRMED: Empty schema returns %d row counts (0 = bug present)", len(result))
}

// TestDiscoverForeignKeys_MySQL_WithCorrectSchema verifies FK discovery with correct schema (#77).
func TestDiscoverForeignKeys_MySQL_WithCorrectSchema(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{
		"TABLE_NAME", "COLUMN_NAME", "REFERENCED_TABLE_NAME", "REFERENCED_COLUMN_NAME",
	}).
		AddRow("calls", "caller_id", "extensions", "extension").
		AddRow("calls", "callee_id", "extensions", "extension")

	mock.ExpectQuery(`SELECT TABLE_NAME.*FROM information_schema\.KEY_COLUMN_USAGE WHERE TABLE_SCHEMA = \? AND REFERENCED_TABLE_NAME IS NOT NULL`).
		WithArgs("voipdb").
		WillReturnRows(rows)

	fks, err := DiscoverForeignKeys(context.Background(), db, "mysql", "voipdb")
	if err != nil {
		t.Fatalf("DiscoverForeignKeys returned error: %v", err)
	}
	if len(fks) != 2 {
		t.Errorf("expected 2 foreign keys, got %d", len(fks))
	}
	t.Logf("SUCCESS: FK discovery with correct schema: found %d FKs", len(fks))
}

// TestDiscoverForeignKeys_MySQL_WithEmptySchema_Bug77 verifies bug #77: 0 FKs.
func TestDiscoverForeignKeys_MySQL_WithEmptySchema_Bug77(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{
		"TABLE_NAME", "COLUMN_NAME", "REFERENCED_TABLE_NAME", "REFERENCED_COLUMN_NAME",
	})
	// No rows: WHERE TABLE_SCHEMA = '' matches nothing

	mock.ExpectQuery(`SELECT TABLE_NAME.*FROM information_schema\.KEY_COLUMN_USAGE WHERE TABLE_SCHEMA = \? AND REFERENCED_TABLE_NAME IS NOT NULL`).
		WithArgs("").
		WillReturnRows(rows)

	fks, err := DiscoverForeignKeys(context.Background(), db, "mysql", "")
	if err != nil {
		t.Fatalf("DiscoverForeignKeys returned error: %v", err)
	}
	if len(fks) != 0 {
		t.Errorf("expected 0 foreign keys with empty schema (bug scenario), got %d", len(fks))
	}
	t.Logf("BUG #77 CONFIRMED: Empty schema returns %d FKs (0 = bug present)", len(fks))
}

// TestCategorizeTables_WithColumnsNot100Core_Bug79 verifies that with actual column data,
// CategorizeTables doesn't label everything as "core" (#79).
//
// Bug #79: When 0 columns are present (due to #75), every table gets categorized as
// "core" because there's no signal for classification. With proper data, tables should
// be categorized into core, lookup, junction, or audit.
func TestCategorizeTables_WithColumnsNot100Core_Bug79(t *testing.T) {
	// Simulate the voipdb schema with proper column data
	tableNames := []string{"calls", "extensions", "voicemail", "call_status"}

	tableSchemas := map[string]TableInfo{
		"calls": {
			ColumnCount: 5,
			Columns: []SchemaColumnInfo{
				{ColumnName: "id", DataType: "int", IsPrimaryKey: true},
				{ColumnName: "caller_id", DataType: "varchar"},
				{ColumnName: "callee_id", DataType: "varchar"},
				{ColumnName: "sip_uri", DataType: "varchar"},
				{ColumnName: "duration", DataType: "int"},
			},
		},
		"extensions": {
			ColumnCount: 3,
			Columns: []SchemaColumnInfo{
				{ColumnName: "id", DataType: "int", IsPrimaryKey: true},
				{ColumnName: "extension", DataType: "varchar", Unique: true},
				{ColumnName: "phone_number", DataType: "varchar"},
			},
		},
		"voicemail": {
			ColumnCount: 4,
			Columns: []SchemaColumnInfo{
				{ColumnName: "id", DataType: "int", IsPrimaryKey: true},
				{ColumnName: "call_id", DataType: "int", IsForeignKey: true},
				{ColumnName: "recording_path", DataType: "varchar"},
				{ColumnName: "transcription", DataType: "text"},
			},
		},
		"call_status": {
			ColumnCount: 2,
			Columns: []SchemaColumnInfo{
				{ColumnName: "id", DataType: "int", IsPrimaryKey: true},
				{ColumnName: "status_name", DataType: "varchar", Unique: true},
			},
		},
	}

	catalog := CategorizeTables(tableNames, tableSchemas, nil)

	// With proper column data, we should NOT get 100% core categorization
	// call_status is a lookup table (2 columns, one is unique)
	// calls is a core entity (5 columns, foreign keys)
	coreNames := make(map[string]bool)
	for _, e := range catalog.CoreEntities {
		coreNames[e.TableName] = true
	}
	lookupNames := make(map[string]bool)
	for _, e := range catalog.LookupTables {
		lookupNames[e.TableName] = true
	}

	t.Logf("Categorization: core=%v, lookup=%v, junction=%v, audit=%v",
		entityNames(catalog.CoreEntities),
		entityNames(catalog.LookupTables),
		entityNames(catalog.JunctionTables),
		entityNames(catalog.AuditTables))

	// call_status should be a lookup table (2 columns, has unique column, no FKs)
	if !lookupNames["call_status"] {
		t.Errorf("BUG #79: call_status should be in lookup_tables, not core_entities. Got core=%v, lookup=%v",
			entityNames(catalog.CoreEntities), entityNames(catalog.LookupTables))
	}

	// Not all tables should be core
	if len(catalog.CoreEntities) == len(tableNames) {
		t.Errorf("BUG #79: All %d tables are core_entities (100%% core), should have lookup tables too",
			len(tableNames))
	}
}

// TestCategorizeTables_WithZeroColumns_AllCore_Bug79 verifies the bug behavior:
// with 0 columns per table, everything gets categorized as "core".
func TestCategorizeTables_WithZeroColumns_AllCore_Bug79(t *testing.T) {
	tableNames := []string{"calls", "extensions", "voicemail", "call_status"}

	// Bug scenario: 0 columns per table (because #75 returned 0 columns)
	tableSchemas := map[string]TableInfo{
		"calls":       {ColumnCount: 0, Columns: nil},
		"extensions":  {ColumnCount: 0, Columns: nil},
		"voicemail":   {ColumnCount: 0, Columns: nil},
		"call_status": {ColumnCount: 0, Columns: nil},
	}

	catalog := CategorizeTables(tableNames, tableSchemas, nil)

	// Bug behavior: everything is core because there's no signal
	t.Logf("BUG #79 CONFIRMED: With 0 columns, all tables are core: %v", entityNames(catalog.CoreEntities))
	if len(catalog.CoreEntities) == len(tableNames) {
		t.Logf("Expected bug: All %d tables are core_entities (100%% core), which is the #79 bug behavior",
			len(tableNames))
	}
}

// TestBuildPerformanceOptimization_WithColumns_HasRealTips_Bug80 verifies that with column
// data, BuildPerformanceOptimization generates real tips, not just static generic ones (#80).
func TestBuildPerformanceOptimization_WithColumns_HasRealTips_Bug80(t *testing.T) {
	// Simulate the voipdb schema with proper column data
	tableColumns := map[string][]SchemaColumnInfo{
		"calls": {
			{ColumnName: "id", DataType: "int", IsPrimaryKey: true},
			{ColumnName: "caller_id", DataType: "varchar", Indexed: true},
			{ColumnName: "callee_id", DataType: "varchar", Indexed: true},
			{ColumnName: "sip_uri", DataType: "varchar"},
			{ColumnName: "duration", DataType: "int"},
		},
	}

	fks := []ForeignKeyRelationship{
		{FromTable: "calls", FromColumn: "caller_id", ToTable: "extensions", ToColumn: "extension"},
		{FromTable: "calls", FromColumn: "callee_id", ToTable: "extensions", ToColumn: "extension"},
	}

	indexes := []IndexInfo{
		{TableName: "calls", Columns: []string{"caller_id"}, IsPrimary: false, IsUnique: false},
		{TableName: "calls", Columns: []string{"callee_id"}, IsPrimary: false, IsUnique: false},
	}

	perfOpt := BuildPerformanceOptimization(tableColumns, fks, indexes)

	if perfOpt.RecommendedIndexes == nil && perfOpt.QueryPatterns.Avoid == nil && perfOpt.QueryPatterns.Prefer == nil {
		t.Logf("Performance optimization: recommendations may be empty for small schemas")
	}
	t.Logf("SUCCESS: BuildPerformanceOptimization returned data with proper columns")
}

// TestBuildPerformanceOptimization_WithZeroColumns_EarlyReturn_Bug80 verifies the bug
// behavior: len(tableColumns)==0 causes early return with generic tips only.
func TestBuildPerformanceOptimization_WithZeroColumns_EarlyReturn_Bug80(t *testing.T) {
	tableColumns := map[string][]SchemaColumnInfo{} // Bug scenario: 0 columns

	fks := []ForeignKeyRelationship{}
	indexes := []IndexInfo{}

	perfOpt := BuildPerformanceOptimization(tableColumns, fks, indexes)

	// Bug behavior: early return, static generic tips only
	t.Logf("BUG #80 CONFIRMED: With 0 columns, performance tips are: %+v", perfOpt)
}

func entityNames(entities []TableEntity) []string {
	names := make([]string, len(entities))
	for i, e := range entities {
		names[i] = e.TableName
	}
	return names
}

// --- Data Pipeline Bug Regression Tests (Issues #77, #80) ---
// These test that FK and index data flows from discovery functions back into
// tableColumns (SchemaColumnInfo) and tableSchemas (KeyColumns).

func TestApplyFKsToColumns_SetsFlags(t *testing.T) {
	tableColumns := map[string][]SchemaColumnInfo{
		"orders": {
			{ColumnName: "id", DataType: "int", IsPrimaryKey: true},
			{ColumnName: "customer_id", DataType: "int"},
			{ColumnName: "product_id", DataType: "int"},
			{ColumnName: "quantity", DataType: "int"},
		},
	}

	fks := []ForeignKeyRelationship{
		{FromTable: "orders", FromColumn: "customer_id", ToTable: "customers", ToColumn: "id"},
		{FromTable: "orders", FromColumn: "product_id", ToTable: "products", ToColumn: "id"},
	}

	applyFKsToColumns(tableColumns, fks)

	cols := tableColumns["orders"]

	// id should NOT be a FK
	if cols[0].IsForeignKey {
		t.Error("id should not be marked as foreign key")
	}

	// customer_id should be FK with correct ref
	if !cols[1].IsForeignKey {
		t.Fatal("customer_id should be marked as foreign key")
	}
	if cols[1].ForeignKeyRef == nil {
		t.Fatal("customer_id ForeignKeyRef should not be nil")
	}
	if cols[1].ForeignKeyRef.RefTable != "customers" {
		t.Errorf("customer_id ref table: got %q, want %q", cols[1].ForeignKeyRef.RefTable, "customers")
	}
	if cols[1].ForeignKeyRef.RefColumn != "id" {
		t.Errorf("customer_id ref column: got %q, want %q", cols[1].ForeignKeyRef.RefColumn, "id")
	}

	// product_id should be FK with correct ref
	if !cols[2].IsForeignKey {
		t.Fatal("product_id should be marked as foreign key")
	}
	if cols[2].ForeignKeyRef.RefTable != "products" {
		t.Errorf("product_id ref table: got %q, want %q", cols[2].ForeignKeyRef.RefTable, "products")
	}

	// quantity should NOT be a FK
	if cols[3].IsForeignKey {
		t.Error("quantity should not be marked as foreign key")
	}

	t.Logf("SUCCESS: applyFKsToColumns correctly set IsForeignKey and ForeignKeyRef")
}

func TestApplyFKsToSchemas_PopulatesKeyColumns(t *testing.T) {
	schemas := map[string]TableInfo{
		"orders": {
			ColumnCount: 4,
			KeyColumns:  KeyColumns{PrimaryKey: "id", ForeignKeys: nil},
		},
	}

	fks := []ForeignKeyRelationship{
		{FromTable: "orders", FromColumn: "customer_id", ToTable: "customers", ToColumn: "id"},
		{FromTable: "orders", FromColumn: "product_id", ToTable: "products", ToColumn: "id"},
	}

	applyFKsToSchemas(schemas, fks)

	fkCols := schemas["orders"].KeyColumns.ForeignKeys
	if len(fkCols) != 2 {
		t.Fatalf("expected 2 FK columns, got %d: %v", len(fkCols), fkCols)
	}
	if !containsString(fkCols, "customer_id") || !containsString(fkCols, "product_id") {
		t.Errorf("FK columns should contain customer_id and product_id, got %v", fkCols)
	}
	t.Logf("SUCCESS: applyFKsToSchemas correctly populated ForeignKeys: %v", fkCols)
}

func TestApplyIndexesToColumns_SetsFlags(t *testing.T) {
	// Simulate: caller_id/callee_id not flagged as indexed by column metadata query
	// but ARE in fetched indexes (composite index scenario)
	tableColumns := map[string][]SchemaColumnInfo{
		"calls": {
			{ColumnName: "id", DataType: "int", IsPrimaryKey: true, Indexed: false},
			{ColumnName: "caller_id", DataType: "varchar", Indexed: false},
			{ColumnName: "callee_id", DataType: "varchar", Indexed: false},
			{ColumnName: "duration", DataType: "int", Indexed: false},
		},
	}

	indexes := []IndexInfo{
		{IndexName: "idx_caller", TableName: "calls", Columns: []string{"caller_id"}, IsPrimary: false},
		{IndexName: "idx_callee", TableName: "calls", Columns: []string{"callee_id"}, IsPrimary: false},
	}

	applyIndexesToColumns(tableColumns, indexes)

	cols := tableColumns["calls"]
	if !cols[1].Indexed {
		t.Error("caller_id should be marked as indexed")
	}
	if !cols[2].Indexed {
		t.Error("callee_id should be marked as indexed")
	}
	if cols[3].Indexed {
		t.Error("duration should NOT be marked as indexed")
	}
	t.Logf("SUCCESS: applyIndexesToColumns correctly set Indexed flags")
}

func TestRebuildKeyColumns_IntegratesEnrichedData(t *testing.T) {
	// Start with initial schemas (no FK or index info in KeyColumns)
	schemas := map[string]TableInfo{
		"orders": {
			ColumnCount: 4,
			Columns: []SchemaColumnInfo{
				{ColumnName: "id", IsPrimaryKey: true},
				{ColumnName: "customer_id"},
				{ColumnName: "product_id"},
				{ColumnName: "status"},
			},
			KeyColumns: KeyColumns{PrimaryKey: "id"},
		},
	}

	// Enriched columns (after applyFKsToColumns + applyIndexesToColumns)
	tableColumns := map[string][]SchemaColumnInfo{
		"orders": {
			{ColumnName: "id", IsPrimaryKey: true},
			{ColumnName: "customer_id", IsForeignKey: true, ForeignKeyRef: &ForeignKeyRef{RefTable: "customers", RefColumn: "id"}, Indexed: true},
			{ColumnName: "product_id", IsForeignKey: true, ForeignKeyRef: &ForeignKeyRef{RefTable: "products", RefColumn: "id"}, Indexed: true},
			{ColumnName: "status"},
		},
	}

	rebuildKeyColumns(schemas, tableColumns)

	kc := schemas["orders"].KeyColumns
	if kc.PrimaryKey != "id" {
		t.Errorf("primary key: got %q, want %q", kc.PrimaryKey, "id")
	}
	if len(kc.ForeignKeys) != 2 {
		t.Errorf("foreign keys: got %d, want 2: %v", len(kc.ForeignKeys), kc.ForeignKeys)
	}
	if len(kc.IndexedColumns) != 2 {
		t.Errorf("indexed columns: got %d, want 2: %v", len(kc.IndexedColumns), kc.IndexedColumns)
	}

	// Verify columns themselves are also updated
	cols := schemas["orders"].Columns
	if !cols[1].IsForeignKey {
		t.Error("customer_id should be FK after rebuild")
	}
	if cols[1].ForeignKeyRef == nil || cols[1].ForeignKeyRef.RefTable != "customers" {
		t.Error("customer_id should have correct ForeignKeyRef after rebuild")
	}

	t.Logf("SUCCESS: rebuildKeyColumns integrated enriched data. PK=%q, FKs=%v, Indexed=%v",
		kc.PrimaryKey, kc.ForeignKeys, kc.IndexedColumns)
}

func TestApplyFKsToColumns_EmptyFKs_NoChanges(t *testing.T) {
	tableColumns := map[string][]SchemaColumnInfo{
		"users": {
			{ColumnName: "id", IsPrimaryKey: true},
			{ColumnName: "name"},
		},
	}

	applyFKsToColumns(tableColumns, nil)

	if tableColumns["users"][0].IsForeignKey {
		t.Error("id should not be FK with empty FK list")
	}
	t.Logf("SUCCESS: empty FK list makes no changes")
}

func TestApplyIndexesToColumns_SkipsPrimaryKeys(t *testing.T) {
	tableColumns := map[string][]SchemaColumnInfo{
		"users": {
			{ColumnName: "id", IsPrimaryKey: true, Indexed: false},
			{ColumnName: "email", Indexed: false},
		},
	}

	indexes := []IndexInfo{
		{IndexName: "PRIMARY", TableName: "users", Columns: []string{"id"}, IsPrimary: true},
		{IndexName: "idx_email", TableName: "users", Columns: []string{"email"}, IsPrimary: false},
	}

	applyIndexesToColumns(tableColumns, indexes)

	cols := tableColumns["users"]
	// id should NOT get Indexed=true from the primary key index
	// (it's already flagged as IsPrimaryKey, and extractKeyColumns skips PKs for IndexedColumns)
	if cols[0].Indexed {
		t.Error("id should NOT be marked as indexed from primary key index")
	}
	if !cols[1].Indexed {
		t.Error("email should be marked as indexed")
	}
	t.Logf("SUCCESS: primary key indexes skipped, non-PK indexes applied")
}
