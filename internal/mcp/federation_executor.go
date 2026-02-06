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
			executeSubQueryWorker(ctx, sq, resultIndex, profiles, semaphore, results, appendError)
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

func executeSubQueryWorker(
	ctx context.Context,
	sq SubQuery,
	resultIndex int,
	profiles map[string]config.Profile,
	semaphore chan struct{},
	results []*SubQueryResult,
	appendError func(FederationError),
) {
	if err := ctx.Err(); err != nil {
		appendError(contextCanceledError(sq, err))
		return
	}
	if !acquireExecutionSlot(ctx, semaphore, sq, appendError) {
		return
	}
	defer releaseExecutionSlot(semaphore)

	subResult, federationError := runSubQuery(ctx, sq, profiles)
	if federationError != nil {
		appendError(*federationError)
		return
	}
	fillSubQueryIdentity(subResult, sq)
	results[resultIndex] = subResult
}

func acquireExecutionSlot(ctx context.Context, semaphore chan struct{}, sq SubQuery, appendError func(FederationError)) bool {
	select {
	case semaphore <- struct{}{}:
		return true
	case <-ctx.Done():
		appendError(contextCanceledError(sq, ctx.Err()))
		return false
	}
}

func releaseExecutionSlot(semaphore chan struct{}) {
	<-semaphore
}

func runSubQuery(ctx context.Context, sq SubQuery, profiles map[string]config.Profile) (*SubQueryResult, *FederationError) {
	profile, ok := profiles[sq.Profile]
	if !ok {
		err := FederationError{
			Profile: sq.Profile,
			Alias:   sq.Alias,
			Code:    "PROFILE_NOT_FOUND",
			Message: fmt.Sprintf("profile %s not found", sq.Profile),
		}
		return nil, &err
	}

	subResult, executeErr := federationExecuteSubQuery(ctx, sq, profile)
	if executeErr != nil {
		err := FederationError{
			Profile: sq.Profile,
			Alias:   sq.Alias,
			Code:    "SUBQUERY_EXECUTION_FAILED",
			Message: executeErr.Error(),
		}
		return nil, &err
	}
	if subResult == nil {
		err := FederationError{
			Profile: sq.Profile,
			Alias:   sq.Alias,
			Code:    "EMPTY_SUBQUERY_RESULT",
			Message: "subquery returned nil result",
		}
		return nil, &err
	}
	return subResult, nil
}

func contextCanceledError(sq SubQuery, err error) FederationError {
	message := "context canceled"
	if err != nil {
		message = err.Error()
	}
	return FederationError{
		Profile: sq.Profile,
		Alias:   sq.Alias,
		Code:    "CONTEXT_CANCELED",
		Message: message,
	}
}

func fillSubQueryIdentity(subResult *SubQueryResult, sq SubQuery) {
	if strings.TrimSpace(subResult.Alias) == "" {
		subResult.Alias = sq.Alias
	}
	if strings.TrimSpace(subResult.Profile) == "" {
		subResult.Profile = sq.Profile
	}
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

	resultByAlias := mapResultsByAlias(results)
	current, joinedAliases, err := initializeJoinState(resultByAlias, joins[0])
	if err != nil {
		return nil, err
	}

	for _, join := range joins[1:] {
		current, err = applyJoinStep(current, join, joinedAliases, resultByAlias)
		if err != nil {
			return nil, err
		}
	}

	return current, nil
}

func mapResultsByAlias(results []SubQueryResult) map[string]*SubQueryResult {
	resultByAlias := make(map[string]*SubQueryResult, len(results))
	for idx := range results {
		copyResult := results[idx]
		resultByAlias[copyResult.Alias] = &copyResult
	}
	return resultByAlias
}

func initializeJoinState(resultByAlias map[string]*SubQueryResult, firstJoin JoinCondition) (*JoinResult, map[string]struct{}, error) {
	leftAlias := aliasFromQualified(firstJoin.Left)
	rightAlias := aliasFromQualified(firstJoin.Right)
	leftResult, leftOK := resultByAlias[leftAlias]
	rightResult, rightOK := resultByAlias[rightAlias]
	if !leftOK || !rightOK {
		return nil, nil, fmt.Errorf("join references unknown aliases: %s <-> %s", leftAlias, rightAlias)
	}
	current, err := PerformJoin(leftResult, rightResult, normalizedJoinCondition(firstJoin))
	if err != nil {
		return nil, nil, err
	}
	return current, map[string]struct{}{
		leftAlias:  {},
		rightAlias: {},
	}, nil
}

func applyJoinStep(
	current *JoinResult,
	join JoinCondition,
	joinedAliases map[string]struct{},
	resultByAlias map[string]*SubQueryResult,
) (*JoinResult, error) {
	leftAlias := aliasFromQualified(join.Left)
	rightAlias := aliasFromQualified(join.Right)

	switch {
	case isJoined(joinedAliases, leftAlias) && !isJoined(joinedAliases, rightAlias):
		return joinWithAlias(current, join, rightAlias, true, joinedAliases, resultByAlias)
	case !isJoined(joinedAliases, leftAlias) && isJoined(joinedAliases, rightAlias):
		return joinWithAlias(current, join, leftAlias, false, joinedAliases, resultByAlias)
	case isJoined(joinedAliases, leftAlias) && isJoined(joinedAliases, rightAlias):
		return current, nil
	default:
		return nil, fmt.Errorf("join ordering requires at least one previously joined alias (%s <-> %s)", leftAlias, rightAlias)
	}
}

func joinWithAlias(
	current *JoinResult,
	join JoinCondition,
	alias string,
	currentIsLeft bool,
	joinedAliases map[string]struct{},
	resultByAlias map[string]*SubQueryResult,
) (*JoinResult, error) {
	nextSide, ok := resultByAlias[alias]
	if !ok {
		return nil, fmt.Errorf("join references unknown alias: %s", alias)
	}
	joined := joinedSubQuery(current)
	condition := normalizedJoinCondition(join)
	left := joined
	right := nextSide
	if !currentIsLeft {
		left = nextSide
		right = joined
	}

	result, err := PerformJoin(left, right, condition)
	if err != nil {
		return nil, err
	}
	joinedAliases[alias] = struct{}{}
	return result, nil
}

func joinedSubQuery(current *JoinResult) *SubQueryResult {
	return &SubQueryResult{
		Alias:   "joined",
		Profile: "joined",
		Columns: current.Columns,
		Rows:    current.Rows,
	}
}

func normalizedJoinCondition(join JoinCondition) JoinCondition {
	return JoinCondition{
		Left:  columnFromQualified(join.Left),
		Right: columnFromQualified(join.Right),
		Type:  join.Type,
	}
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
