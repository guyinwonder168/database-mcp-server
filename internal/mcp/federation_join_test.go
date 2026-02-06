package mcp

import (
	"math"
	"testing"
)

func TestPerformJoin(t *testing.T) {
	left := &SubQueryResult{
		Columns: []string{"id", "name"},
		Rows:    []Row{{1, "Alice"}, {2, "Bob"}},
	}
	right := &SubQueryResult{
		Columns: []string{"user_id", "total"},
		Rows:    []Row{{1, 100}, {1, 200}, {3, 50}},
	}

	joined, err := PerformJoin(left, right, JoinCondition{
		Left:  "id",
		Right: "user_id",
		Type:  FederationJoinInner,
	})
	if err != nil {
		t.Fatalf("expected join success, got %v", err)
	}

	if len(joined.Columns) != 4 {
		t.Fatalf("expected 4 joined columns, got %d", len(joined.Columns))
	}
	if len(joined.Rows) != 2 {
		t.Fatalf("expected 2 joined rows, got %d", len(joined.Rows))
	}
	if joined.Rows[0][1] != "Alice" || joined.Rows[1][3] != 200 {
		t.Fatalf("unexpected join rows: %+v", joined.Rows)
	}
}

func TestPerformJoinLeftAndRight(t *testing.T) {
	left := &SubQueryResult{
		Columns: []string{"id", "name"},
		Rows:    []Row{{1, "Alice"}, {2, "Bob"}},
	}
	right := &SubQueryResult{
		Columns: []string{"user_id", "total"},
		Rows:    []Row{{1, 100}, {3, 50}},
	}

	leftJoin, err := PerformJoin(left, right, JoinCondition{
		Left:  "id",
		Right: "user_id",
		Type:  FederationJoinLeft,
	})
	if err != nil {
		t.Fatalf("expected left join success, got %v", err)
	}
	if len(leftJoin.Rows) != 2 {
		t.Fatalf("expected 2 rows for left join, got %d", len(leftJoin.Rows))
	}
	if leftJoin.Rows[1][2] != nil {
		t.Fatalf("expected nil right payload for unmatched left row: %+v", leftJoin.Rows[1])
	}

	rightJoin, err := PerformJoin(left, right, JoinCondition{
		Left:  "id",
		Right: "user_id",
		Type:  FederationJoinRight,
	})
	if err != nil {
		t.Fatalf("expected right join success, got %v", err)
	}
	if len(rightJoin.Rows) != 2 {
		t.Fatalf("expected 2 rows for right join, got %d", len(rightJoin.Rows))
	}
	if rightJoin.Rows[1][0] != nil || rightJoin.Rows[1][2] != 3 {
		t.Fatalf("expected unmatched right row with nil left payload: %+v", rightJoin.Rows[1])
	}
}

func TestNormalizeDataTypes(t *testing.T) {
	results := []SubQueryResult{
		{
			Columns: []string{"id", "payload"},
			Rows: []Row{
				{1, []byte("alpha")},
				{2, []byte("beta")},
			},
		},
	}

	normalized := NormalizeDataTypes(results)
	if normalized[0].Rows[0][0] != int64(1) {
		t.Fatalf("expected int -> int64 normalization, got %T", normalized[0].Rows[0][0])
	}
	if normalized[0].Rows[0][1] != "alpha" {
		t.Fatalf("expected []byte -> string normalization")
	}
}

func TestAggregateResults(t *testing.T) {
	results := []SubQueryResult{
		{
			Columns: []string{"user_id", "total"},
			Rows:    []Row{{1, 100}, {2, 50}, {3, 25}},
		},
	}

	aggregated, err := AggregateResults(results, []Aggregation{
		{Function: "COUNT", Column: "*", Alias: "rows"},
	})
	if err != nil {
		t.Fatalf("expected aggregation success, got %v", err)
	}
	if len(aggregated.Rows) != 1 || aggregated.Rows[0][0] != 3 {
		t.Fatalf("unexpected count aggregation output: %+v", aggregated.Rows)
	}

	sum, err := AggregateResults(results, []Aggregation{
		{Function: "SUM", Column: "total", Alias: "sum_total"},
	})
	if err != nil {
		t.Fatalf("expected SUM aggregation success, got %v", err)
	}
	if sum.Rows[0][0] != float64(175) {
		t.Fatalf("unexpected SUM value: %+v", sum.Rows[0][0])
	}
}

func TestUnionWithoutJoin(t *testing.T) {
	aligned := unionWithoutJoin([]SubQueryResult{
		{Profile: "p1", Alias: "u", Columns: []string{"id"}, Rows: []Row{{1}}},
		{Profile: "p2", Alias: "o", Columns: []string{"id"}, Rows: []Row{{2}}},
	})
	if len(aligned.Columns) != 1 || aligned.Columns[0] != "id" {
		t.Fatalf("expected aligned columns passthrough, got %+v", aligned.Columns)
	}
	if len(aligned.Rows) != 2 {
		t.Fatalf("expected merged rows for aligned columns, got %d", len(aligned.Rows))
	}

	serialized := unionWithoutJoin([]SubQueryResult{
		{Profile: "p1", Alias: "u", Columns: []string{"id"}, Rows: []Row{{1}}},
		{Profile: "p2", Alias: "o", Columns: []string{"user_id"}, Rows: []Row{{2}}},
	})
	if len(serialized.Columns) != 3 || serialized.Columns[2] != federationSerializedRowColumn {
		t.Fatalf("expected serialized fallback columns, got %+v", serialized.Columns)
	}
	if len(serialized.Rows) != 2 {
		t.Fatalf("expected serialized rows, got %d", len(serialized.Rows))
	}
}

func TestIsFederationReadOnlySQL(t *testing.T) {
	if !isFederationReadOnlySQL("SELECT * FROM users") {
		t.Fatalf("expected SELECT to be read-only")
	}
	if !isFederationReadOnlySQL("WITH cte AS (SELECT 1) SELECT * FROM cte") {
		t.Fatalf("expected CTE SELECT to be read-only")
	}
	if isFederationReadOnlySQL("SELECT 1; DROP TABLE users") {
		t.Fatalf("expected multi-statement query to be rejected")
	}
	if isFederationReadOnlySQL("DELETE FROM users") {
		t.Fatalf("expected DELETE to be rejected")
	}
}

func TestNumericHelpers(t *testing.T) {
	overflow := normalizeDataValue(uint64(math.MaxInt64) + 1)
	if _, ok := overflow.(float64); !ok {
		t.Fatalf("expected uint64 overflow to normalize as float64, got %T", overflow)
	}

	if v, ok := federationToFloat64("12.5"); !ok || v != 12.5 {
		t.Fatalf("expected parseable numeric string, got v=%v ok=%v", v, ok)
	}
	if _, ok := federationToFloat64("bad"); ok {
		t.Fatalf("expected invalid numeric string to fail conversion")
	}

	row := Row{nil, 5}
	if aggregationCount([]Row{row}, 0) != 0 {
		t.Fatalf("expected nil value to be excluded from column COUNT")
	}
}
