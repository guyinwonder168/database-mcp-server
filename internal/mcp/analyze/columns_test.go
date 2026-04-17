package analyze

import (
	"context"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestFetchColumnsBulk_MySQL(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{
		"TABLE_NAME", "COLUMN_NAME", "DATA_TYPE", "IS_NULLABLE",
		"COLUMN_DEFAULT", "COLUMN_KEY", "EXTRA",
	}).
		AddRow("users", "id", "int", "NO", nil, "PRI", "auto_increment").
		AddRow("users", "email", "varchar", "NO", nil, "UNI", "").
		AddRow("users", "name", "varchar", "YES", nil, "", "").
		AddRow("orders", "id", "int", "NO", nil, "PRI", "auto_increment").
		AddRow("orders", "user_id", "int", "NO", nil, "MUL", "")

	mock.ExpectQuery(`SELECT TABLE_NAME, COLUMN_NAME, DATA_TYPE, IS_NULLABLE, COLUMN_DEFAULT, COLUMN_KEY, EXTRA FROM information_schema\.COLUMNS WHERE TABLE_SCHEMA = \? ORDER BY TABLE_NAME, ORDINAL_POSITION`).
		WithArgs("mydb").
		WillReturnRows(rows)

	columns, err := FetchColumnsBulk(context.Background(), db, "mysql", "mydb")
	if err != nil {
		t.Fatalf("FetchColumnsBulk returned error: %v", err)
	}
	if len(columns) != 2 {
		t.Fatalf("expected 2 tables, got %d", len(columns))
	}
	if len(columns["users"]) != 3 {
		t.Fatalf("expected 3 columns for users, got %d", len(columns["users"]))
	}
	if len(columns["orders"]) != 2 {
		t.Fatalf("expected 2 columns for orders, got %d", len(columns["orders"]))
	}

	// Check primary key detection
	if !columns["users"][0].IsPrimaryKey {
		t.Error("expected users.id to be primary key")
	}
	if columns["users"][0].ColumnName != "id" {
		t.Errorf("expected first column name 'id', got %q", columns["users"][0].ColumnName)
	}
	if !columns["users"][1].Unique {
		t.Error("expected users.email to be unique")
	}
	if !columns["users"][1].Indexed {
		t.Error("expected users.email to be indexed (UNI key)")
	}
	if !columns["orders"][1].Indexed {
		t.Error("expected orders.user_id to be indexed (MUL key)")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestFetchColumnsBulk_PostgreSQL(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{
		"table_name", "column_name", "data_type", "is_nullable",
		"column_default", "constraint_type",
	}).
		AddRow("users", "id", "integer", "NO", "nextval('users_id_seq')", "PRIMARY KEY").
		AddRow("users", "name", "character varying", "YES", nil, "")

	mock.ExpectQuery(`SELECT c\.table_name, c\.column_name, c\.data_type, c\.is_nullable, c\.column_default, COALESCE\(.*\) AS constraint_type FROM information_schema\.columns c WHERE c\.table_schema = \? ORDER BY c\.table_name, c\.ordinal_position`).
		WithArgs("public").
		WillReturnRows(rows)

	columns, err := FetchColumnsBulk(context.Background(), db, "postgres", "public")
	if err != nil {
		t.Fatalf("FetchColumnsBulk returned error: %v", err)
	}
	if len(columns) != 1 {
		t.Fatalf("expected 1 table, got %d", len(columns))
	}
	if len(columns["users"]) != 2 {
		t.Fatalf("expected 2 columns for users, got %d", len(columns["users"]))
	}
	if !columns["users"][0].IsPrimaryKey {
		t.Error("expected users.id to be primary key")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestFetchColumnsBulk_EmptyResult(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{
		"TABLE_NAME", "COLUMN_NAME", "DATA_TYPE", "IS_NULLABLE",
		"COLUMN_DEFAULT", "COLUMN_KEY", "EXTRA",
	})

	mock.ExpectQuery(`SELECT.*information_schema\.COLUMNS.*`).
		WithArgs("emptydb").
		WillReturnRows(rows)

	columns, err := FetchColumnsBulk(context.Background(), db, "mysql", "emptydb")
	if err != nil {
		t.Fatalf("FetchColumnsBulk returned error: %v", err)
	}
	if len(columns) != 0 {
		t.Fatalf("expected 0 tables for empty db, got %d", len(columns))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestFetchColumnsBulk_UnsupportedDB(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	_, err = FetchColumnsBulk(context.Background(), db, "oracle", "somedb")
	if err == nil {
		t.Fatal("expected error for unsupported db type, got nil")
	}
}

func TestFetchColumnsBulk_MySQLNullable(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{
		"TABLE_NAME", "COLUMN_NAME", "DATA_TYPE", "IS_NULLABLE",
		"COLUMN_DEFAULT", "COLUMN_KEY", "EXTRA",
	}).
		AddRow("products", "id", "int", "NO", nil, "PRI", "").
		AddRow("products", "description", "text", "YES", nil, "", "").
		AddRow("products", "price", "decimal", "NO", "0.00", "", "")

	mock.ExpectQuery(`SELECT.*information_schema\.COLUMNS.*`).
		WithArgs("shopdb").
		WillReturnRows(rows)

	columns, err := FetchColumnsBulk(context.Background(), db, "mysql", "shopdb")
	if err != nil {
		t.Fatalf("FetchColumnsBulk returned error: %v", err)
	}

	// Nullable checks
	if columns["products"][1].IsNullable != true {
		t.Error("expected description to be nullable")
	}
	if columns["products"][2].IsNullable != false {
		t.Error("expected price to be NOT nullable")
	}
	// Default value check
	if dv, ok := columns["products"][2].DefaultValue.(string); !ok || dv != "0.00" {
		t.Errorf("expected default '0.00', got %v", columns["products"][2].DefaultValue)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestFetchColumnsBulk_PostgreSQLNullable(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{
		"table_name", "column_name", "data_type", "is_nullable",
		"column_default", "constraint_type",
	}).
		AddRow("items", "id", "integer", "NO", nil, "PRIMARY KEY").
		AddRow("items", "notes", "text", "YES", nil, "")

	mock.ExpectQuery(`SELECT c\.table_name.*information_schema\.columns c.*`).
		WithArgs("public").
		WillReturnRows(rows)

	columns, err := FetchColumnsBulk(context.Background(), db, "postgres", "public")
	if err != nil {
		t.Fatalf("FetchColumnsBulk returned error: %v", err)
	}

	if columns["items"][1].IsNullable != true {
		t.Error("expected notes to be nullable")
	}
	if columns["items"][0].IsNullable != false {
		t.Error("expected id to be NOT nullable")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}
