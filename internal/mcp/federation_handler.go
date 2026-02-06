package mcp

import (
	"context"
	"database-mcp-provider/internal/config"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s *MCPServer) handleFederatedQuery(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input FederatedQueryRequest,
) (*mcp.CallToolResult, any, error) {
	if err := validateFederatedRequest(input); err != nil {
		structErr := NewStructuredError(
			ErrorCodeInvalidInput,
			"Invalid federated query request",
			err.Error(),
		).WithSuggestions(
			ErrorSuggestion{
				Action:      "Provide SQL with profile.table references or explicit sub_queries",
				Description: "Each subquery requires profile, alias, and read-only SQL",
				Example:     `{"sub_queries":[{"profile":"crm_db","sql":"SELECT * FROM users","alias":"u"}]}`,
			},
		)
		return errorResult(structErr), nil, nil
	}

	cfg, err := config.LoadConfig(s.ConfigPath)
	if err != nil {
		structErr := NewStructuredError(
			ErrorCodeConfigNotFound,
			"Failed to load configuration",
			err.Error(),
		)
		return errorResult(structErr), nil, err
	}

	profileMap := make(map[string]config.Profile, len(cfg.Profiles))
	availableProfiles := make([]string, 0, len(cfg.Profiles))
	for _, profile := range cfg.Profiles {
		profileMap[profile.ProfileName] = profile
		availableProfiles = append(availableProfiles, profile.ProfileName)
	}
	sort.Strings(availableProfiles)

	plan := &FederatedQueryPlan{
		SQL:            input.SQL,
		SubQueries:     append([]SubQuery(nil), input.SubQueries...),
		Joins:          append([]JoinCondition(nil), input.Joins...),
		Aggregations:   append([]Aggregation(nil), input.Aggregations...),
		Limit:          input.Limit,
		Offset:         input.Offset,
		MaxConcurrency: input.MaxConcurrency,
	}

	if strings.TrimSpace(input.SQL) != "" {
		parsedPlan, parseErr := ParseFederatedQuery(input.SQL)
		if parseErr != nil {
			structErr := NewStructuredError(
				ErrorCodeInvalidInput,
				"Unable to parse federated SQL",
				parseErr.Error(),
			)
			return errorResult(structErr), nil, nil
		}
		subQueries, buildErr := BuildSubQueries(parsedPlan, availableProfiles)
		if buildErr != nil {
			structErr := NewStructuredError(
				ErrorCodeProfileNotFound,
				"Unable to build federated subqueries",
				buildErr.Error(),
			)
			return errorResult(structErr), nil, nil
		}
		parsedPlan.SubQueries = subQueries
		parsedPlan.Aggregations = plan.Aggregations
		parsedPlan.MaxConcurrency = plan.MaxConcurrency
		if plan.Limit != 0 {
			parsedPlan.Limit = plan.Limit
		}
		parsedPlan.Offset = plan.Offset
		if len(plan.Joins) > 0 {
			parsedPlan.Joins = plan.Joins
		}
		if len(plan.SubQueries) > 0 {
			parsedPlan.SubQueries = plan.SubQueries
		}
		plan = parsedPlan
	}

	optimizedPlan := OptimizeFederationPlan(plan)
	if optimizedPlan == nil {
		structErr := NewStructuredError(
			ErrorCodeInvalidInput,
			"Invalid federation plan",
			"federation plan is nil after optimization",
		)
		return errorResult(structErr), nil, nil
	}

	profilesForExecution := make(map[string]config.Profile)
	for _, subQuery := range optimizedPlan.SubQueries {
		profile, ok := profileMap[subQuery.Profile]
		if !ok {
			structErr := NewStructuredError(
				ErrorCodeProfileNotFound,
				fmt.Sprintf("Profile '%s' not found", subQuery.Profile),
				"Subquery references unknown profile",
			).WithContext("alias", subQuery.Alias)
			return errorResult(structErr), nil, nil
		}
		profilesForExecution[subQuery.Profile] = profile
	}

	result, execErr := ExecuteFederatedQuery(ctx, optimizedPlan, profilesForExecution)
	if execErr != nil {
		structErr := NewStructuredError(
			ErrorCodeSQLExecutionError,
			"Federated query execution failed",
			execErr.Error(),
		)
		return errorResult(structErr), nil, nil
	}

	responseBytes, err := buildFederationResponse(result)
	if err != nil {
		return nil, nil, err
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(responseBytes)},
		},
	}, nil, nil
}

func validateFederatedRequest(req FederatedQueryRequest) error {
	if strings.TrimSpace(req.SQL) == "" && len(req.SubQueries) == 0 {
		return fmt.Errorf("either sql or sub_queries must be provided")
	}
	if req.Limit < 0 {
		return fmt.Errorf("limit must be >= 0")
	}
	if req.Offset < 0 {
		return fmt.Errorf("offset must be >= 0")
	}
	if req.MaxConcurrency < 0 {
		return fmt.Errorf("max_concurrency must be >= 0")
	}

	seenAliases := make(map[string]struct{}, len(req.SubQueries))
	for _, subQuery := range req.SubQueries {
		if strings.TrimSpace(subQuery.Profile) == "" {
			return fmt.Errorf("subquery profile is required")
		}
		if strings.TrimSpace(subQuery.Alias) == "" {
			return fmt.Errorf("subquery alias is required")
		}
		if strings.TrimSpace(subQuery.SQL) == "" {
			return fmt.Errorf("subquery sql is required")
		}
		if !isFederationReadOnlySQL(subQuery.SQL) {
			return fmt.Errorf("subquery %s contains non read-only SQL", subQuery.Alias)
		}
		if _, exists := seenAliases[subQuery.Alias]; exists {
			return fmt.Errorf("duplicate subquery alias: %s", subQuery.Alias)
		}
		seenAliases[subQuery.Alias] = struct{}{}
	}

	for _, join := range req.Joins {
		if err := validateJoinCondition(join); err != nil {
			return err
		}
	}

	return nil
}

func buildFederationResponse(result *FederatedQueryResult) ([]byte, error) {
	if result == nil {
		return nil, fmt.Errorf("result is required")
	}
	return json.Marshal(result)
}
