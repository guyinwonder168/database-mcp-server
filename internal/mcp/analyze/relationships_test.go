package analyze

import (
	"context"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestDiscoverForeignKeys_MySQL(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{
		"TABLE_NAME", "COLUMN_NAME", "REFERENCED_TABLE_NAME", "REFERENCED_COLUMN_NAME",
	}).
		AddRow("orders", "user_id", "users", "id").
		AddRow("orders", "product_id", "products", "id").
		AddRow("order_items", "order_id", "orders", "id")

	mock.ExpectQuery(`SELECT TABLE_NAME, COLUMN_NAME, REFERENCED_TABLE_NAME, REFERENCED_COLUMN_NAME FROM information_schema\.KEY_COLUMN_USAGE WHERE TABLE_SCHEMA = \? AND REFERENCED_TABLE_NAME IS NOT NULL`).
		WithArgs("mydb").
		WillReturnRows(rows)

	fks, err := DiscoverForeignKeys(context.Background(), db, "mysql", "mydb")
	if err != nil {
		t.Fatalf("DiscoverForeignKeys returned error: %v", err)
	}
	if len(fks) != 3 {
		t.Fatalf("expected 3 FKs, got %d", len(fks))
	}
	if fks[0].FromTable != "orders" || fks[0].FromColumn != "user_id" {
		t.Errorf("expected orders.user_id, got %s.%s", fks[0].FromTable, fks[0].FromColumn)
	}
	if fks[0].ToTable != "users" || fks[0].ToColumn != "id" {
		t.Errorf("expected users.id, got %s.%s", fks[0].ToTable, fks[0].ToColumn)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestDiscoverForeignKeys_PostgreSQL(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{
		"table_name", "column_name", "referenced_table_name", "referenced_column_name",
	}).
		AddRow("orders", "user_id", "users", "id").
		AddRow("reviews", "product_id", "products", "id")

	mock.ExpectQuery(`SELECT tc\.table_name.*constraint_column_usage ccu.*WHERE tc\.constraint_type = 'FOREIGN KEY' AND tc\.table_schema = \$1`).
		WithArgs("public").
		WillReturnRows(rows)

	fks, err := DiscoverForeignKeys(context.Background(), db, "postgres", "public")
	if err != nil {
		t.Fatalf("DiscoverForeignKeys returned error: %v", err)
	}
	if len(fks) != 2 {
		t.Fatalf("expected 2 FKs, got %d", len(fks))
	}
	if fks[1].FromTable != "reviews" {
		t.Errorf("expected reviews, got %s", fks[1].FromTable)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestDiscoverForeignKeys_SQLite(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	// SQLite: TVF pragma_foreign_key_list with bind parameter
	ordersFK := sqlmock.NewRows([]string{"id", "seq", "table", "from", "to", "on_update", "on_delete", "match"}).
		AddRow(0, 0, "users", "user_id", "id", "NO ACTION", "NO ACTION", "NONE")
	mock.ExpectQuery(`SELECT.*FROM pragma_foreign_key_list WHERE arg = \?`).WithArgs("orders").WillReturnRows(ordersFK)

	// users table has no FKs
	usersFK := sqlmock.NewRows([]string{"id", "seq", "table", "from", "to", "on_update", "on_delete", "match"})
	mock.ExpectQuery(`SELECT.*FROM pragma_foreign_key_list WHERE arg = \?`).WithArgs("users").WillReturnRows(usersFK)

	fks, err := DiscoverForeignKeys(context.Background(), db, "sqlite", "", []string{"orders", "users"})
	if err != nil {
		t.Fatalf("DiscoverForeignKeys returned error: %v", err)
	}
	if len(fks) != 1 {
		t.Fatalf("expected 1 FK, got %d", len(fks))
	}
	if fks[0].FromTable != "orders" || fks[0].FromColumn != "user_id" {
		t.Errorf("expected orders.user_id, got %s.%s", fks[0].FromTable, fks[0].FromColumn)
	}
	if fks[0].ToTable != "users" || fks[0].ToColumn != "id" {
		t.Errorf("expected users.id, got %s.%s", fks[0].ToTable, fks[0].ToColumn)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestDiscoverForeignKeys_NoFKs(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{
		"TABLE_NAME", "COLUMN_NAME", "REFERENCED_TABLE_NAME", "REFERENCED_COLUMN_NAME",
	})
	mock.ExpectQuery(`SELECT.*information_schema\.KEY_COLUMN_USAGE.*`).
		WithArgs("mydb").
		WillReturnRows(rows)

	fks, err := DiscoverForeignKeys(context.Background(), db, "mysql", "mydb")
	if err != nil {
		t.Fatalf("DiscoverForeignKeys returned error: %v", err)
	}
	if len(fks) != 0 {
		t.Fatalf("expected 0 FKs, got %d", len(fks))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestDiscoverForeignKeys_UnsupportedDB(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	_, err = DiscoverForeignKeys(context.Background(), db, "oracle", "somedb")
	if err == nil {
		t.Fatal("expected error for unsupported db type, got nil")
	}
}
