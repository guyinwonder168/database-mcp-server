package mcp

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	federationTablePattern = regexp.MustCompile(`(?i)\b(from|join)\s+([a-zA-Z0-9_]+)\.([a-zA-Z0-9_]+)(?:\s+(?:as\s+)?([a-zA-Z0-9_]+))?`)
	federationJoinPattern  = regexp.MustCompile(`(?i)\b(?:(inner|left|right|full)(?:\s+outer)?\s+)?join\s+[a-zA-Z0-9_]+\.[a-zA-Z0-9_]+(?:\s+(?:as\s+)?[a-zA-Z0-9_]+)?\s+on\s+([a-zA-Z0-9_]+\.[a-zA-Z0-9_]+)\s*=\s*([a-zA-Z0-9_]+\.[a-zA-Z0-9_]+)`)
	federationLimitPattern = regexp.MustCompile(`(?i)\blimit\s+([0-9]+)`)
	federationOffPattern   = regexp.MustCompile(`(?i)\boffset\s+([0-9]+)`)
)

// ParseFederatedQuery parses a simplified federated SQL expression into an execution plan.
//
// Supported syntax includes:
// - profile.table references in FROM/JOIN clauses
// - JOIN with ON left = right
// - optional LIMIT/OFFSET
func ParseFederatedQuery(sqlText string) (*FederatedQueryPlan, error) {
	trimmed := strings.TrimSpace(sqlText)
	if trimmed == "" {
		return nil, fmt.Errorf("sql is required")
	}

	lower := strings.ToLower(trimmed)
	if !strings.HasPrefix(lower, "select") && !strings.HasPrefix(lower, "with") {
		return nil, fmt.Errorf("only SELECT/CTE statements are supported for federation")
	}

	tableMatches := federationTablePattern.FindAllStringSubmatch(trimmed, -1)
	if len(tableMatches) == 0 {
		return nil, fmt.Errorf("no federated table references found; expected profile.table syntax")
	}

	tables := make([]FederatedTable, 0, len(tableMatches))
	seenAliases := make(map[string]struct{}, len(tableMatches))
	for _, match := range tableMatches {
		profile := match[2]
		table := match[3]
		alias := strings.TrimSpace(match[4])
		if alias == "" {
			alias = table
		}
		if _, exists := seenAliases[alias]; exists {
			continue
		}
		seenAliases[alias] = struct{}{}
		tables = append(tables, FederatedTable{
			Profile: profile,
			Table:   table,
			Alias:   alias,
		})
	}

	joins := make([]JoinCondition, 0)
	joinMatches := federationJoinPattern.FindAllStringSubmatch(trimmed, -1)
	for _, match := range joinMatches {
		joinType := normalizeJoinType(match[1])
		joins = append(joins, JoinCondition{
			Left:  match[2],
			Right: match[3],
			Type:  joinType,
		})
	}

	plan := &FederatedQueryPlan{
		SQL:    trimmed,
		Tables: tables,
		Joins:  joins,
	}

	if limitMatch := federationLimitPattern.FindStringSubmatch(trimmed); len(limitMatch) == 2 {
		if limit, err := strconv.Atoi(limitMatch[1]); err == nil {
			plan.Limit = limit
		}
	}
	if offsetMatch := federationOffPattern.FindStringSubmatch(trimmed); len(offsetMatch) == 2 {
		if offset, err := strconv.Atoi(offsetMatch[1]); err == nil {
			plan.Offset = offset
		}
	}

	return plan, nil
}

// BuildSubQueries converts parsed plan tables into executable subqueries.
func BuildSubQueries(plan *FederatedQueryPlan, availableProfiles []string) ([]SubQuery, error) {
	if plan == nil {
		return nil, fmt.Errorf("plan is required")
	}
	if len(plan.Tables) == 0 {
		return nil, fmt.Errorf("plan has no tables to build subqueries")
	}

	available := make(map[string]struct{}, len(availableProfiles))
	for _, profile := range availableProfiles {
		available[profile] = struct{}{}
	}

	subqueries := make([]SubQuery, 0, len(plan.Tables))
	for _, table := range plan.Tables {
		if len(available) > 0 {
			if _, ok := available[table.Profile]; !ok {
				return nil, fmt.Errorf("profile %s is not available", table.Profile)
			}
		}
		subqueries = append(subqueries, SubQuery{
			Profile: table.Profile,
			Alias:   table.Alias,
			SQL:     fmt.Sprintf("SELECT * FROM %s", table.Table),
		})
	}

	return subqueries, nil
}

// OptimizeFederationPlan applies deterministic defaults and normalization.
func OptimizeFederationPlan(plan *FederatedQueryPlan) *FederatedQueryPlan {
	if plan == nil {
		return nil
	}

	optimized := &FederatedQueryPlan{
		SQL:            plan.SQL,
		Limit:          plan.Limit,
		Offset:         plan.Offset,
		MaxConcurrency: plan.MaxConcurrency,
		Tables:         append([]FederatedTable(nil), plan.Tables...),
		SubQueries:     append([]SubQuery(nil), plan.SubQueries...),
		Joins:          append([]JoinCondition(nil), plan.Joins...),
		Aggregations:   append([]Aggregation(nil), plan.Aggregations...),
	}

	if optimized.Limit <= 0 {
		optimized.Limit = defaultFederationLimit
	}
	if optimized.MaxConcurrency <= 0 {
		maxC := len(optimized.SubQueries)
		if maxC == 0 {
			maxC = 1
		}
		if maxC > defaultFederationConcurrency {
			maxC = defaultFederationConcurrency
		}
		optimized.MaxConcurrency = maxC
	}

	sort.Slice(optimized.SubQueries, func(i, j int) bool {
		left := optimized.SubQueries[i]
		right := optimized.SubQueries[j]
		if left.Profile == right.Profile {
			return left.Alias < right.Alias
		}
		return left.Profile < right.Profile
	})

	for idx := range optimized.Joins {
		optimized.Joins[idx].Type = normalizeJoinType(optimized.Joins[idx].Type)
	}

	return optimized
}

// EstimateFederationCost returns a lightweight cost estimate for diagnostics.
func EstimateFederationCost(plan *FederatedQueryPlan) CostEstimate {
	if plan == nil {
		return CostEstimate{}
	}

	subqueryCount := len(plan.SubQueries)
	if subqueryCount == 0 {
		subqueryCount = len(plan.Tables)
	}
	if subqueryCount == 0 {
		subqueryCount = 1
	}

	estimatedRows := 1000 * subqueryCount
	if plan.Limit > 0 && plan.Limit < estimatedRows {
		estimatedRows = plan.Limit
	}

	joinCount := len(plan.Joins)
	estimatedTime := int64((subqueryCount * 40) + (joinCount * 25))
	transferBytes := int64(estimatedRows * subqueryCount * 64)

	return CostEstimate{
		EstimatedRows:       estimatedRows,
		EstimatedTimeMs:     estimatedTime,
		DataTransferBytes:   transferBytes,
		SubQueriesCount:     subqueryCount,
		JoinOperationsCount: joinCount,
	}
}

// DetermineExecutionOrder computes dependency-aware execution steps.
func DetermineExecutionOrder(subqueries []SubQuery, joins []JoinCondition) []ExecutionStep {
	if len(subqueries) == 0 {
		return nil
	}

	dependencies := make(map[string]map[string]struct{}, len(subqueries))
	for _, sq := range subqueries {
		dependencies[sq.Alias] = make(map[string]struct{})
	}

	// For each join expression, right side depends on left side alias.
	for _, join := range joins {
		leftAlias := aliasFromQualified(join.Left)
		rightAlias := aliasFromQualified(join.Right)
		if leftAlias == "" || rightAlias == "" || leftAlias == rightAlias {
			continue
		}
		if _, ok := dependencies[rightAlias]; ok {
			dependencies[rightAlias][leftAlias] = struct{}{}
		}
	}

	steps := make([]ExecutionStep, 0, len(subqueries))
	for _, sq := range subqueries {
		depList := make([]string, 0, len(dependencies[sq.Alias]))
		for dep := range dependencies[sq.Alias] {
			depList = append(depList, dep)
		}
		sort.Strings(depList)
		steps = append(steps, ExecutionStep{
			Alias:     sq.Alias,
			Profile:   sq.Profile,
			DependsOn: depList,
		})
	}

	return steps
}

func aliasFromQualified(value string) string {
	parts := strings.Split(strings.TrimSpace(value), ".")
	if len(parts) == 2 {
		return parts[0]
	}
	return ""
}
