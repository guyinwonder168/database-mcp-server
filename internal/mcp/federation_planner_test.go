package mcp

import "testing"

func TestParseFederatedQuery(t *testing.T) {
	sqlText := "SELECT * FROM profile1.users u JOIN profile2.orders o ON u.id = o.user_id LIMIT 10 OFFSET 5"

	plan, err := ParseFederatedQuery(sqlText)
	if err != nil {
		t.Fatalf("expected parse success, got %v", err)
	}

	if len(plan.Tables) != 2 {
		t.Fatalf("expected 2 tables, got %d", len(plan.Tables))
	}
	if plan.Tables[0].Profile != "profile1" || plan.Tables[0].Table != "users" || plan.Tables[0].Alias != "u" {
		t.Fatalf("unexpected first table parse: %+v", plan.Tables[0])
	}
	if plan.Tables[1].Profile != "profile2" || plan.Tables[1].Table != "orders" || plan.Tables[1].Alias != "o" {
		t.Fatalf("unexpected second table parse: %+v", plan.Tables[1])
	}
	if len(plan.Joins) != 1 {
		t.Fatalf("expected one join, got %d", len(plan.Joins))
	}
	if plan.Joins[0].Left != "u.id" || plan.Joins[0].Right != "o.user_id" || plan.Joins[0].Type != FederationJoinInner {
		t.Fatalf("unexpected join parse: %+v", plan.Joins[0])
	}
	if plan.Limit != 10 || plan.Offset != 5 {
		t.Fatalf("unexpected limit/offset values: limit=%d offset=%d", plan.Limit, plan.Offset)
	}
}

func TestBuildSubQueries(t *testing.T) {
	plan := &FederatedQueryPlan{
		Tables: []FederatedTable{
			{Profile: "p1", Table: "users", Alias: "u"},
			{Profile: "p2", Table: "orders", Alias: "o"},
		},
	}

	subqueries, err := BuildSubQueries(plan, []string{"p1", "p2"})
	if err != nil {
		t.Fatalf("expected build success, got %v", err)
	}
	if len(subqueries) != 2 {
		t.Fatalf("expected 2 subqueries, got %d", len(subqueries))
	}
	if subqueries[0].SQL != "SELECT * FROM users" || subqueries[1].SQL != "SELECT * FROM orders" {
		t.Fatalf("unexpected generated SQL: %+v", subqueries)
	}

	_, err = BuildSubQueries(plan, []string{"p1"})
	if err == nil {
		t.Fatalf("expected profile validation error")
	}
}

func TestOptimizeFederationPlanAndCost(t *testing.T) {
	plan := &FederatedQueryPlan{
		SubQueries: []SubQuery{
			{Profile: "p2", Alias: "o", SQL: "SELECT * FROM orders"},
			{Profile: "p1", Alias: "u", SQL: "SELECT * FROM users"},
		},
		Joins: []JoinCondition{
			{Left: "u.id", Right: "o.user_id", Type: "left"},
		},
	}

	optimized := OptimizeFederationPlan(plan)
	if optimized.Limit != defaultFederationLimit {
		t.Fatalf("expected default limit %d, got %d", defaultFederationLimit, optimized.Limit)
	}
	if optimized.MaxConcurrency <= 0 {
		t.Fatalf("expected positive max concurrency")
	}
	if optimized.SubQueries[0].Profile != "p1" {
		t.Fatalf("expected deterministic subquery ordering")
	}
	if optimized.Joins[0].Type != FederationJoinLeft {
		t.Fatalf("expected normalized join type, got %s", optimized.Joins[0].Type)
	}

	cost := EstimateFederationCost(optimized)
	if cost.SubQueriesCount != 2 || cost.JoinOperationsCount != 1 {
		t.Fatalf("unexpected cost metadata: %+v", cost)
	}
	if cost.EstimatedRows <= 0 || cost.EstimatedTimeMs <= 0 {
		t.Fatalf("expected positive cost estimate: %+v", cost)
	}
}

func TestDetermineExecutionOrder(t *testing.T) {
	subqueries := []SubQuery{
		{Profile: "p1", Alias: "u"},
		{Profile: "p2", Alias: "o"},
	}
	joins := []JoinCondition{{Left: "u.id", Right: "o.user_id", Type: FederationJoinInner}}

	steps := DetermineExecutionOrder(subqueries, joins)
	if len(steps) != 2 {
		t.Fatalf("expected 2 execution steps, got %d", len(steps))
	}
	if len(steps[1].DependsOn) != 1 || steps[1].DependsOn[0] != "u" {
		t.Fatalf("expected right side dependency on u, got %+v", steps[1])
	}
}
