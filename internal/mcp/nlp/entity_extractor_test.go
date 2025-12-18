package nlp

import "testing"

func TestExtractEntitiesSQL(t *testing.T) {
	res := ExtractEntities("SELECT id, name FROM users u JOIN orders o ON u.id = o.user_id")
	if len(res.Tables) == 0 {
		t.Fatalf("expected at least one table")
	}
	if res.Tables[0] != "users" && res.Tables[0] != "orders" {
		t.Fatalf("unexpected table: %v", res.Tables)
	}
}

func TestExtractEntitiesFallback(t *testing.T) {
	res := ExtractEntities("show me orders and customers totals")
	if len(res.Tables) == 0 {
		t.Fatalf("expected fallback tables")
	}
}
