package analyze

// relationships.go handles foreign key discovery and implicit relationship detection.
// BUG-006 fix: queries information_schema.KEY_COLUMN_USAGE for real FKs,
// reduces false positives from implicit column-name matching.
