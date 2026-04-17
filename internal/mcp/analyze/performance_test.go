package analyze

import (
	"context"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestFetchIndexes_MySQL(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	// MySQL information_schema.STATISTICS returns one row per column in an index
	rows := sqlmock.NewRows([]string{"TABLE_NAME", "INDEX_NAME", "COLUMN_NAME", "NON_UNIQUE", "SEQ_IN_INDEX"}).
		AddRow("users", "PRIMARY", "id", 0, 1).
		AddRow("users", "idx_email", "email", 1, 1).
		AddRow("orders", "PRIMARY", "id", 0, 1).
		AddRow("orders", "idx_user_id", "user_id", 1, 1).
		AddRow("orders", "idx_user_id", "created_at", 1, 2) // composite index

	mock.ExpectQuery(`SELECT TABLE_NAME, INDEX_NAME, COLUMN_NAME, NON_UNIQUE, SEQ_IN_INDEX FROM information_schema\.STATISTICS WHERE TABLE_SCHEMA = \? ORDER BY TABLE_NAME, INDEX_NAME, SEQ_IN_INDEX`).
		WithArgs("mydb").
		WillReturnRows(rows)

	indexes, err := FetchIndexes(context.Background(), db, "mysql", "mydb")
	if err != nil {
		t.Fatalf("FetchIndexes returned error: %v", err)
	}

	// Should have 3 indexes: PRIMARY on users, idx_email on users, PRIMARY on orders, idx_user_id on orders
	if len(indexes) != 4 {
		t.Fatalf("expected 4 indexes, got %d", len(indexes))
	}

	// Check PRIMARY on users
	pk := findIndexByName(indexes, "users", "PRIMARY")
	if pk == nil {
		t.Fatal("expected PRIMARY index on users")
	}
	if !pk.IsPrimary {
		t.Error("expected PRIMARY to be IsPrimary")
	}
	if len(pk.Columns) != 1 || pk.Columns[0] != "id" {
		t.Errorf("expected PRIMARY columns [id], got %v", pk.Columns)
	}

	// Check composite index on orders
	comp := findIndexByName(indexes, "orders", "idx_user_id")
	if comp == nil {
		t.Fatal("expected idx_user_id index on orders")
	}
	if len(comp.Columns) != 2 || comp.Columns[0] != "user_id" || comp.Columns[1] != "created_at" {
		t.Errorf("expected idx_user_id columns [user_id, created_at], got %v", comp.Columns)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestFetchIndexes_PostgreSQL(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	// PostgreSQL pg_indexes returns one row per index
	// We'll parse the indexdef to extract columns
	rows := sqlmock.NewRows([]string{"tablename", "indexname", "indexdef"}).
		AddRow("users", "users_pkey", "CREATE UNIQUE INDEX users_pkey ON public.users USING btree (id)").
		AddRow("users", "idx_users_email", "CREATE INDEX idx_users_email ON public.users USING btree (email)").
		AddRow("orders", "orders_pkey", "CREATE UNIQUE INDEX orders_pkey ON public.orders USING btree (id)")

	mock.ExpectQuery(`SELECT tablename, indexname, indexdef FROM pg_indexes WHERE schemaname = \$1 ORDER BY tablename, indexname`).
		WithArgs("public").
		WillReturnRows(rows)

	indexes, err := FetchIndexes(context.Background(), db, "postgres", "public")
	if err != nil {
		t.Fatalf("FetchIndexes returned error: %v", err)
	}

	if len(indexes) != 3 {
		t.Fatalf("expected 3 indexes, got %d", len(indexes))
	}

	pk := findIndexByName(indexes, "users", "users_pkey")
	if pk == nil {
		t.Fatal("expected users_pkey index on users")
	}
	if !pk.IsPrimary {
		t.Error("expected users_pkey to be IsPrimary")
	}
	if len(pk.Columns) != 1 || pk.Columns[0] != "id" {
		t.Errorf("expected users_pkey columns [id], got %v", pk.Columns)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestFetchIndexes_SQLite(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	// SQLite: first query PRAGMA index_list, then PRAGMA index_info per index
	// index_list for users table
	indexList := sqlmock.NewRows([]string{"seq", "name", "unique"}).
		AddRow(0, "idx_users_email", 1).
		AddRow(1, "sqlite_autoindex_users_1", 1) // auto-created for PRIMARY KEY

	mock.ExpectQuery(`PRAGMA index_list\('users'\)`).WillReturnRows(indexList)

	// index_info for idx_users_email
	indexInfo := sqlmock.NewRows([]string{"seqno", "cid", "name"}).
		AddRow(0, 1, "email")

	mock.ExpectQuery(`PRAGMA index_info\('idx_users_email'\)`).WillReturnRows(indexInfo)

	// index_info for sqlite_autoindex_users_1
	autoIndexInfo := sqlmock.NewRows([]string{"seqno", "cid", "name"}).
		AddRow(0, 0, "id")

	mock.ExpectQuery(`PRAGMA index_info\('sqlite_autoindex_users_1'\)`).WillReturnRows(autoIndexInfo)

	indexes, err := FetchIndexes(context.Background(), db, "sqlite", "", []string{"users"})
	if err != nil {
		t.Fatalf("FetchIndexes returned error: %v", err)
	}

	if len(indexes) != 2 {
		t.Fatalf("expected 2 indexes, got %d", len(indexes))
	}

	emailIdx := findIndexByName(indexes, "users", "idx_users_email")
	if emailIdx == nil {
		t.Fatal("expected idx_users_email index on users")
	}
	if !emailIdx.IsUnique {
		t.Error("expected idx_users_email to be IsUnique")
	}
	if len(emailIdx.Columns) != 1 || emailIdx.Columns[0] != "email" {
		t.Errorf("expected idx_users_email columns [email], got %v", emailIdx.Columns)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestFetchIndexes_EmptyResult(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"TABLE_NAME", "INDEX_NAME", "COLUMN_NAME", "NON_UNIQUE", "SEQ_IN_INDEX"})
	mock.ExpectQuery(`SELECT TABLE_NAME, INDEX_NAME, COLUMN_NAME, NON_UNIQUE, SEQ_IN_INDEX FROM information_schema\.STATISTICS.*`).
		WithArgs("emptydb").
		WillReturnRows(rows)

	indexes, err := FetchIndexes(context.Background(), db, "mysql", "emptydb")
	if err != nil {
		t.Fatalf("FetchIndexes returned error: %v", err)
	}
	if len(indexes) != 0 {
		t.Fatalf("expected 0 indexes, got %d", len(indexes))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestFetchIndexes_UnsupportedDB(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	_, err = FetchIndexes(context.Background(), db, "oracle", "somedb")
	if err == nil {
		t.Fatal("expected error for unsupported db type, got nil")
	}
}

func TestBuildPerformanceOptimization(t *testing.T) {
	// Test with FK columns that lack indexes
	fks := []ForeignKeyRelationship{
		{FromTable: "orders", FromColumn: "user_id", ToTable: "users", ToColumn: "id"},
		{FromTable: "order_items", FromColumn: "order_id", ToTable: "orders", ToColumn: "id"},
	}

	tableColumns := map[string][]SchemaColumnInfo{
		"orders": {
			{ColumnName: "id", IsPrimaryKey: true},
			{ColumnName: "user_id", IsForeignKey: true},
			{ColumnName: "created_at"},
		},
		"order_items": {
			{ColumnName: "id", IsPrimaryKey: true},
			{ColumnName: "order_id", IsForeignKey: true},
			{ColumnName: "product_id", IsForeignKey: true},
		},
	}

	// Existing indexes: only PRIMARY keys
	existingIndexes := []IndexInfo{
		{TableName: "orders", IndexName: "PRIMARY", Columns: []string{"id"}, IsPrimary: true},
		{TableName: "order_items", IndexName: "PRIMARY", Columns: []string{"id"}, IsPrimary: true},
	}

	result := BuildPerformanceOptimization(tableColumns, fks, existingIndexes)

	// Should recommend indexes on FK columns that don't have indexes
	if len(result.RecommendedIndexes) == 0 {
		t.Fatal("expected recommended indexes, got none")
	}

	// Check that user_id on orders is recommended
	foundUserID := false
	foundOrderID := false
	for _, rec := range result.RecommendedIndexes {
		if rec.Table == "orders" && len(rec.Columns) == 1 && rec.Columns[0] == "user_id" {
			foundUserID = true
		}
		if rec.Table == "order_items" && len(rec.Columns) == 1 && rec.Columns[0] == "order_id" {
			foundOrderID = true
		}
	}
	if !foundUserID {
		t.Error("expected recommendation for orders.user_id")
	}
	if !foundOrderID {
		t.Error("expected recommendation for order_items.order_id")
	}

	// QueryPatterns should have some recommendations
	if len(result.QueryPatterns.Prefer) == 0 {
		t.Error("expected query pattern preferences")
	}
}

func TestBuildPerformanceOptimization_NoRecommendations(t *testing.T) {
	// All FK columns already have indexes — no recommendations needed
	fks := []ForeignKeyRelationship{
		{FromTable: "orders", FromColumn: "user_id", ToTable: "users", ToColumn: "id"},
	}

	tableColumns := map[string][]SchemaColumnInfo{
		"orders": {
			{ColumnName: "id", IsPrimaryKey: true},
			{ColumnName: "user_id", IsForeignKey: true},
		},
	}

	// Existing indexes cover the FK column
	existingIndexes := []IndexInfo{
		{TableName: "orders", IndexName: "PRIMARY", Columns: []string{"id"}, IsPrimary: true},
		{TableName: "orders", IndexName: "idx_user_id", Columns: []string{"user_id"}, IsUnique: false},
	}

	result := BuildPerformanceOptimization(tableColumns, fks, existingIndexes)

	// Should have no index recommendations (FK already indexed)
	if len(result.RecommendedIndexes) != 0 {
		t.Errorf("expected 0 recommended indexes, got %d: %v", len(result.RecommendedIndexes), result.RecommendedIndexes)
	}
}

func TestBuildPerformanceOptimization_EmptyInput(t *testing.T) {
	result := BuildPerformanceOptimization(nil, nil, nil)

	if len(result.RecommendedIndexes) != 0 {
		t.Errorf("expected 0 recommended indexes for empty input, got %d", len(result.RecommendedIndexes))
	}
	if len(result.QueryPatterns.Prefer) == 0 {
		t.Error("expected default query pattern preferences even for empty input")
	}
}

// findIndexByName is a test helper
func findIndexByName(indexes []IndexInfo, tableName, indexName string) *IndexInfo {
	for _, idx := range indexes {
		if idx.TableName == tableName && idx.IndexName == indexName {
			return &idx
		}
	}
	return nil
}
