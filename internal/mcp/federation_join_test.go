package mcp

import (
	"context"
	"database-mcp-provider/internal/config"
	"math"
	"testing"
	"time"
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

func TestAggregateResultsAdditionalPaths(t *testing.T) {
	results := []SubQueryResult{
		{
			Columns: []string{"total", "label"},
			Rows:    []Row{{10, "a"}, {20, "b"}, {nil, "c"}, {"n/a", "d"}},
		},
	}

	avg, err := AggregateResults(results, []Aggregation{{Function: "AVG", Column: "total", Alias: "avg_total"}})
	if err != nil {
		t.Fatalf("expected AVG aggregation success, got %v", err)
	}
	if avg.Rows[0][0] != float64(15) {
		t.Fatalf("unexpected AVG result: %+v", avg.Rows)
	}

	minimum, err := AggregateResults(results, []Aggregation{{Function: "MIN", Column: "total", Alias: "min_total"}})
	if err != nil {
		t.Fatalf("expected MIN aggregation success, got %v", err)
	}
	if minimum.Rows[0][0] != float64(10) {
		t.Fatalf("unexpected MIN result: %+v", minimum.Rows)
	}

	maximum, err := AggregateResults(results, []Aggregation{{Function: "MAX", Column: "total", Alias: "max_total"}})
	if err != nil {
		t.Fatalf("expected MAX aggregation success, got %v", err)
	}
	if maximum.Rows[0][0] != float64(20) {
		t.Fatalf("unexpected MAX result: %+v", maximum.Rows)
	}

	countDefaultAlias, err := AggregateResults(results, []Aggregation{{Function: "COUNT", Column: "*"}})
	if err != nil {
		t.Fatalf("expected COUNT aggregation success, got %v", err)
	}
	if len(countDefaultAlias.Columns) != 1 || countDefaultAlias.Columns[0] != "count" {
		t.Fatalf("expected default alias 'count', got %+v", countDefaultAlias.Columns)
	}

	noNumericValues, err := AggregateResults(results, []Aggregation{{Function: "SUM", Column: "label", Alias: "sum_label"}})
	if err != nil {
		t.Fatalf("expected SUM over non-numeric values to return 0, got %v", err)
	}
	if noNumericValues.Rows[0][0] != 0 {
		t.Fatalf("expected zero SUM for non-numeric inputs, got %+v", noNumericValues.Rows)
	}

	if _, err := AggregateResults(nil, []Aggregation{{Function: "COUNT", Column: "*"}}); err == nil {
		t.Fatalf("expected empty result validation error")
	}
	if _, err := AggregateResults(results, []Aggregation{{Function: "", Column: "*"}}); err == nil {
		t.Fatalf("expected missing-function validation error")
	}
	if _, err := AggregateResults(results, []Aggregation{{Function: "COUNT", Column: "*", GroupBy: []string{"label"}}}); err == nil {
		t.Fatalf("expected unsupported group_by error")
	}
	if _, err := AggregateResults(results, []Aggregation{{Function: "SUM", Column: "missing"}}); err == nil {
		t.Fatalf("expected missing column error")
	}
	if _, err := AggregateResults(results, []Aggregation{{Function: "SUM", Column: "*"}}); err == nil {
		t.Fatalf("expected numeric-column-required error for SUM(*)")
	}
	if _, err := AggregateResults(results, []Aggregation{{Function: "MEDIAN", Column: "total"}}); err == nil {
		t.Fatalf("expected unsupported function error")
	}
}

func TestNormalizeDataValueAdditionalTypes(t *testing.T) {
	now := time.Date(2026, 2, 6, 12, 34, 56, 0, time.FixedZone("UTC+7", 7*60*60))
	normalizedTime, ok := normalizeDataValue(now).(string)
	if !ok {
		t.Fatalf("expected time normalization to string, got %T", normalizeDataValue(now))
	}
	if normalizedTime != now.UTC().Format(time.RFC3339Nano) {
		t.Fatalf("unexpected normalized time value: %s", normalizedTime)
	}

	if normalizeDataValue(uint8(7)) != int64(7) {
		t.Fatalf("expected uint8 normalization to int64")
	}
	if normalizeDataValue(uint32(9)) != int64(9) {
		t.Fatalf("expected uint32 normalization to int64")
	}
	if normalizeDataValue(float32(1.25)) != float64(1.25) {
		t.Fatalf("expected float32 normalization to float64")
	}

	type customValue struct{ Label string }
	input := customValue{Label: "x"}
	if normalizeDataValue(input) != input {
		t.Fatalf("expected custom type passthrough")
	}

	if rowValue(Row{1}, 2) != nil {
		t.Fatalf("expected out-of-range row value to be nil")
	}
	if findColumnIndex([]string{"id"}, "missing") != -1 {
		t.Fatalf("expected findColumnIndex miss to return -1")
	}
	if normalizeJoinKey(nil) != "__nil__" {
		t.Fatalf("expected nil join key sentinel")
	}
	if columnFromQualified("id") != "id" {
		t.Fatalf("expected unqualified column passthrough")
	}
}

func TestExecuteSubQueryAndJoinValidationErrors(t *testing.T) {
	ctx := context.Background()
	profile := config.Profile{
		ProfileName:  "p1",
		DBType:       "sqlite",
		DatabaseName: ":memory:",
	}

	if _, err := ExecuteSubQuery(ctx, SubQuery{Profile: "p1", Alias: "u", SQL: ""}, profile); err == nil {
		t.Fatalf("expected empty SQL validation error")
	}
	if _, err := ExecuteSubQuery(ctx, SubQuery{Profile: "p1", Alias: "u", SQL: "DELETE FROM users"}, profile); err == nil {
		t.Fatalf("expected read-only SQL validation error")
	}

	if _, err := ExecuteSubQuery(ctx, SubQuery{Profile: "p1", Alias: "u", SQL: "SELECT * FROM missing_table"}, profile); err == nil {
		t.Fatalf("expected query error for missing sqlite table")
	}

	if _, err := ExecuteSubQuery(ctx, SubQuery{Profile: "p1", Alias: "u", SQL: "SELECT 1"}, config.Profile{DBType: "invalid"}); err == nil {
		t.Fatalf("expected open connection error for invalid profile type")
	}

	if _, err := PerformHashJoin(nil, &SubQueryResult{}, JoinCondition{Left: "id", Right: "id", Type: FederationJoinInner}); err == nil {
		t.Fatalf("expected nil-side validation error")
	}
	if _, err := PerformHashJoin(
		&SubQueryResult{Columns: []string{"id"}, Rows: []Row{{1}}},
		&SubQueryResult{Columns: []string{"id"}, Rows: []Row{{1}}},
		JoinCondition{Left: "id", Right: "id", Type: "CROSS"},
	); err == nil {
		t.Fatalf("expected join type validation error")
	}
}

func TestFederationJoinUtilityEdgeCases(t *testing.T) {
	if _, err := AggregateResults([]SubQueryResult{
		{Columns: []string{"id"}, Rows: []Row{{1}}},
	}, nil); err != nil {
		t.Fatalf("expected passthrough for empty aggregations, got %v", err)
	}

	if isFederationReadOnlySQL("") {
		t.Fatalf("expected empty SQL to be rejected")
	}
	if !isFederationReadOnlySQL("SELECT 1;") {
		t.Fatalf("expected single trailing semicolon to be accepted")
	}
	if isFederationReadOnlySQL("SELECT * FROM users UPDATE users SET name='x'") {
		t.Fatalf("expected disallowed token detection for UPDATE")
	}

	minResult, err := AggregateResults([]SubQueryResult{
		{Columns: []string{"total"}, Rows: []Row{{20}, {10}}},
	}, []Aggregation{{Function: "MIN", Column: "total", Alias: "min_total"}})
	if err != nil {
		t.Fatalf("expected MIN aggregation success, got %v", err)
	}
	if minResult.Rows[0][0] != float64(10) {
		t.Fatalf("unexpected MIN value: %+v", minResult.Rows)
	}

	if aggregationCount([]Row{{1}, {nil}}, 0) != 1 {
		t.Fatalf("expected aggregationCount to count only non-nil column values")
	}

	if normalizeDataValue(int8(1)) != int64(1) {
		t.Fatalf("expected int8 normalization to int64")
	}
	if normalizeDataValue(int16(2)) != int64(2) {
		t.Fatalf("expected int16 normalization to int64")
	}
	if normalizeDataValue(int32(3)) != int64(3) {
		t.Fatalf("expected int32 normalization to int64")
	}
	if normalizeDataValue(uint(4)) != int64(4) {
		t.Fatalf("expected uint normalization to int64")
	}
	if normalizeDataValue(uint16(5)) != int64(5) {
		t.Fatalf("expected uint16 normalization to int64")
	}
	if normalizeDataValue(float64(6.5)) != float64(6.5) {
		t.Fatalf("expected float64 passthrough")
	}

	if parsed, ok := federationToFloat64(float64(7.5)); !ok || parsed != 7.5 {
		t.Fatalf("expected float64 conversion path, got parsed=%v ok=%v", parsed, ok)
	}

	emptyUnion := unionWithoutJoin(nil)
	if len(emptyUnion.Columns) != 0 || len(emptyUnion.Rows) != 0 {
		t.Fatalf("expected empty union result for nil input")
	}
}
