package mcp

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/lib/pq"
)

// QuoteSchemaName quotes a schema name using PostgreSQL identifier quoting rules.
func QuoteSchemaName(schema string) string {
	return pq.QuoteIdentifier(schema)
}

// GetDefaultSchema retrieves the default schema for the current database connection.
// It tries current_schema() first, then queries information_schema.schemata for the first accessible schema,
// and finally falls back to "public".
func GetDefaultSchema(ctx context.Context, conn *sql.Conn) (string, error) {
	// Try to get current schema
	var schema sql.NullString
	err := conn.QueryRowContext(ctx, "SELECT current_schema()").Scan(&schema)
	if err != nil {
		return "", fmt.Errorf("failed to get current schema: %w", err)
	}

	// If we got a non-null, non-empty schema, use it
	if schema.Valid && schema.String != "" {
		return schema.String, nil
	}

	// Query information_schema.schemata for first accessible schema (excluding pg_* and information_schema)
	var foundSchema sql.NullString
	err = conn.QueryRowContext(ctx,
		`SELECT schema_name FROM information_schema.schemata 
		 WHERE schema_name NOT LIKE 'pg_%' AND schema_name != 'information_schema' 
		 LIMIT 1`).Scan(&foundSchema)
	if err != nil && err != sql.ErrNoRows {
		return "", fmt.Errorf("failed to query information_schema.schemata: %w", err)
	}

	// If we found a schema in information_schema, use it
	if foundSchema.Valid && foundSchema.String != "" {
		return foundSchema.String, nil
	}

	// Final fallback to "public"
	return "public", nil
}

// ResolveSchema returns a quoted schema name. If schema is empty, it calls GetDefaultSchema
// to determine the default schema for the connection.
func ResolveSchema(ctx context.Context, conn *sql.Conn, schema string) (string, error) {
	if schema != "" {
		return QuoteSchemaName(schema), nil
	}

	// Get default schema and quote it
	defaultSchema, err := GetDefaultSchema(ctx, conn)
	if err != nil {
		return "", fmt.Errorf("failed to resolve schema: %w", err)
	}

	return QuoteSchemaName(defaultSchema), nil
}
