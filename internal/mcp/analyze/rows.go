package analyze

// rows.go handles row count queries for all backends.
// BUG-005 fix: populates TableInfo.RowCount from information_schema.TABLES / pg_stat_user_tables / COUNT(*).
