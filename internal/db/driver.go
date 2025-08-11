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
	driverName := profileType
	switch profileType {
	case "mariadb":
		driverName = "mysql"
	case "sqlite":
		driverName = "sqlite3"
	}
	db, err := sql.Open(driverName, dsn)
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

func DSN(profileType, host string, port int, user, pass, dbname, sslmode string) string {
	switch profileType {
	case "mysql", "mariadb":
		// (Optional) add TLS parameters here in future if needed
		return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", user, pass, host, port, dbname)
	case "postgres":
		mode := sslmode
		if mode == "" {
			mode = "require" // secure default
		}
		return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s", host, port, user, pass, dbname, mode)
	case "sqlite":
		return dbname
	default:
		return ""
	}
}
