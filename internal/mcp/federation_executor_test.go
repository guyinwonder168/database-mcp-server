package mcp

import (
	"context"
	"database-mcp-provider/internal/config"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestExecuteConcurrently(t *testing.T) {
	original := federationExecuteSubQuery
	defer func() { federationExecuteSubQuery = original }()

	federationExecuteSubQuery = func(_ context.Context, subquery SubQuery, _ config.Profile) (*SubQueryResult, error) {
		if subquery.Alias == "bad" {
			return nil, errors.New("simulated error")
		}
		return &SubQueryResult{
			Profile: subquery.Profile,
			Alias:   subquery.Alias,
			Columns: []string{"id"},
			Rows:    []Row{{1}},
		}, nil
	}

	subqueries := []SubQuery{
		{Profile: "p1", Alias: "u", SQL: "SELECT 1"},
		{Profile: "p2", Alias: "bad", SQL: "SELECT 1"},
	}
	profiles := map[string]config.Profile{
		"p1": {ProfileName: "p1", DBType: "sqlite"},
		"p2": {ProfileName: "p2", DBType: "sqlite"},
	}

	results, federationErrors := executeConcurrentlyWithContext(context.Background(), subqueries, profiles, 2)
	if len(results) != 1 {
		t.Fatalf("expected 1 successful result, got %d", len(results))
	}
	if len(federationErrors) != 1 {
		t.Fatalf("expected 1 federation error, got %d", len(federationErrors))
	}
}

func TestExecuteFederatedQueryWithJoinAndLimit(t *testing.T) {
	original := federationExecuteSubQuery
	defer func() { federationExecuteSubQuery = original }()

	federationExecuteSubQuery = func(_ context.Context, subquery SubQuery, _ config.Profile) (*SubQueryResult, error) {
		switch subquery.Alias {
		case "u":
			return &SubQueryResult{
				Profile: subquery.Profile,
				Alias:   subquery.Alias,
				Columns: []string{"id", "name"},
				Rows:    []Row{{1, "Alice"}, {2, "Bob"}},
			}, nil
		case "o":
			return &SubQueryResult{
				Profile: subquery.Profile,
				Alias:   subquery.Alias,
				Columns: []string{"user_id", "total"},
				Rows:    []Row{{1, 100}, {1, 200}, {3, 50}},
			}, nil
		default:
			return nil, errors.New("unknown alias")
		}
	}

	plan := &FederatedQueryPlan{
		SubQueries: []SubQuery{
			{Profile: "p1", Alias: "u", SQL: "SELECT * FROM users"},
			{Profile: "p2", Alias: "o", SQL: "SELECT * FROM orders"},
		},
		Joins: []JoinCondition{
			{Left: "u.id", Right: "o.user_id", Type: FederationJoinInner},
		},
		Limit: 1,
	}
	profiles := map[string]config.Profile{
		"p1": {ProfileName: "p1", DBType: "sqlite"},
		"p2": {ProfileName: "p2", DBType: "sqlite"},
	}

	result, err := ExecuteFederatedQuery(context.Background(), plan, profiles)
	if err != nil {
		t.Fatalf("expected federated query success, got %v", err)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("expected applied limit with 1 row, got %d", len(result.Rows))
	}
	if result.Metadata.ExecutionTimeMs < 0 {
		t.Fatalf("expected non-negative execution time")
	}
	if result.Metadata.RowsFromEach["u"] != 2 || result.Metadata.RowsFromEach["o"] != 3 {
		t.Fatalf("unexpected rows_from_each: %+v", result.Metadata.RowsFromEach)
	}
}

func TestHandlePartialFailureAndPagination(t *testing.T) {
	partial, err := HandlePartialFailure([]SubQueryResult{
		{
			Profile: "p1",
			Alias:   "u",
			Columns: []string{"id"},
			Rows:    []Row{{1}, {2}, {3}},
		},
	}, []FederationError{{Alias: "o", Message: "timeout"}})
	if err != nil {
		t.Fatalf("expected partial result, got %v", err)
	}
	if len(partial.Metadata.Errors) != 1 {
		t.Fatalf("expected one propagated error")
	}

	paginated := ApplyLimitsAndOffsets(partial, 1, 1)
	if len(paginated.Rows) != 1 {
		t.Fatalf("expected one paginated row, got %d", len(paginated.Rows))
	}
	if paginated.Rows[0][0] != 2 {
		t.Fatalf("unexpected paginated row value: %+v", paginated.Rows[0])
	}
}

func TestExecuteConcurrentlyWrapperAndJoinPipelineBranches(t *testing.T) {
	original := federationExecuteSubQuery
	defer func() { federationExecuteSubQuery = original }()

	federationExecuteSubQuery = func(_ context.Context, subquery SubQuery, _ config.Profile) (*SubQueryResult, error) {
		switch subquery.Alias {
		case "a":
			return &SubQueryResult{
				Profile: "p1",
				Alias:   "a",
				Columns: []string{"id"},
				Rows:    []Row{{1}},
			}, nil
		case "b":
			return &SubQueryResult{
				Profile: "p2",
				Alias:   "b",
				Columns: []string{"id", "a_id"},
				Rows:    []Row{{10, 1}},
			}, nil
		case "c":
			return &SubQueryResult{
				Profile: "p3",
				Alias:   "c",
				Columns: []string{"b_id"},
				Rows:    []Row{{1}},
			}, nil
		default:
			return nil, errors.New("unexpected alias")
		}
	}

	subqueries := []SubQuery{
		{Profile: "p1", Alias: "a", SQL: "SELECT 1"},
		{Profile: "p2", Alias: "b", SQL: "SELECT 1"},
	}
	profiles := map[string]config.Profile{
		"p1": {ProfileName: "p1", DBType: "sqlite"},
		"p2": {ProfileName: "p2", DBType: "sqlite"},
	}

	wrapperResults := ExecuteConcurrently(subqueries, profiles, 2)
	if len(wrapperResults) != 2 {
		t.Fatalf("expected wrapper ExecuteConcurrently to return two results, got %d", len(wrapperResults))
	}

	joined, err := executeJoinPipeline([]SubQueryResult{
		{Profile: "p1", Alias: "a", Columns: []string{"id"}, Rows: []Row{{1}}},
		{Profile: "p2", Alias: "b", Columns: []string{"id", "a_id"}, Rows: []Row{{10, 1}}},
		{Profile: "p3", Alias: "c", Columns: []string{"b_id"}, Rows: []Row{{1}}},
	}, []JoinCondition{
		{Left: "a.id", Right: "b.a_id", Type: FederationJoinInner},
		{Left: "b.id", Right: "c.b_id", Type: FederationJoinInner},
	})
	if err != nil {
		t.Fatalf("expected multi-step join pipeline success, got %v", err)
	}
	if len(joined.Rows) != 1 {
		t.Fatalf("expected one joined row, got %d", len(joined.Rows))
	}

	_, err = executeJoinPipeline([]SubQueryResult{
		{Profile: "p1", Alias: "a", Columns: []string{"id"}, Rows: []Row{{1}}},
	}, []JoinCondition{
		{Left: "missing.id", Right: "a.id", Type: FederationJoinInner},
	})
	if err == nil {
		t.Fatalf("expected unknown alias error in join pipeline")
	}
}

func TestHandlePartialFailureErrors(t *testing.T) {
	_, err := HandlePartialFailure(nil, nil)
	if err == nil {
		t.Fatalf("expected error when no results and no errors are provided")
	}

	_, err = HandlePartialFailure(nil, []FederationError{{Message: "boom"}})
	if err == nil {
		t.Fatalf("expected error when all subqueries failed")
	}

	// Cover helper for empty map copy path.
	cloned := ApplyLimitsAndOffsets(&FederatedQueryResult{
		Columns:  []string{"id"},
		Rows:     []Row{{1}},
		Metadata: FederationMetadata{},
	}, 0, 0)
	if cloned == nil {
		t.Fatalf("expected non-nil cloned result")
	}
}

func TestExecuteFederatedQueryValidationAndFailures(t *testing.T) {
	if _, err := ExecuteFederatedQuery(context.Background(), nil, nil); err == nil {
		t.Fatalf("expected nil plan validation error")
	}

	if _, err := ExecuteFederatedQuery(context.Background(), &FederatedQueryPlan{}, nil); err == nil {
		t.Fatalf("expected empty subqueries validation error")
	}

	plan := &FederatedQueryPlan{
		SubQueries: []SubQuery{
			{Profile: "missing", Alias: "u", SQL: "SELECT 1"},
		},
	}
	_, err := ExecuteFederatedQuery(context.Background(), plan, map[string]config.Profile{})
	if err == nil {
		t.Fatalf("expected all-subqueries-failed error")
	}
	if !strings.Contains(err.Error(), "all subqueries failed") {
		t.Fatalf("expected all-subqueries-failed message, got %v", err)
	}
}

func TestExecuteFederatedQueryAggregationPaths(t *testing.T) {
	original := federationExecuteSubQuery
	defer func() { federationExecuteSubQuery = original }()

	federationExecuteSubQuery = func(_ context.Context, subquery SubQuery, _ config.Profile) (*SubQueryResult, error) {
		return &SubQueryResult{
			Profile: subquery.Profile,
			Alias:   subquery.Alias,
			Columns: []string{"total"},
			Rows:    []Row{{10}, {5}},
		}, nil
	}

	profiles := map[string]config.Profile{
		"p1": {ProfileName: "p1", DBType: "sqlite"},
	}

	successPlan := &FederatedQueryPlan{
		SubQueries: []SubQuery{{Profile: "p1", Alias: "u", SQL: "SELECT total FROM orders"}},
		Aggregations: []Aggregation{
			{Function: "SUM", Column: "total", Alias: "sum_total"},
		},
	}
	result, err := ExecuteFederatedQuery(context.Background(), successPlan, profiles)
	if err != nil {
		t.Fatalf("expected aggregation success, got %v", err)
	}
	if len(result.Rows) != 1 || result.Rows[0][0] != float64(15) {
		t.Fatalf("unexpected aggregation output: %+v", result.Rows)
	}

	failPlan := &FederatedQueryPlan{
		SubQueries: []SubQuery{{Profile: "p1", Alias: "u", SQL: "SELECT total FROM orders"}},
		Aggregations: []Aggregation{
			{Function: "UNSUPPORTED", Column: "total"},
		},
	}
	if _, err := ExecuteFederatedQuery(context.Background(), failPlan, profiles); err == nil {
		t.Fatalf("expected aggregation error")
	}
}

func TestExecuteFederatedQueryJoinFallbackAndPagination(t *testing.T) {
	original := federationExecuteSubQuery
	defer func() { federationExecuteSubQuery = original }()

	federationExecuteSubQuery = func(_ context.Context, subquery SubQuery, _ config.Profile) (*SubQueryResult, error) {
		switch subquery.Alias {
		case "u":
			return &SubQueryResult{
				Profile: subquery.Profile,
				Alias:   subquery.Alias,
				Columns: []string{"id", "name"},
				Rows:    []Row{{1, "Alice"}, {2, "Bob"}},
			}, nil
		case "o":
			return &SubQueryResult{
				Profile: subquery.Profile,
				Alias:   subquery.Alias,
				Columns: []string{"order_id"},
				Rows:    []Row{{10}},
			}, nil
		case "bad":
			return nil, errors.New("timeout")
		default:
			return nil, errors.New("unexpected alias")
		}
	}

	plan := &FederatedQueryPlan{
		SubQueries: []SubQuery{
			{Profile: "p1", Alias: "u", SQL: "SELECT * FROM users"},
			{Profile: "p2", Alias: "o", SQL: "SELECT * FROM orders"},
			{Profile: "p3", Alias: "bad", SQL: "SELECT * FROM slow_table"},
		},
		Joins: []JoinCondition{
			// o.user_id does not exist and forces join fallback path.
			{Left: "u.id", Right: "o.user_id", Type: FederationJoinInner},
		},
		Limit:  1,
		Offset: 1,
	}

	profiles := map[string]config.Profile{
		"p1": {ProfileName: "p1", DBType: "sqlite"},
		"p2": {ProfileName: "p2", DBType: "sqlite"},
		"p3": {ProfileName: "p3", DBType: "sqlite"},
	}

	result, err := ExecuteFederatedQuery(context.Background(), plan, profiles)
	if err != nil {
		t.Fatalf("expected fallback partial success, got %v", err)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("expected offset+limit pagination, got %d rows", len(result.Rows))
	}
	if len(result.Metadata.Errors) != 1 {
		t.Fatalf("expected propagated subquery error, got %+v", result.Metadata.Errors)
	}
}

func TestExecuteConcurrentlyWithContextEdgeCases(t *testing.T) {
	original := federationExecuteSubQuery
	defer func() { federationExecuteSubQuery = original }()

	subqueries := []SubQuery{{Profile: "p1", Alias: "u", SQL: "SELECT 1"}}
	profiles := map[string]config.Profile{
		"p1": {ProfileName: "p1", DBType: "sqlite"},
	}

	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	results, federationErrors := executeConcurrentlyWithContext(canceledCtx, subqueries, profiles, 1)
	if len(results) != 0 {
		t.Fatalf("expected zero results for canceled context")
	}
	if len(federationErrors) != 1 || federationErrors[0].Code != "CONTEXT_CANCELED" {
		t.Fatalf("expected CONTEXT_CANCELED error, got %+v", federationErrors)
	}

	federationExecuteSubQuery = func(_ context.Context, _ SubQuery, _ config.Profile) (*SubQueryResult, error) {
		return nil, nil
	}
	results, federationErrors = executeConcurrentlyWithContext(context.Background(), subqueries, profiles, 1)
	if len(results) != 0 {
		t.Fatalf("expected zero results when subquery returns nil result")
	}
	if len(federationErrors) != 1 || federationErrors[0].Code != "EMPTY_SUBQUERY_RESULT" {
		t.Fatalf("expected EMPTY_SUBQUERY_RESULT error, got %+v", federationErrors)
	}

	federationExecuteSubQuery = func(_ context.Context, _ SubQuery, _ config.Profile) (*SubQueryResult, error) {
		return &SubQueryResult{
			Columns: []string{"id"},
			Rows:    []Row{{1}},
		}, nil
	}
	results, federationErrors = executeConcurrentlyWithContext(context.Background(), subqueries, profiles, 1)
	if len(federationErrors) != 0 {
		t.Fatalf("expected no errors, got %+v", federationErrors)
	}
	if len(results) != 1 || results[0].Alias != "u" || results[0].Profile != "p1" {
		t.Fatalf("expected alias/profile defaults to be populated, got %+v", results)
	}

	results, federationErrors = executeConcurrentlyWithContext(context.Background(), subqueries, map[string]config.Profile{}, 1)
	if len(results) != 0 {
		t.Fatalf("expected zero results for missing profile")
	}
	if len(federationErrors) != 1 || federationErrors[0].Code != "PROFILE_NOT_FOUND" {
		t.Fatalf("expected PROFILE_NOT_FOUND error, got %+v", federationErrors)
	}

	results, federationErrors = executeConcurrentlyWithContext(context.Background(), nil, profiles, 1)
	if len(results) != 0 || len(federationErrors) != 0 {
		t.Fatalf("expected empty result and errors for empty subqueries")
	}

	federationExecuteSubQuery = func(_ context.Context, sq SubQuery, _ config.Profile) (*SubQueryResult, error) {
		return &SubQueryResult{
			Profile: sq.Profile,
			Alias:   sq.Alias,
			Columns: []string{"id"},
			Rows:    []Row{{1}},
		}, nil
	}
	results, federationErrors = executeConcurrentlyWithContext(context.Background(), []SubQuery{
		{Profile: "p1", Alias: "z", SQL: "SELECT 1"},
		{Profile: "p1", Alias: "a", SQL: "SELECT 1"},
	}, profiles, 0)
	if len(federationErrors) != 0 {
		t.Fatalf("expected no errors with maxConcurrency defaulting, got %+v", federationErrors)
	}
	if len(results) != 2 || results[0].Alias != "a" || results[1].Alias != "z" {
		t.Fatalf("expected deterministic alias sorting, got %+v", results)
	}

	blockCtx, cancel := context.WithCancel(context.Background())
	release := make(chan struct{})
	started := make(chan struct{})
	var startedOnce sync.Once
	federationExecuteSubQuery = func(_ context.Context, sq SubQuery, _ config.Profile) (*SubQueryResult, error) {
		startedOnce.Do(func() {
			close(started)
		})
		<-release
		return &SubQueryResult{
			Profile: sq.Profile,
			Alias:   sq.Alias,
			Columns: []string{"id"},
			Rows:    []Row{{1}},
		}, nil
	}
	done := make(chan struct {
		results []SubQueryResult
		errors  []FederationError
	}, 1)
	go func() {
		res, errs := executeConcurrentlyWithContext(blockCtx, []SubQuery{
			{Profile: "p1", Alias: "first", SQL: "SELECT 1"},
			{Profile: "p1", Alias: "second", SQL: "SELECT 1"},
		}, profiles, 1)
		done <- struct {
			results []SubQueryResult
			errors  []FederationError
		}{results: res, errors: errs}
	}()
	<-started
	time.Sleep(20 * time.Millisecond)
	cancel()
	close(release)
	out := <-done
	foundCanceled := false
	for _, federationError := range out.errors {
		if federationError.Code == "CONTEXT_CANCELED" {
			foundCanceled = true
			break
		}
	}
	if !foundCanceled {
		t.Fatalf("expected CONTEXT_CANCELED from blocked semaphore path, got %+v", out.errors)
	}
}

func TestApplyLimitsAndOffsetsEdgeCases(t *testing.T) {
	if got := ApplyLimitsAndOffsets(nil, 1, 1); got != nil {
		t.Fatalf("expected nil passthrough")
	}

	original := &FederatedQueryResult{
		Columns: []string{"id"},
		Rows:    []Row{{1}, {2}},
		Metadata: FederationMetadata{
			RowsFromEach: map[string]int{"u": 2},
			Errors:       []FederationError{{Alias: "u", Message: "sample"}},
		},
	}

	cloned := ApplyLimitsAndOffsets(original, -1, -1)
	if len(cloned.Rows) != 2 {
		t.Fatalf("expected negative bounds to clamp without dropping rows, got %d", len(cloned.Rows))
	}
	cloned.Metadata.RowsFromEach["u"] = 100
	if original.Metadata.RowsFromEach["u"] != 2 {
		t.Fatalf("expected metadata map clone isolation")
	}

	empty := ApplyLimitsAndOffsets(original, 1, 10)
	if len(empty.Rows) != 0 {
		t.Fatalf("expected empty rows for offset beyond size, got %d", len(empty.Rows))
	}
}

func TestExecuteJoinPipelineAdditionalBranches(t *testing.T) {
	if _, err := executeJoinPipeline(nil, nil); err == nil {
		t.Fatalf("expected empty results validation error")
	}

	unioned, err := executeJoinPipeline([]SubQueryResult{
		{Alias: "a", Columns: []string{"id"}, Rows: []Row{{1}}},
	}, nil)
	if err != nil {
		t.Fatalf("expected union path success, got %v", err)
	}
	if len(unioned.Rows) != 1 {
		t.Fatalf("expected one unioned row, got %d", len(unioned.Rows))
	}

	base := []SubQueryResult{
		{Alias: "a", Columns: []string{"id"}, Rows: []Row{{1}}},
		{Alias: "b", Columns: []string{"id", "a_id"}, Rows: []Row{{10, 1}}},
		{Alias: "c", Columns: []string{"a_id"}, Rows: []Row{{1}}},
		{Alias: "d", Columns: []string{"id"}, Rows: []Row{{99}}},
	}

	joined, err := executeJoinPipeline(base[:3], []JoinCondition{
		{Left: "a.id", Right: "b.a_id", Type: FederationJoinInner},
		{Left: "c.a_id", Right: "a.id", Type: FederationJoinInner},
	})
	if err != nil {
		t.Fatalf("expected join path with right-side joined alias, got %v", err)
	}
	if len(joined.Rows) == 0 {
		t.Fatalf("expected joined rows")
	}

	_, err = executeJoinPipeline(base, []JoinCondition{
		{Left: "a.id", Right: "b.a_id", Type: FederationJoinInner},
		{Left: "c.a_id", Right: "d.id", Type: FederationJoinInner},
	})
	if err == nil {
		t.Fatalf("expected join ordering error when no side was previously joined")
	}

	_, err = executeJoinPipeline(base[:2], []JoinCondition{
		{Left: "a.id", Right: "b.a_id", Type: FederationJoinInner},
		{Left: "a.id", Right: "missing.id", Type: FederationJoinInner},
	})
	if err == nil {
		t.Fatalf("expected unknown-right-alias error for case 1")
	}

	_, err = executeJoinPipeline(base[:3], []JoinCondition{
		{Left: "a.id", Right: "b.a_id", Type: FederationJoinInner},
		{Left: "a.id", Right: "c.missing", Type: FederationJoinInner},
	})
	if err == nil {
		t.Fatalf("expected join execution error for case 1 when right column is missing")
	}

	_, err = executeJoinPipeline(base[:2], []JoinCondition{
		{Left: "a.id", Right: "b.a_id", Type: FederationJoinInner},
		{Left: "missing.id", Right: "a.id", Type: FederationJoinInner},
	})
	if err == nil {
		t.Fatalf("expected unknown-left-alias error for case 2")
	}

	_, err = executeJoinPipeline(base[:3], []JoinCondition{
		{Left: "a.id", Right: "b.a_id", Type: FederationJoinInner},
		{Left: "c.missing", Right: "a.id", Type: FederationJoinInner},
	})
	if err == nil {
		t.Fatalf("expected join execution error for case 2 when left column is missing")
	}

	redundant, err := executeJoinPipeline(base[:2], []JoinCondition{
		{Left: "a.id", Right: "b.a_id", Type: FederationJoinInner},
		{Left: "b.a_id", Right: "a.id", Type: FederationJoinInner},
	})
	if err != nil {
		t.Fatalf("expected redundant join to be skipped, got %v", err)
	}
	if len(redundant.Rows) == 0 {
		t.Fatalf("expected rows after redundant join sequence")
	}
}

func TestRowsFromEachUsesProfileWhenAliasIsEmpty(t *testing.T) {
	counts := rowsFromEach([]SubQueryResult{
		{Profile: "p1", Alias: "", Rows: []Row{{1}, {2}}},
	})
	if counts["p1"] != 2 {
		t.Fatalf("expected profile fallback key, got %+v", counts)
	}
}
