//go:build cgo

package mcp

import (
	"testing"
)

func TestBuildExplainStatement(t *testing.T) {
	tests := []struct {
		dbType string
		sql    string
		want   string
	}{
		{"mysql", "SELECT * FROM users", "EXPLAIN FORMAT=JSON SELECT * FROM users"},
		{"mariadb", "select 1", "EXPLAIN FORMAT=JSON select 1"},
		{"postgres", "SELECT 1", "EXPLAIN (FORMAT JSON) SELECT 1"},
		{"sqlite", "SELECT 1", "EXPLAIN QUERY PLAN SELECT 1"},
	}
	for _, tc := range tests {
		got, err := buildExplainStatement(tc.dbType, tc.sql)
		if err != nil {
			t.Fatalf("buildExplainStatement(%s) returned error: %v", tc.dbType, err)
		}
		if got != tc.want {
			t.Fatalf("buildExplainStatement(%s) = %s, want %s", tc.dbType, got, tc.want)
		}
	}
	if _, err := buildExplainStatement("oracle", "select 1"); err == nil {
		t.Fatalf("expected error for unsupported db_type")
	}
}

func TestParsePostgresPlanJSON(t *testing.T) {
	raw := `[{"Plan":{"Node Type":"Seq Scan","Relation Name":"users","Alias":"users","Startup Cost":0.00,"Total Cost":11.35,"Plan Rows":10}}]`
	plan, err := parseExplainRows("postgres", []string{"QUERY PLAN"}, [][]interface{}{{raw}})
	if err != nil {
		t.Fatalf("parseExplainRows postgres error: %v", err)
	}
	if len(plan.Steps) == 0 {
		t.Fatalf("expected steps in plan")
	}
	first := plan.Steps[0]
	if first.Operation != "Seq Scan" {
		t.Fatalf("expected operation Seq Scan, got %s", first.Operation)
	}
	if first.Target != "users" {
		t.Fatalf("expected target users, got %s", first.Target)
	}
	if first.EstimatedRows != 10 {
		t.Fatalf("expected rows 10, got %v", first.EstimatedRows)
	}
	if first.EstimatedCost != 11.35 {
		t.Fatalf("expected cost 11.35, got %v", first.EstimatedCost)
	}
}

func TestParseMySQLPlanJSON(t *testing.T) {
	raw := `{"query_block":{"select_id":1,"table":{"table_name":"users","access_type":"ALL","rows_examined_per_scan":10,"attached_condition":"` + "`id`" + ` < 100"},"cost_info":{"query_cost":"1.20"}}}`
	plan, err := parseExplainRows("mysql", []string{"EXPLAIN"}, [][]interface{}{{raw}})
	if err != nil {
		t.Fatalf("parseExplainRows mysql error: %v", err)
	}
	if len(plan.Steps) == 0 {
		t.Fatalf("expected steps in plan")
	}
	foundTable := false
	for _, step := range plan.Steps {
		if step.Target == "users" {
			foundTable = true
			if step.Detail == "" {
				t.Fatalf("expected access type detail for table step")
			}
		}
	}
	if !foundTable {
		t.Fatalf("did not find table step for users")
	}
}

func TestParseSQLitePlan(t *testing.T) {
	columns := []string{"selectid", "order", "from", "detail"}
	rows := [][]interface{}{
		{int64(0), int64(0), int64(0), "SCAN TABLE users"},
	}
	plan, err := parseExplainRows("sqlite", columns, rows)
	if err != nil {
		t.Fatalf("parseExplainRows sqlite error: %v", err)
	}
	if len(plan.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(plan.Steps))
	}
	step := plan.Steps[0]
	if step.Operation != "SCAN" {
		t.Fatalf("expected operation SCAN, got %s", step.Operation)
	}
	if step.Target != "users" {
		t.Fatalf("expected target users, got %s", step.Target)
	}
}
