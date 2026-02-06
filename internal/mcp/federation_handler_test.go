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
