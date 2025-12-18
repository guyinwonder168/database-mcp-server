package mcp

import "testing"

func TestEstimatePerformanceMissingIndex(t *testing.T) {
	plan := &ExplainPlan{
		DBType: "postgres",
		Steps: []PlanStep{
			{Operation: "Seq Scan", Target: "users", EstimatedCost: 10, EstimatedRows: 100},
		},
	}
	findings := []OptimizationFinding{
		{Rule: "missing_index", Suggestion: "add index"},
	}
	est := EstimatePerformance(plan, findings)
	if est.Improvement.UpperPercent < 50 {
		t.Fatalf("expected high improvement upper bound, got %.2f", est.Improvement.UpperPercent)
	}
	if est.Confidence < 0.7 {
		t.Fatalf("expected confidence >= 0.7, got %.2f", est.Confidence)
	}
}

func TestEstimatePerformanceNoCost(t *testing.T) {
	plan := &ExplainPlan{DBType: "sqlite", Steps: []PlanStep{{Operation: "SCAN", Target: "products"}}}
	est := EstimatePerformance(plan, nil)
	if est.BaselineCost != 0 {
		t.Fatalf("expected baseline cost 0, got %.2f", est.BaselineCost)
	}
	if est.Improvement.UpperPercent == 0 {
		t.Fatalf("expected non-zero default improvement range")
	}
}

func BenchmarkEstimatePerformance(b *testing.B) {
	plan := &ExplainPlan{
		DBType: "postgres",
		Steps: []PlanStep{
			{Operation: "Seq Scan", Target: "users", EstimatedCost: 10, EstimatedRows: 100},
			{Operation: "Hash Join", Target: "orders", EstimatedCost: 20, EstimatedRows: 200},
		},
	}
	findings := []OptimizationFinding{{Rule: "inefficient_join", Suggestion: "add join indexes"}}
	for i := 0; i < b.N; i++ {
		_ = EstimatePerformance(plan, findings)
	}
}
