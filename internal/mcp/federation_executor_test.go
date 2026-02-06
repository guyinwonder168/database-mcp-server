package mcp

import (
	"context"
	"database-mcp-provider/internal/config"
	"errors"
	"testing"
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
