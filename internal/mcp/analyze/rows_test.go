package analyze

import (
	"context"
	"fmt"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestFetchRowCounts_MySQL(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"TABLE_NAME", "TABLE_ROWS"}).
		AddRow("users", int64(50000)).
		AddRow("orders", int64(120000)).
		AddRow("products", int64(3500))

	mock.ExpectQuery(`SELECT TABLE_NAME, TABLE_ROWS FROM information_schema\.TABLES WHERE TABLE_SCHEMA = \? AND TABLE_TYPE = 'BASE TABLE'`).
		WithArgs("mydb").
		WillReturnRows(rows)

	counts, err := FetchRowCounts(context.Background(), db, "mysql", "mydb", []string{"users", "orders", "products"})
	if err != nil {
		t.Fatalf("FetchRowCounts returned error: %v", err)
	}
	if counts["users"] != 50000 {
		t.Errorf("expected users row_count=50000, got %d", counts["users"])
	}
	if counts["orders"] != 120000 {
		t.Errorf("expected orders row_count=120000, got %d", counts["orders"])
	}
	if counts["products"] != 3500 {
		t.Errorf("expected products row_count=3500, got %d", counts["products"])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestFetchRowCounts_PostgreSQL(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"relname", "n_live_tup"}).
		AddRow("users", int64(25000)).
		AddRow("orders", int64(80000))

	mock.ExpectQuery(`SELECT relname, n_live_tup FROM pg_stat_user_tables WHERE schemaname = \$1`).
		WithArgs("public").
		WillReturnRows(rows)

	counts, err := FetchRowCounts(context.Background(), db, "postgres", "public", []string{"users", "orders"})
	if err != nil {
		t.Fatalf("FetchRowCounts returned error: %v", err)
	}
	if counts["users"] != 25000 {
		t.Errorf("expected users row_count=25000, got %d", counts["users"])
	}
	if counts["orders"] != 80000 {
		t.Errorf("expected orders row_count=80000, got %d", counts["orders"])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestFetchRowCounts_SQLite(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	// SQLite uses COUNT(*) per table
	userRows := sqlmock.NewRows([]string{"cnt"}).AddRow(int64(150))
	mock.ExpectQuery(`SELECT COUNT\(\*\) AS cnt FROM "users"`).WillReturnRows(userRows)

	orderRows := sqlmock.NewRows([]string{"cnt"}).AddRow(int64(4200))
	mock.ExpectQuery(`SELECT COUNT\(\*\) AS cnt FROM "orders"`).WillReturnRows(orderRows)

	counts, err := FetchRowCounts(context.Background(), db, "sqlite", "", []string{"users", "orders"})
	if err != nil {
		t.Fatalf("FetchRowCounts returned error: %v", err)
	}
	if counts["users"] != 150 {
		t.Errorf("expected users row_count=150, got %d", counts["users"])
	}
	if counts["orders"] != 4200 {
		t.Errorf("expected orders row_count=4200, got %d", counts["orders"])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestFetchRowCounts_EmptyResult(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"TABLE_NAME", "TABLE_ROWS"})
	mock.ExpectQuery(`SELECT TABLE_NAME, TABLE_ROWS FROM information_schema\.TABLES.*`).
		WithArgs("emptydb").
		WillReturnRows(rows)

	counts, err := FetchRowCounts(context.Background(), db, "mysql", "emptydb", []string{})
	if err != nil {
		t.Fatalf("FetchRowCounts returned error: %v", err)
	}
	if len(counts) != 0 {
		t.Fatalf("expected 0 tables, got %d", len(counts))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestFetchRowCounts_SQLiteTableFailure(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	// First table succeeds
	userRows := sqlmock.NewRows([]string{"cnt"}).AddRow(int64(10))
	mock.ExpectQuery(`SELECT COUNT\(\*\) AS cnt FROM "users"`).WillReturnRows(userRows)

	// Second table fails — no expectation set, sqlmock returns error
	// SQLite COUNT(*) failure should be silently skipped

	counts, err := FetchRowCounts(context.Background(), db, "sqlite", "", []string{"users"})
	if err != nil {
		t.Fatalf("FetchRowCounts returned error: %v", err)
	}
	if counts["users"] != 10 {
		t.Errorf("expected users row_count=10, got %d", counts["users"])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestFetchRowCounts_UnsupportedDB(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	_, err = FetchRowCounts(context.Background(), db, "oracle", "somedb", []string{"users"})
	if err == nil {
		t.Fatal("expected error for unsupported db type, got nil")
	}
}

// --- Sample Row Fetching Tests ---

func TestNormalizeSampleSize(t *testing.T) {
	tests := []struct {
		name  string
		input int
		want  int
	}{
		{"zero defaults to 10", 0, 10},
		{"negative defaults to 10", -5, 10},
		{"positive passthrough", 25, 25},
		{"one passthrough", 1, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeSampleSize(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeSampleSize(%d) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestSampleQueryForDB(t *testing.T) {
	tests := []struct {
		name      string
		dbType    string
		table     string
		size      int
		wantQuery string
		wantOK    bool
	}{
		{"MySQL", "mysql", "users", 5, "SELECT * FROM `users` LIMIT 5", true},
		{"MariaDB", "mariadb", "orders", 10, "SELECT * FROM `orders` LIMIT 10", true},
		{"Postgres", "postgres", "users", 5, "SELECT * FROM \"users\" LIMIT 5", true},
		{"PostgreSQL", "postgresql", "users", 5, "SELECT * FROM \"users\" LIMIT 5", true},
		{"SQLite", "sqlite", "users", 5, `SELECT * FROM "users" LIMIT 5`, true},
		{"Unsupported", "oracle", "users", 5, "", false},
		{"Empty dbType", "", "users", 5, "", false},
		{"InvalidIdentifier_SQLInjection", "sqlite", "users; DROP TABLE", 5, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := SampleQueryForDB(tt.dbType, tt.table, tt.size)
			if ok != tt.wantOK {
				t.Errorf("SampleQueryForDB() ok = %v, want %v", ok, tt.wantOK)
			}
			if got != tt.wantQuery {
				t.Errorf("SampleQueryForDB() query = %q, want %q", got, tt.wantQuery)
			}
		})
	}
}

func TestNormalizeSampleRow(t *testing.T) {
	tests := []struct {
		name     string
		columns  []string
		row      []interface{}
		wantKeys []string
		wantVals map[string]interface{}
	}{
		{
			name:     "string values",
			columns:  []string{"id", "name"},
			row:      []interface{}{int64(1), "Alice"},
			wantKeys: []string{"id", "name"},
			wantVals: map[string]interface{}{"id": int64(1), "name": "Alice"},
		},
		{
			name:     "byte slice converted to string",
			columns:  []string{"data"},
			row:      []interface{}{[]byte("hello")},
			wantKeys: []string{"data"},
			wantVals: map[string]interface{}{"data": "hello"},
		},
		{
			name:     "nil value preserved",
			columns:  []string{"id", "deleted_at"},
			row:      []interface{}{int64(1), nil},
			wantKeys: []string{"id", "deleted_at"},
			wantVals: map[string]interface{}{"id": int64(1), "deleted_at": nil},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeSampleRow(tt.columns, tt.row)
			if len(got) != len(tt.wantKeys) {
				t.Fatalf("NormalizeSampleRow() len = %d, want %d", len(got), len(tt.wantKeys))
			}
			for k, v := range tt.wantVals {
				if got[k] != v {
					t.Errorf("NormalizeSampleRow()[%q] = %v, want %v", k, got[k], v)
				}
			}
		})
	}
}

func TestScanSampleRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	// Simulate a SELECT * query returning 2 rows
	mockRows := sqlmock.NewRows([]string{"id", "name"}).
		AddRow(int64(1), "Alice").
		AddRow(int64(2), "Bob")
	mock.ExpectQuery("SELECT").WillReturnRows(mockRows)

	rows, err := db.QueryContext(context.Background(), "SELECT * FROM users")
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	defer rows.Close()

	result := ScanSampleRows(rows, "users")
	if len(result) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(result))
	}
	if result[0]["name"] != "Alice" {
		t.Errorf("expected row[0][name] = Alice, got %v", result[0]["name"])
	}
	if result[1]["name"] != "Bob" {
		t.Errorf("expected row[1][name] = Bob, got %v", result[1]["name"])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestScanSampleRows_ByteConversion(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	// sqlmock returns []byte for string columns
	mockRows := sqlmock.NewRows([]string{"id", "data"}).
		AddRow(int64(1), []byte("binary data"))
	mock.ExpectQuery("SELECT").WillReturnRows(mockRows)

	rows, err := db.QueryContext(context.Background(), "SELECT * FROM test")
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	defer rows.Close()

	result := ScanSampleRows(rows, "test")
	if len(result) != 1 {
		t.Fatalf("expected 1 row, got %d", len(result))
	}
	// NormalizeSampleRow should convert []byte to string
	if s, ok := result[0]["data"].(string); !ok || s != "binary data" {
		t.Errorf("expected data to be string 'binary data', got %T %v", result[0]["data"], result[0]["data"])
	}
}

func TestFetchSampleRows_UnsupportedDB(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	result := FetchSampleRows(context.Background(), db, "users", "oracle", 5)
	if len(result) != 0 {
		t.Errorf("expected empty result for unsupported db, got %d rows", len(result))
	}
}

func TestFetchSampleRows_MySQL(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	mockRows := sqlmock.NewRows([]string{"id", "name"}).
		AddRow(int64(1), "Alice").
		AddRow(int64(2), "Bob")
	mock.ExpectQuery("SELECT \\* FROM `users` LIMIT 5").WillReturnRows(mockRows)

	result := FetchSampleRows(context.Background(), db, "users", "mysql", 5)
	if len(result) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(result))
	}
	if result[0]["name"] != "Alice" {
		t.Errorf("expected row[0][name] = Alice, got %v", result[0]["name"])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestFetchSampleRows_QueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT").WillReturnError(fmt.Errorf("connection lost"))

	result := FetchSampleRows(context.Background(), db, "users", "mysql", 5)
	if len(result) != 0 {
		t.Errorf("expected empty result on query error, got %d rows", len(result))
	}
}
