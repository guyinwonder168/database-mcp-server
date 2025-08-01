// driver.go
package db

import (
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

// OpenConnection opens a DB connection and applies pooling config if provided.
func OpenConnectionWithPool(profileType, dsn string, maxPoolSize int) (*sql.DB, error) {
	db, err := sql.Open(profileType, dsn)
	if err != nil {
		return nil, err
	}
	if maxPoolSize > 0 {
		db.SetMaxOpenConns(maxPoolSize)
		db.SetMaxIdleConns(maxPoolSize / 2)
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	return db, nil
}

// Legacy for compatibility (will be removed)
func OpenConnection(profileType, dsn string) (*sql.DB, error) {
	return OpenConnectionWithPool(profileType, dsn, 0)
}

func DSN(profileType, host string, port int, user, pass, dbname string) string {
	switch profileType {
	case "mysql", "mariadb":
		return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", user, pass, host, port, dbname)
	case "postgres":
		return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", host, port, user, pass, dbname)
	case "sqlite":
		return dbname
	default:
		return ""
	}
}
