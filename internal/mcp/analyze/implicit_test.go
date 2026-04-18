package analyze

import (
	"testing"
)

func TestDetectImplicitRelationships_ExactIDMatch(t *testing.T) {
	tables := map[string][]SchemaColumnInfo{
		"users": {
			{ColumnName: "id", IsPrimaryKey: true},
			{ColumnName: "name"},
		},
		"orders": {
			{ColumnName: "id", IsPrimaryKey: true},
			{ColumnName: "user_id"}, // exact: targetTable_id → high confidence
		},
	}

	rels := DetectImplicitRelationships(tables)
	if len(rels) != 1 {
		t.Fatalf("expected 1 implicit relationship, got %d", len(rels))
	}
	if rels[0].ConnectionBasis != "naming_convention:exact_id" {
		t.Errorf("expected naming_convention:exact_id, got %s", rels[0].ConnectionBasis)
	}
	if rels[0].ConfidenceScore < 0.7 {
		t.Errorf("expected confidence >= 0.7 for exact_id match, got %f", rels[0].ConfidenceScore)
	}
	if rels[0].Tables[0] != "orders" || rels[0].Tables[1] != "users" {
		t.Errorf("expected orders→users, got %s→%s", rels[0].Tables[0], rels[0].Tables[1])
	}
}

func TestDetectImplicitRelationships_SuffixIDMatch(t *testing.T) {
	tables := map[string][]SchemaColumnInfo{
		"products": {
			{ColumnName: "id", IsPrimaryKey: true},
		},
		"order_items": {
			{ColumnName: "id", IsPrimaryKey: true},
			{ColumnName: "fk_product_id"}, // compound prefix: "fk_product" contains "product"
		},
	}

	rels := DetectImplicitRelationships(tables)
	if len(rels) != 1 {
		t.Fatalf("expected 1 implicit relationship, got %d", len(rels))
	}
	if rels[0].ConnectionBasis != "naming_convention:suffix_id" {
		t.Errorf("expected naming_convention:suffix_id, got %s", rels[0].ConnectionBasis)
	}
	if rels[0].ConfidenceScore < 0.6 {
		t.Errorf("expected confidence >= 0.6 for suffix_id match, got %f", rels[0].ConfidenceScore)
	}
}

func TestDetectImplicitRelationships_CommonColumnsExcluded(t *testing.T) {
	tables := map[string][]SchemaColumnInfo{
		"users": {
			{ColumnName: "id", IsPrimaryKey: true},
			{ColumnName: "created"},    // common — should NOT create relationship
			{ColumnName: "name"},       // common — should NOT create relationship
			{ColumnName: "status"},     // common — should NOT create relationship
			{ColumnName: "updated_at"}, // common — should NOT create relationship
		},
		"orders": {
			{ColumnName: "id", IsPrimaryKey: true},
			{ColumnName: "created"},    // common — should NOT create relationship
			{ColumnName: "name"},       // common — should NOT create relationship
			{ColumnName: "status"},     // common — should NOT create relationship
			{ColumnName: "updated_at"}, // common — should NOT create relationship
		},
	}

	rels := DetectImplicitRelationships(tables)
	// Common columns should NOT generate implicit relationships
	for _, r := range rels {
		for _, col := range []string{"created", "name", "status", "updated_at"} {
			if r.FromColumn == col {
				t.Errorf("common column %q should not generate implicit relationship: %s.%s → %s.%s",
					col, r.Tables[0], r.FromColumn, r.Tables[1], r.ToColumn)
			}
		}
	}
	if len(rels) != 0 {
		t.Fatalf("expected 0 implicit relationships from common columns, got %d", len(rels))
	}
}

func TestDetectImplicitRelationships_MultipleRelationships(t *testing.T) {
	tables := map[string][]SchemaColumnInfo{
		"users":    {{ColumnName: "id", IsPrimaryKey: true}},
		"products": {{ColumnName: "id", IsPrimaryKey: true}},
		"orders": {
			{ColumnName: "id", IsPrimaryKey: true},
			{ColumnName: "user_id"},    // → users
			{ColumnName: "product_id"}, // → products
		},
	}

	rels := DetectImplicitRelationships(tables)
	if len(rels) != 2 {
		t.Fatalf("expected 2 implicit relationships, got %d", len(rels))
	}
}

func TestDetectImplicitRelationships_NoFalseIDMatch(t *testing.T) {
	tables := map[string][]SchemaColumnInfo{
		"categories": {
			{ColumnName: "id", IsPrimaryKey: true},
		},
		"products": {
			{ColumnName: "id", IsPrimaryKey: true},
			{ColumnName: "category_id"}, // should match categories, not anything else
		},
		"users": {
			{ColumnName: "id", IsPrimaryKey: true},
		},
	}

	rels := DetectImplicitRelationships(tables)
	if len(rels) != 1 {
		t.Fatalf("expected 1 implicit relationship, got %d", len(rels))
	}
	if rels[0].Tables[1] != "categories" {
		t.Errorf("expected relationship to categories, got %s", rels[0].Tables[1])
	}
}

func TestDetectImplicitRelationships_SingleTableNoRels(t *testing.T) {
	tables := map[string][]SchemaColumnInfo{
		"users": {
			{ColumnName: "id", IsPrimaryKey: true},
			{ColumnName: "name"},
		},
	}

	rels := DetectImplicitRelationships(tables)
	if len(rels) != 0 {
		t.Fatalf("expected 0 relationships for single table, got %d", len(rels))
	}
}

func TestIsCommonColumn(t *testing.T) {
	tests := []struct {
		col      string
		expected bool
	}{
		{"id", true},
		{"created", true},
		{"updated_at", true},
		{"name", true},
		{"status", true},
		{"user_id", false},
		{"product_id", false},
		{"category_id", false},
		{"email", false},
		{"price", false},
	}
	for _, tc := range tests {
		result := isCommonColumn(tc.col)
		if result != tc.expected {
			t.Errorf("isCommonColumn(%q) = %v, expected %v", tc.col, result, tc.expected)
		}
	}
}
