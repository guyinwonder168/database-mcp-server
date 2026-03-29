package mcp

import (
	"context"
	"testing"
)

func TestQuoteSchemaName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "public schema",
			input:    "public",
			expected: `"public"`,
		},
		{
			name:     "snake_case schema",
			input:    "bitnami_redmine",
			expected: `"bitnami_redmine"`,
		},
		{
			name:     "mixed case schema",
			input:    "MySchema",
			expected: `"MySchema"`,
		},
		{
			name:     "schema with special chars",
			input:    `schema"name`,
			expected: `"schema""name"`,
		},
		{
			name:     "empty string",
			input:    "",
			expected: `""`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := QuoteSchemaName(tt.input)
			if got != tt.expected {
				t.Errorf("QuoteSchemaName(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestResolveSchema(t *testing.T) {
	tests := []struct {
		name     string
		schema   string
		expected string
	}{
		{
			name:     "non-empty schema",
			schema:   "public",
			expected: `"public"`,
		},
		{
			name:     "mixed case schema",
			schema:   "MySchema",
			expected: `"MySchema"`,
		},
		{
			name:     "schema with special chars",
			schema:   `schema"name`,
			expected: `"schema""name"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			got, err := ResolveSchema(ctx, nil, tt.schema)
			if err != nil {
				t.Fatalf("ResolveSchema(ctx, nil, %q) returned error: %v", tt.schema, err)
			}
			if got != tt.expected {
				t.Errorf("ResolveSchema(ctx, nil, %q) = %q, want %q", tt.schema, got, tt.expected)
			}
		})
	}
}
