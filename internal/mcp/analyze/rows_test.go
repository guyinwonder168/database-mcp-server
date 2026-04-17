package analyze

import (
	"context"
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
