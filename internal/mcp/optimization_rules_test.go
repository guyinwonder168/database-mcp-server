package mcp

import "testing"

func TestMissingIndexRule(t *testing.T) {
	plan := &ExplainPlan{
		DBType: "postgres",
		Steps: []PlanStep{
			{
				Operation: "Seq Scan",
				Target:    "users",
				Detail:    "users",
				Extra: map[string]interface{}{
					"filter": "id > 10",
				},
			},
		},
	}
	engine := NewDefaultOptimizationRuleEngine()
	findings := engine.Evaluate(plan)
	if len(findings) == 0 {
		t.Fatalf("expected missing_index finding")
	}
	found := false
	for _, f := range findings {
		if f.Rule == "missing_index" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("missing_index rule not triggered")
	}
}

func TestMissingIndexRuleSkippedWhenIndexPresent(t *testing.T) {
	plan := &ExplainPlan{
		DBType: "postgres",
		Steps: []PlanStep{
			{
				Operation: "Index Scan",
				Target:    "users",
				Detail:    "users using idx_users_id",
				Extra: map[string]interface{}{
					"index_name": "idx_users_id",
				},
			},
		},
	}
	engine := NewDefaultOptimizationRuleEngine()
	findings := engine.Evaluate(plan)
	for _, f := range findings {
		if f.Rule == "missing_index" {
			t.Fatalf("missing_index should not trigger when index is used")
		}
	}
}

func TestInefficientJoinRule(t *testing.T) {
	plan := &ExplainPlan{
		DBType: "mysql",
		Steps: []PlanStep{
			{Operation: "table", Target: "orders", Detail: "ALL"},
			{Operation: "table", Target: "users", Detail: "ALL"},
		},
	}
	engine := NewDefaultOptimizationRuleEngine()
	findings := engine.Evaluate(plan)
	if len(findings) == 0 {
		t.Fatalf("expected inefficient_join finding")
	}
	found := false
	for _, f := range findings {
		if f.Rule == "inefficient_join" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("inefficient_join rule not triggered")
	}
}

func TestSQLiteFullScanRule(t *testing.T) {
	plan := &ExplainPlan{
		DBType: "sqlite",
		Steps: []PlanStep{
			{Operation: "SCAN", Target: "products", Detail: "SCAN TABLE products"},
		},
	}
	engine := NewDefaultOptimizationRuleEngine()
	findings := engine.Evaluate(plan)
	if len(findings) == 0 {
		t.Fatalf("expected sqlite_full_scan finding")
	}
	if findings[0].Rule != "sqlite_full_scan" {
		t.Fatalf("expected sqlite_full_scan rule, got %s", findings[0].Rule)
	}
}
