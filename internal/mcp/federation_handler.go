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
		return invalidFederatedRequestResult(err), nil, nil
	}

	cfg, err := config.LoadConfig(s.ConfigPath)
	if err != nil {
		return errorResult(NewStructuredError(
			ErrorCodeConfigNotFound,
			"Failed to load configuration",
			err.Error(),
		)), nil, err
	}

	profileMap, availableProfiles := buildProfileIndex(cfg.Profiles)
	plan, planErr := buildFederationPlan(input, availableProfiles)
	if planErr != nil {
		return errorResult(planErr), nil, nil
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

	profilesForExecution, profileErr := resolveExecutionProfiles(optimizedPlan.SubQueries, profileMap)
	if profileErr != nil {
		return errorResult(profileErr), nil, nil
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
	if err := validateFederationPagination(req); err != nil {
		return err
	}
	if err := validateFederationSubQueries(req.SubQueries); err != nil {
		return err
	}
	return validateFederationJoins(req.Joins)
}

func buildFederationResponse(result *FederatedQueryResult) ([]byte, error) {
	if result == nil {
		return nil, fmt.Errorf("result is required")
	}
	return json.Marshal(result)
}

func invalidFederatedRequestResult(err error) *mcp.CallToolResult {
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
	return errorResult(structErr)
}

func buildProfileIndex(profiles []config.Profile) (map[string]config.Profile, []string) {
	profileMap := make(map[string]config.Profile, len(profiles))
	availableProfiles := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		profileMap[profile.ProfileName] = profile
		availableProfiles = append(availableProfiles, profile.ProfileName)
	}
	sort.Strings(availableProfiles)
	return profileMap, availableProfiles
}

func baseFederationPlan(input FederatedQueryRequest) *FederatedQueryPlan {
	return &FederatedQueryPlan{
		SQL:            input.SQL,
		SubQueries:     append([]SubQuery(nil), input.SubQueries...),
		Joins:          append([]JoinCondition(nil), input.Joins...),
		Aggregations:   append([]Aggregation(nil), input.Aggregations...),
		Limit:          input.Limit,
		Offset:         input.Offset,
		MaxConcurrency: input.MaxConcurrency,
	}
}

func buildFederationPlan(input FederatedQueryRequest, availableProfiles []string) (*FederatedQueryPlan, *StructuredError) {
	plan := baseFederationPlan(input)
	if strings.TrimSpace(input.SQL) == "" {
		return plan, nil
	}
	parsedPlan, parseErr := ParseFederatedQuery(input.SQL)
	if parseErr != nil {
		return nil, NewStructuredError(
			ErrorCodeInvalidInput,
			"Unable to parse federated SQL",
			parseErr.Error(),
		)
	}
	subQueries, buildErr := BuildSubQueries(parsedPlan, availableProfiles)
	if buildErr != nil {
		return nil, NewStructuredError(
			ErrorCodeProfileNotFound,
			"Unable to build federated subqueries",
			buildErr.Error(),
		)
	}
	parsedPlan.SubQueries = subQueries
	applyFederationPlanOverrides(parsedPlan, plan)
	return parsedPlan, nil
}

func applyFederationPlanOverrides(parsedPlan, base *FederatedQueryPlan) {
	parsedPlan.Aggregations = base.Aggregations
	parsedPlan.MaxConcurrency = base.MaxConcurrency
	if base.Limit != 0 {
		parsedPlan.Limit = base.Limit
	}
	parsedPlan.Offset = base.Offset
	if len(base.Joins) > 0 {
		parsedPlan.Joins = base.Joins
	}
	if len(base.SubQueries) > 0 {
		parsedPlan.SubQueries = base.SubQueries
	}
}

func resolveExecutionProfiles(subQueries []SubQuery, profileMap map[string]config.Profile) (map[string]config.Profile, *StructuredError) {
	profilesForExecution := make(map[string]config.Profile)
	for _, subQuery := range subQueries {
		profile, ok := profileMap[subQuery.Profile]
		if !ok {
			return nil, NewStructuredError(
				ErrorCodeProfileNotFound,
				fmt.Sprintf("Profile '%s' not found", subQuery.Profile),
				"Subquery references unknown profile",
			).WithContext("alias", subQuery.Alias)
		}
		profilesForExecution[subQuery.Profile] = profile
	}
	return profilesForExecution, nil
}

func validateFederationPagination(req FederatedQueryRequest) error {
	if req.Limit < 0 {
		return fmt.Errorf("limit must be >= 0")
	}
	if req.Offset < 0 {
		return fmt.Errorf("offset must be >= 0")
	}
	if req.MaxConcurrency < 0 {
		return fmt.Errorf("max_concurrency must be >= 0")
	}
	return nil
}

func validateFederationSubQueries(subQueries []SubQuery) error {
	seenAliases := make(map[string]struct{}, len(subQueries))
	for _, subQuery := range subQueries {
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
	return nil
}

func validateFederationJoins(joins []JoinCondition) error {
	for _, join := range joins {
		if err := validateJoinCondition(join); err != nil {
			return err
		}
	}
	return nil
}
