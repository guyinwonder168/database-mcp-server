package mcp

import (
	"fmt"
	"strings"
)

const (
	// Join type constants used by the federation engine.
	FederationJoinInner = "INNER"
	FederationJoinLeft  = "LEFT"
	FederationJoinRight = "RIGHT"
	FederationJoinFull  = "FULL"
)

const (
	defaultFederationLimit       = 1000
	defaultFederationConcurrency = 4
	defaultFederationPoolSize    = 5
)

// Row represents one result row from a subquery or federated result.
type Row []interface{}

// FederatedQueryRequest defines input for the federated-query tool.
type FederatedQueryRequest struct {
	SQL            string          `json:"sql,omitempty"`
	SubQueries     []SubQuery      `json:"sub_queries,omitempty"`
	Joins          []JoinCondition `json:"joins,omitempty"`
	Aggregations   []Aggregation   `json:"aggregations,omitempty"`
	Limit          int             `json:"limit,omitempty"`
	Offset         int             `json:"offset,omitempty"`
	MaxConcurrency int             `json:"max_concurrency,omitempty"`
}

// SubQuery defines one query target in federation execution.
type SubQuery struct {
	Profile string `json:"profile"`
	SQL     string `json:"sql"`
	Alias   string `json:"alias"`
}

// JoinCondition defines how two subquery results are joined.
type JoinCondition struct {
	Left  string `json:"left"`           // alias.column
	Right string `json:"right"`          // alias.column
	Type  string `json:"type,omitempty"` // INNER, LEFT, RIGHT, FULL
}

// Aggregation defines a post-processing aggregation over result rows.
type Aggregation struct {
	Function string   `json:"function"`           // COUNT, SUM, AVG, MIN, MAX
	Column   string   `json:"column,omitempty"`   // column name, optional for COUNT(*)
	Alias    string   `json:"alias,omitempty"`    // output column alias
	GroupBy  []string `json:"group_by,omitempty"` // reserved for future grouped aggregations
}

// FederatedQueryResult contains final rows and execution metadata.
type FederatedQueryResult struct {
	Columns  []string           `json:"columns"`
	Rows     []Row              `json:"rows"`
	Metadata FederationMetadata `json:"metadata"`
}

// FederationMetadata stores execution details for federated runs.
type FederationMetadata struct {
	ExecutionTimeMs int64             `json:"execution_time_ms"`
	RowsFromEach    map[string]int    `json:"rows_from_each"`
	Errors          []FederationError `json:"errors,omitempty"`
}

// FederationError represents one subquery execution error.
type FederationError struct {
	Profile string `json:"profile,omitempty"`
	Alias   string `json:"alias,omitempty"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
}

// SubQueryResult is the in-memory result of one executed subquery.
type SubQueryResult struct {
	Profile string   `json:"profile"`
	Alias   string   `json:"alias"`
	Columns []string `json:"columns"`
	Rows    []Row    `json:"rows"`
}

// JoinResult is the output of joining two result sets.
type JoinResult struct {
	Columns []string `json:"columns"`
	Rows    []Row    `json:"rows"`
}

// AggregatedResult is the output of aggregation processing.
type AggregatedResult struct {
	Columns []string `json:"columns"`
	Rows    []Row    `json:"rows"`
}

// FederatedTable describes one table parsed from federated SQL.
type FederatedTable struct {
	Profile string `json:"profile"`
	Table   string `json:"table"`
	Alias   string `json:"alias"`
}

// FederatedQueryPlan is the parsed/optimized execution plan.
type FederatedQueryPlan struct {
	SQL            string           `json:"sql,omitempty"`
	Tables         []FederatedTable `json:"tables,omitempty"`
	SubQueries     []SubQuery       `json:"sub_queries,omitempty"`
	Joins          []JoinCondition  `json:"joins,omitempty"`
	Aggregations   []Aggregation    `json:"aggregations,omitempty"`
	Limit          int              `json:"limit,omitempty"`
	Offset         int              `json:"offset,omitempty"`
	MaxConcurrency int              `json:"max_concurrency,omitempty"`
}

// CostEstimate provides rough execution cost projection for planning/debugging.
type CostEstimate struct {
	EstimatedRows       int   `json:"estimated_rows"`
	EstimatedTimeMs     int64 `json:"estimated_time_ms"`
	DataTransferBytes   int64 `json:"data_transfer_bytes"`
	SubQueriesCount     int   `json:"subqueries_count"`
	JoinOperationsCount int   `json:"join_operations_count"`
}

// ExecutionStep represents one subquery execution step with dependencies.
type ExecutionStep struct {
	Alias     string   `json:"alias"`
	Profile   string   `json:"profile"`
	DependsOn []string `json:"depends_on,omitempty"`
}

func normalizeJoinType(joinType string) string {
	switch strings.ToUpper(strings.TrimSpace(joinType)) {
	case "", FederationJoinInner:
		return FederationJoinInner
	case FederationJoinLeft:
		return FederationJoinLeft
	case FederationJoinRight:
		return FederationJoinRight
	case FederationJoinFull:
		return FederationJoinFull
	default:
		return strings.ToUpper(strings.TrimSpace(joinType))
	}
}

func isSupportedJoinType(joinType string) bool {
	switch normalizeJoinType(joinType) {
	case FederationJoinInner, FederationJoinLeft, FederationJoinRight, FederationJoinFull:
		return true
	default:
		return false
	}
}

func validateJoinCondition(join JoinCondition) error {
	if strings.TrimSpace(join.Left) == "" || strings.TrimSpace(join.Right) == "" {
		return fmt.Errorf("join condition requires both left and right expressions")
	}
	if !isSupportedJoinType(join.Type) {
		return fmt.Errorf("unsupported join type: %s", join.Type)
	}
	return nil
}
