package mcp

import "testing"

func TestNormalizeJoinType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "default_inner", input: "", expected: FederationJoinInner},
		{name: "left", input: "left", expected: FederationJoinLeft},
		{name: "right", input: "RIGHT", expected: FederationJoinRight},
		{name: "full", input: "full", expected: FederationJoinFull},
		{name: "unknown", input: "cross", expected: "CROSS"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := normalizeJoinType(tt.input)
			if actual != tt.expected {
				t.Fatalf("expected %s, got %s", tt.expected, actual)
			}
		})
	}
}

func TestValidateJoinCondition(t *testing.T) {
	if err := validateJoinCondition(JoinCondition{
		Left:  "u.id",
		Right: "o.user_id",
		Type:  "INNER",
	}); err != nil {
		t.Fatalf("expected valid join, got %v", err)
	}

	if err := validateJoinCondition(JoinCondition{
		Left:  "",
		Right: "o.user_id",
		Type:  "INNER",
	}); err == nil {
		t.Fatalf("expected error for missing join left expression")
	}

	if err := validateJoinCondition(JoinCondition{
		Left:  "u.id",
		Right: "o.user_id",
		Type:  "CROSS",
	}); err == nil {
		t.Fatalf("expected error for unsupported join type")
	}
}
