package mcp

import (
	"context"
	"database-mcp-provider/internal/config"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type executeSubQueryFn func(context.Context, SubQuery, config.Profile) (*SubQueryResult, error)

var federationExecuteSubQuery executeSubQueryFn = ExecuteSubQuery

// ExecuteFederatedQuery runs all plan subqueries, applies joins/aggregations, and returns final rows.
func ExecuteFederatedQuery(
	ctx context.Context,
	plan *FederatedQueryPlan,
	profiles map[string]config.Profile,
) (*FederatedQueryResult, error) {
	if plan == nil {
		return nil, fmt.Errorf("plan is required")
	}
	if len(plan.SubQueries) == 0 {
		return nil, fmt.Errorf("at least one subquery is required")
	}

	start := time.Now()
	maxConcurrency := plan.MaxConcurrency
	if maxConcurrency <= 0 {
		maxConcurrency = defaultFederationConcurrency
	}

	subResults, federationErrors := executeConcurrentlyWithContext(ctx, plan.SubQueries, profiles, maxConcurrency)
	if len(subResults) == 0 {
		if len(federationErrors) > 0 {
			return nil, fmt.Errorf("all subqueries failed: %s", federationErrors[0].Message)
		}
		return nil, fmt.Errorf("no subquery results returned")
	}

	normalized := NormalizeDataTypes(subResults)
	joinedResult, joinErr := executeJoinPipeline(normalized, plan.Joins)
	if joinErr != nil {
		partial, partialErr := HandlePartialFailure(normalized, federationErrors)
		if partialErr != nil {
			return nil, joinErr
		}
		partial.Metadata.ExecutionTimeMs = time.Since(start).Milliseconds()
		return ApplyLimitsAndOffsets(partial, plan.Limit, plan.Offset), nil
	}

	columns := joinedResult.Columns
	rows := joinedResult.Rows
	if len(plan.Aggregations) > 0 {
		aggregated, err := AggregateResults(
			[]SubQueryResult{{Alias: "federated", Profile: "federated", Columns: columns, Rows: rows}},
			plan.Aggregations,
		)
		if err != nil {
			return nil, err
		}
		columns = aggregated.Columns
		rows = aggregated.Rows
	}

	result := &FederatedQueryResult{
		Columns: columns,
		Rows:    rows,
		Metadata: FederationMetadata{
			ExecutionTimeMs: time.Since(start).Milliseconds(),
			RowsFromEach:    rowsFromEach(subResults),
			Errors:          federationErrors,
		},
	}
	return ApplyLimitsAndOffsets(result, plan.Limit, plan.Offset), nil
}

// ExecuteConcurrently executes subqueries using default background context.
func ExecuteConcurrently(
	subqueries []SubQuery,
	profiles map[string]config.Profile,
	maxConcurrency int,
) []SubQueryResult {
	results, _ := executeConcurrentlyWithContext(context.Background(), subqueries, profiles, maxConcurrency)
	return results
}

func executeConcurrentlyWithContext(
	ctx context.Context,
	subqueries []SubQuery,
	profiles map[string]config.Profile,
	maxConcurrency int,
) ([]SubQueryResult, []FederationError) {
	if len(subqueries) == 0 {
		return nil, nil
	}
	if maxConcurrency <= 0 {
		maxConcurrency = defaultFederationConcurrency
	}
	if maxConcurrency > len(subqueries) {
		maxConcurrency = len(subqueries)
	}

	semaphore := make(chan struct{}, maxConcurrency)
	results := make([]*SubQueryResult, len(subqueries))
	errors := make([]FederationError, 0)
	var errorsMu sync.Mutex
	var wg sync.WaitGroup

	appendError := func(err FederationError) {
		errorsMu.Lock()
		errors = append(errors, err)
		errorsMu.Unlock()
	}

	for idx := range subqueries {
		subquery := subqueries[idx]
		wg.Add(1)
		go func(resultIndex int, sq SubQuery) {
			defer wg.Done()

			if ctx.Err() != nil {
				appendError(FederationError{
					Profile: sq.Profile,
					Alias:   sq.Alias,
					Code:    "CONTEXT_CANCELED",
					Message: ctx.Err().Error(),
				})
				return
			}

			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				appendError(FederationError{
					Profile: sq.Profile,
					Alias:   sq.Alias,
					Code:    "CONTEXT_CANCELED",
					Message: ctx.Err().Error(),
				})
				return
			}

			profile, ok := profiles[sq.Profile]
			if !ok {
				appendError(FederationError{
					Profile: sq.Profile,
					Alias:   sq.Alias,
					Code:    "PROFILE_NOT_FOUND",
					Message: fmt.Sprintf("profile %s not found", sq.Profile),
				})
				return
			}

			subResult, err := federationExecuteSubQuery(ctx, sq, profile)
			if err != nil {
				appendError(FederationError{
					Profile: sq.Profile,
					Alias:   sq.Alias,
					Code:    "SUBQUERY_EXECUTION_FAILED",
					Message: err.Error(),
				})
				return
			}
			if subResult == nil {
				appendError(FederationError{
					Profile: sq.Profile,
					Alias:   sq.Alias,
					Code:    "EMPTY_SUBQUERY_RESULT",
					Message: "subquery returned nil result",
				})
				return
			}

			if strings.TrimSpace(subResult.Alias) == "" {
				subResult.Alias = sq.Alias
			}
			if strings.TrimSpace(subResult.Profile) == "" {
				subResult.Profile = sq.Profile
			}
			results[resultIndex] = subResult
		}(idx, subquery)
	}
	wg.Wait()

	finalResults := make([]SubQueryResult, 0, len(subqueries))
	for _, result := range results {
		if result != nil {
			finalResults = append(finalResults, *result)
		}
	}

	sort.Slice(finalResults, func(i, j int) bool {
		if finalResults[i].Profile == finalResults[j].Profile {
			return finalResults[i].Alias < finalResults[j].Alias
		}
		return finalResults[i].Profile < finalResults[j].Profile
	})

	return finalResults, errors
}

// HandlePartialFailure returns a partial success result when at least one subquery succeeded.
func HandlePartialFailure(
	results []SubQueryResult,
	errors []FederationError,
) (*FederatedQueryResult, error) {
	if len(results) == 0 {
		if len(errors) == 0 {
			return nil, fmt.Errorf("no successful subquery results")
		}
		return nil, fmt.Errorf("all subqueries failed")
	}

	unioned := unionWithoutJoin(results)
	return &FederatedQueryResult{
		Columns: unioned.Columns,
		Rows:    unioned.Rows,
		Metadata: FederationMetadata{
			RowsFromEach: rowsFromEach(results),
			Errors:       errors,
		},
	}, nil
}

// ApplyLimitsAndOffsets applies response pagination to federated rows.
func ApplyLimitsAndOffsets(
	result *FederatedQueryResult,
	limit, offset int,
) *FederatedQueryResult {
	if result == nil {
		return nil
	}

	cloned := &FederatedQueryResult{
		Columns: append([]string(nil), result.Columns...),
		Rows:    append([]Row(nil), result.Rows...),
		Metadata: FederationMetadata{
			ExecutionTimeMs: result.Metadata.ExecutionTimeMs,
			RowsFromEach:    copyRowsFromEach(result.Metadata.RowsFromEach),
			Errors:          append([]FederationError(nil), result.Metadata.Errors...),
		},
	}

	if offset < 0 {
		offset = 0
	}
	if limit < 0 {
		limit = 0
	}

	if offset >= len(cloned.Rows) {
		cloned.Rows = []Row{}
		return cloned
	}
	if offset > 0 {
		cloned.Rows = cloned.Rows[offset:]
	}
	if limit > 0 && limit < len(cloned.Rows) {
		cloned.Rows = cloned.Rows[:limit]
	}
	return cloned
}

func executeJoinPipeline(results []SubQueryResult, joins []JoinCondition) (*JoinResult, error) {
	if len(results) == 0 {
		return nil, fmt.Errorf("at least one result is required")
	}
	if len(joins) == 0 {
		unioned := unionWithoutJoin(results)
		return &unioned, nil
	}

	resultByAlias := make(map[string]*SubQueryResult, len(results))
	for idx := range results {
		copyResult := results[idx]
		resultByAlias[copyResult.Alias] = &copyResult
	}

	firstJoin := joins[0]
	leftAlias := aliasFromQualified(firstJoin.Left)
	rightAlias := aliasFromQualified(firstJoin.Right)
	leftResult, leftOK := resultByAlias[leftAlias]
	rightResult, rightOK := resultByAlias[rightAlias]
	if !leftOK || !rightOK {
		return nil, fmt.Errorf("join references unknown aliases: %s <-> %s", leftAlias, rightAlias)
	}

	current, err := PerformJoin(leftResult, rightResult, JoinCondition{
		Left:  columnFromQualified(firstJoin.Left),
		Right: columnFromQualified(firstJoin.Right),
		Type:  firstJoin.Type,
	})
	if err != nil {
		return nil, err
	}

	joinedAliases := map[string]struct{}{
		leftAlias:  {},
		rightAlias: {},
	}

	for _, join := range joins[1:] {
		nextLeftAlias := aliasFromQualified(join.Left)
		nextRightAlias := aliasFromQualified(join.Right)

		switch {
		case isJoined(joinedAliases, nextLeftAlias) && !isJoined(joinedAliases, nextRightAlias):
			rightSide, ok := resultByAlias[nextRightAlias]
			if !ok {
				return nil, fmt.Errorf("join references unknown alias: %s", nextRightAlias)
			}
			leftSide := &SubQueryResult{
				Alias:   "joined",
				Profile: "joined",
				Columns: current.Columns,
				Rows:    current.Rows,
			}
			current, err = PerformJoin(leftSide, rightSide, JoinCondition{
				Left:  columnFromQualified(join.Left),
				Right: columnFromQualified(join.Right),
				Type:  join.Type,
			})
			if err != nil {
				return nil, err
			}
			joinedAliases[nextRightAlias] = struct{}{}

		case !isJoined(joinedAliases, nextLeftAlias) && isJoined(joinedAliases, nextRightAlias):
			leftSide, ok := resultByAlias[nextLeftAlias]
			if !ok {
				return nil, fmt.Errorf("join references unknown alias: %s", nextLeftAlias)
			}
			rightSide := &SubQueryResult{
				Alias:   "joined",
				Profile: "joined",
				Columns: current.Columns,
				Rows:    current.Rows,
			}
			current, err = PerformJoin(leftSide, rightSide, JoinCondition{
				Left:  columnFromQualified(join.Left),
				Right: columnFromQualified(join.Right),
				Type:  join.Type,
			})
			if err != nil {
				return nil, err
			}
			joinedAliases[nextLeftAlias] = struct{}{}

		case isJoined(joinedAliases, nextLeftAlias) && isJoined(joinedAliases, nextRightAlias):
			// Already part of current joined set; skip redundant join.
			continue

		default:
			return nil, fmt.Errorf("join ordering requires at least one previously joined alias (%s <-> %s)", nextLeftAlias, nextRightAlias)
		}
	}

	return current, nil
}

func isJoined(joined map[string]struct{}, alias string) bool {
	_, ok := joined[alias]
	return ok
}

func rowsFromEach(results []SubQueryResult) map[string]int {
	counts := make(map[string]int, len(results))
	for _, result := range results {
		key := result.Alias
		if strings.TrimSpace(key) == "" {
			key = result.Profile
		}
		counts[key] = len(result.Rows)
	}
	return counts
}

func copyRowsFromEach(input map[string]int) map[string]int {
	if len(input) == 0 {
		return map[string]int{}
	}
	output := make(map[string]int, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}
