package analyze

// columns.go handles column metadata queries for MySQL/MariaDB, PostgreSQL, and SQLite.
// BUG-004 fix: uses bulk information_schema.COLUMNS queries instead of per-table SHOW COLUMNS.
