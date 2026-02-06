//go:build cgo

package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"database-mcp-provider/internal/config"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestValidateFederatedRequest(t *testing.T) {
	err := validateFederatedRequest(FederatedQueryRequest{})
	if err == nil {
		t.Fatalf("expected validation error when both sql and subqueries are empty")
	}

	valid := FederatedQueryRequest{
		SubQueries: []SubQuery{
			{Profile: "p1", Alias: "u", SQL: "SELECT * FROM users"},
		},
	}
	if err := validateFederatedRequest(valid); err != nil {
		t.Fatalf("expected valid request, got %v", err)
	}
}

func TestBuildFederationResponse(t *testing.T) {
	payload, err := buildFederationResponse(&FederatedQueryResult{
		Columns: []string{"id"},
		Rows:    []Row{{1}},
		Metadata: FederationMetadata{
			ExecutionTimeMs: 10,
			RowsFromEach:    map[string]int{"u": 1},
		},
	})
	if err != nil {
		t.Fatalf("expected buildFederationResponse success, got %v", err)
	}
	if len(payload) == 0 {
		t.Fatalf("expected non-empty serialized response")
	}

	if _, err := buildFederationResponse(nil); err == nil {
		t.Fatalf("expected nil result validation error")
	}
}

func TestHandleFederatedQuery(t *testing.T) {
	testConfig := "test_config_federation.yaml"
	defer os.Remove(testConfig)

	cfg := &config.Config{
		Profiles: []config.Profile{
			{ProfileName: "p1", DBType: "sqlite", DatabaseName: testSQLiteDBPath},
			{ProfileName: "p2", DBType: "sqlite", DatabaseName: testSQLiteDBPath},
		},
		MaxPoolSize: 5,
	}
	if err := config.SaveConfig(testConfig, cfg); err != nil {
		t.Fatalf("failed to save federation test config: %v", err)
	}

	server := NewMCPServerWithConfig(testConfig)
	ctx := context.Background()

	original := federationExecuteSubQuery
	defer func() { federationExecuteSubQuery = original }()

	federationExecuteSubQuery = func(_ context.Context, subquery SubQuery, _ config.Profile) (*SubQueryResult, error) {
		switch subquery.Alias {
		case "u":
			return &SubQueryResult{
				Profile: subquery.Profile,
				Alias:   subquery.Alias,
				Columns: []string{"id", "name"},
				Rows:    []Row{{1, "Alice"}},
			}, nil
		case "o":
			return &SubQueryResult{
				Profile: subquery.Profile,
				Alias:   subquery.Alias,
				Columns: []string{"user_id", "total"},
				Rows:    []Row{{1, 100}},
			}, nil
		default:
			return nil, errors.New("unknown alias")
		}
	}

	response, _, err := server.handleFederatedQuery(ctx, nil, FederatedQueryRequest{
		SubQueries: []SubQuery{
			{Profile: "p1", Alias: "u", SQL: "SELECT * FROM users"},
			{Profile: "p2", Alias: "o", SQL: "SELECT * FROM orders"},
		},
		Joins: []JoinCondition{
			{Left: "u.id", Right: "o.user_id", Type: FederationJoinInner},
		},
		Limit: 1,
	})
	if err != nil {
		t.Fatalf("handleFederatedQuery failed: %v", err)
	}
	if response == nil || len(response.Content) == 0 {
		t.Fatalf("expected non-empty federated response content")
	}

	textContent, ok := response.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected text content response, got %T", response.Content[0])
	}

	var parsed FederatedQueryResult
	if err := json.Unmarshal([]byte(textContent.Text), &parsed); err != nil {
		t.Fatalf("failed to parse federated response: %v", err)
	}
	if len(parsed.Rows) != 1 {
		t.Fatalf("expected 1 joined row, got %d", len(parsed.Rows))
	}
}

func TestHandleFederatedQuery_SQLParserPath(t *testing.T) {
	testConfig := "test_config_federation_sql_path.yaml"
	defer os.Remove(testConfig)

	cfg := &config.Config{
		Profiles: []config.Profile{
			{ProfileName: "profile1", DBType: "sqlite", DatabaseName: testSQLiteDBPath},
			{ProfileName: "profile2", DBType: "sqlite", DatabaseName: testSQLiteDBPath},
		},
		MaxPoolSize: 5,
	}
	if err := config.SaveConfig(testConfig, cfg); err != nil {
		t.Fatalf("failed to save federation sql-path config: %v", err)
	}

	server := NewMCPServerWithConfig(testConfig)
	ctx := context.Background()

	original := federationExecuteSubQuery
	defer func() { federationExecuteSubQuery = original }()

	federationExecuteSubQuery = func(_ context.Context, subquery SubQuery, _ config.Profile) (*SubQueryResult, error) {
		switch subquery.Alias {
		case "u":
			return &SubQueryResult{
				Profile: subquery.Profile,
				Alias:   subquery.Alias,
				Columns: []string{"id", "name"},
				Rows:    []Row{{1, "Alice"}},
			}, nil
		case "o":
			return &SubQueryResult{
				Profile: subquery.Profile,
				Alias:   subquery.Alias,
				Columns: []string{"user_id", "total"},
				Rows:    []Row{{1, 100}},
			}, nil
		default:
			return nil, errors.New("unknown alias")
		}
	}

	response, _, err := server.handleFederatedQuery(ctx, nil, FederatedQueryRequest{
		SQL:   "SELECT * FROM profile1.users u JOIN profile2.orders o ON u.id = o.user_id LIMIT 5",
		Limit: 1,
	})
	if err != nil {
		t.Fatalf("handleFederatedQuery SQL parser path failed: %v", err)
	}
	if response == nil || len(response.Content) == 0 {
		t.Fatalf("expected non-empty response content")
	}
}

func TestHandleFederatedQuery_SQLParserPath_WithOverrides(t *testing.T) {
	testConfig := "test_config_federation_sql_override.yaml"
	defer os.Remove(testConfig)

	cfg := &config.Config{
		Profiles: []config.Profile{
			{ProfileName: "profile1", DBType: "sqlite", DatabaseName: testSQLiteDBPath},
			{ProfileName: "profile2", DBType: "sqlite", DatabaseName: testSQLiteDBPath},
		},
		MaxPoolSize: 5,
	}
	if err := config.SaveConfig(testConfig, cfg); err != nil {
		t.Fatalf("failed to save federation sql-override config: %v", err)
	}

	server := NewMCPServerWithConfig(testConfig)
	ctx := context.Background()

	original := federationExecuteSubQuery
	defer func() { federationExecuteSubQuery = original }()

	federationExecuteSubQuery = func(_ context.Context, subquery SubQuery, _ config.Profile) (*SubQueryResult, error) {
		switch subquery.Alias {
		case "x":
			return &SubQueryResult{
				Profile: subquery.Profile,
				Alias:   subquery.Alias,
				Columns: []string{"id"},
				Rows:    []Row{{1}},
			}, nil
		case "y":
			return &SubQueryResult{
				Profile: subquery.Profile,
				Alias:   subquery.Alias,
				Columns: []string{"user_id"},
				Rows:    []Row{{1}},
			}, nil
		default:
			return nil, errors.New("unexpected alias")
		}
	}

	response, _, err := server.handleFederatedQuery(ctx, nil, FederatedQueryRequest{
		SQL: "SELECT * FROM profile1.users u JOIN profile2.orders o ON u.id = o.user_id",
		SubQueries: []SubQuery{
			{Profile: "profile1", Alias: "x", SQL: "SELECT id FROM users"},
			{Profile: "profile2", Alias: "y", SQL: "SELECT user_id FROM orders"},
		},
		Joins: []JoinCondition{
			{Left: "x.id", Right: "y.user_id", Type: FederationJoinInner},
		},
	})
	if err != nil {
		t.Fatalf("expected SQL parser path with overrides success, got %v", err)
	}
	if response == nil || len(response.Content) == 0 {
		t.Fatalf("expected non-empty response content with overrides")
	}
}

func TestHandleFederatedQuery_ProfileMissingAndInvalidRequest(t *testing.T) {
	testConfig := "test_config_federation_invalid.yaml"
	defer os.Remove(testConfig)

	cfg := &config.Config{
		Profiles: []config.Profile{
			{ProfileName: "only_one", DBType: "sqlite", DatabaseName: testSQLiteDBPath},
		},
		MaxPoolSize: 5,
	}
	if err := config.SaveConfig(testConfig, cfg); err != nil {
		t.Fatalf("failed to save federation invalid config: %v", err)
	}

	server := NewMCPServerWithConfig(testConfig)
	ctx := context.Background()

	invalidRes, _, err := server.handleFederatedQuery(ctx, nil, FederatedQueryRequest{})
	if err != nil {
		t.Fatalf("expected structured invalid-input response, got error: %v", err)
	}
	if invalidRes == nil || len(invalidRes.Content) == 0 {
		t.Fatalf("expected structured error content for invalid request")
	}

	missingProfileRes, _, err := server.handleFederatedQuery(ctx, nil, FederatedQueryRequest{
		SubQueries: []SubQuery{
			{Profile: "missing", Alias: "m", SQL: "SELECT 1"},
		},
	})
	if err != nil {
		t.Fatalf("expected structured profile-not-found response, got error: %v", err)
	}
	if missingProfileRes == nil || len(missingProfileRes.Content) == 0 {
		t.Fatalf("expected structured profile-not-found content")
	}
}

func TestValidateFederatedRequest_Errors(t *testing.T) {
	if err := validateFederatedRequest(FederatedQueryRequest{
		SubQueries: []SubQuery{
			{Profile: "p1", Alias: "dup", SQL: "SELECT 1"},
			{Profile: "p1", Alias: "dup", SQL: "SELECT 1"},
		},
	}); err == nil {
		t.Fatalf("expected duplicate alias validation error")
	}

	if err := validateFederatedRequest(FederatedQueryRequest{
		SubQueries: []SubQuery{
			{Profile: "p1", Alias: "u", SQL: "DELETE FROM users"},
		},
	}); err == nil {
		t.Fatalf("expected unsafe SQL validation error")
	}

	if err := validateFederatedRequest(FederatedQueryRequest{
		SubQueries: []SubQuery{
			{Profile: "p1", Alias: "u", SQL: "SELECT 1"},
		},
		Joins: []JoinCondition{
			{Left: "u.id", Right: "o.user_id", Type: "CROSS"},
		},
	}); err == nil {
		t.Fatalf("expected invalid join type validation error")
	}
}

func TestValidateFederatedRequest_FieldValidationErrors(t *testing.T) {
	testCases := []struct {
		name string
		req  FederatedQueryRequest
	}{
		{
			name: "negative limit",
			req: FederatedQueryRequest{
				SQL:   "SELECT 1",
				Limit: -1,
			},
		},
		{
			name: "negative offset",
			req: FederatedQueryRequest{
				SQL:    "SELECT 1",
				Offset: -1,
			},
		},
		{
			name: "negative max concurrency",
			req: FederatedQueryRequest{
				SQL:            "SELECT 1",
				MaxConcurrency: -1,
			},
		},
		{
			name: "missing profile",
			req: FederatedQueryRequest{
				SubQueries: []SubQuery{{Alias: "u", SQL: "SELECT 1"}},
			},
		},
		{
			name: "missing alias",
			req: FederatedQueryRequest{
				SubQueries: []SubQuery{{Profile: "p1", SQL: "SELECT 1"}},
			},
		},
		{
			name: "missing sql",
			req: FederatedQueryRequest{
				SubQueries: []SubQuery{{Profile: "p1", Alias: "u"}},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if err := validateFederatedRequest(tc.req); err == nil {
				t.Fatalf("expected validation error for %s", tc.name)
			}
		})
	}
}

func TestHandleFederatedQuery_ErrorPaths(t *testing.T) {
	ctx := context.Background()

	// Config load error branch.
	serverMissingConfig := NewMCPServerWithConfig("does-not-exist.yaml")
	result, _, err := serverMissingConfig.handleFederatedQuery(ctx, nil, FederatedQueryRequest{
		SubQueries: []SubQuery{
			{Profile: "p1", Alias: "u", SQL: "SELECT 1"},
		},
	})
	if err == nil {
		t.Fatalf("expected config load error")
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatalf("expected structured config error response content")
	}

	testConfig := "test_config_federation_errors.yaml"
	defer os.Remove(testConfig)

	cfg := &config.Config{
		Profiles: []config.Profile{
			{ProfileName: "p1", DBType: "sqlite", DatabaseName: testSQLiteDBPath},
		},
		MaxPoolSize: 5,
	}
	if saveErr := config.SaveConfig(testConfig, cfg); saveErr != nil {
		t.Fatalf("failed to save federation error-path config: %v", saveErr)
	}

	server := NewMCPServerWithConfig(testConfig)

	// SQL parse error branch.
	parseErrRes, _, parseErr := server.handleFederatedQuery(ctx, nil, FederatedQueryRequest{
		SQL: "DELETE FROM p1.users",
	})
	if parseErr != nil {
		t.Fatalf("expected structured parse error response, got %v", parseErr)
	}
	if parseErrRes == nil || len(parseErrRes.Content) == 0 {
		t.Fatalf("expected parse error content")
	}

	// Build subqueries error branch (profile not available).
	buildErrRes, _, buildErr := server.handleFederatedQuery(ctx, nil, FederatedQueryRequest{
		SQL: "SELECT * FROM missing.users u",
	})
	if buildErr != nil {
		t.Fatalf("expected structured build-subqueries error response, got %v", buildErr)
	}
	if buildErrRes == nil || len(buildErrRes.Content) == 0 {
		t.Fatalf("expected build-subqueries error content")
	}

	// Execution error branch.
	original := federationExecuteSubQuery
	defer func() { federationExecuteSubQuery = original }()
	federationExecuteSubQuery = func(_ context.Context, _ SubQuery, _ config.Profile) (*SubQueryResult, error) {
		return nil, errors.New("forced execution failure")
	}

	execErrRes, _, execErr := server.handleFederatedQuery(ctx, nil, FederatedQueryRequest{
		SubQueries: []SubQuery{
			{Profile: "p1", Alias: "u", SQL: "SELECT 1"},
		},
	})
	if execErr != nil {
		t.Fatalf("expected structured execution error response, got %v", execErr)
	}
	if execErrRes == nil || len(execErrRes.Content) == 0 {
		t.Fatalf("expected execution error content")
	}

	// Response serialization error branch.
	federationExecuteSubQuery = func(_ context.Context, subquery SubQuery, _ config.Profile) (*SubQueryResult, error) {
		return &SubQueryResult{
			Profile: subquery.Profile,
			Alias:   subquery.Alias,
			Columns: []string{"id"},
			Rows:    []Row{{make(chan int)}},
		}, nil
	}

	serializedRes, _, serializedErr := server.handleFederatedQuery(ctx, nil, FederatedQueryRequest{
		SubQueries: []SubQuery{
			{Profile: "p1", Alias: "u", SQL: "SELECT 1"},
		},
	})
	if serializedErr == nil {
		t.Fatalf("expected serialization error when response contains unsupported type")
	}
	if serializedRes != nil {
		t.Fatalf("expected nil response when serialization fails")
	}
}
