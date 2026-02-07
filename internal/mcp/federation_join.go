package mcp

import (
	"context"
	"database-mcp-provider/internal/config"
	"database-mcp-provider/internal/db"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

const federationSerializedRowColumn = "row_json"

// ExecuteSubQuery executes one subquery against its profile and returns rows/columns.
func ExecuteSubQuery(ctx context.Context, subquery SubQuery, profile config.Profile) (*SubQueryResult, error) {
	if strings.TrimSpace(subquery.SQL) == "" {
		return nil, fmt.Errorf("subquery sql is required")
	}
	if !isFederationReadOnlySQL(subquery.SQL) {
		return nil, fmt.Errorf("only read-only statements are allowed in federated-query subqueries")
	}

	dsn := db.DSN(
		profile.DBType,
		profile.Host,
		profile.Port,
		profile.Username,
		profile.Password,
		profile.DatabaseName,
		profile.SSLMode,
	)
	conn, err := db.OpenConnectionWithPool(ctx, profile.DBType, dsn, defaultFederationPoolSize)
	if err != nil {
		return nil, fmt.Errorf("open connection for profile %s: %w", subquery.Profile, err)
	}
	defer conn.Close() //nolint:errcheck // Standard pattern: close error is non-critical for response lifecycle

	rows, err := conn.QueryContext(ctx, subquery.SQL)
	if err != nil {
		return nil, fmt.Errorf("execute subquery for profile %s: %w", subquery.Profile, err)
	}
	defer rows.Close() //nolint:errcheck // Standard pattern: close error is non-critical for response lifecycle

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("read result columns for profile %s: %w", subquery.Profile, err)
	}

	resultRows := make([]Row, 0)
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for idx := range values {
			valuePtrs[idx] = &values[idx]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("scan subquery row for profile %s: %w", subquery.Profile, err)
		}

		row := make(Row, len(values))
		copy(row, values)
		resultRows = append(resultRows, row)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate subquery rows for profile %s: %w", subquery.Profile, err)
	}

	return &SubQueryResult{
		Profile: subquery.Profile,
		Alias:   subquery.Alias,
		Columns: columns,
		Rows:    resultRows,
	}, nil
}

// PerformJoin executes a join using hash-join strategy.
func PerformJoin(left, right *SubQueryResult, join JoinCondition) (*JoinResult, error) {
	return PerformHashJoin(left, right, join)
}

// PerformHashJoin joins two result sets by hash index on the right side key.
func PerformHashJoin(left, right *SubQueryResult, join JoinCondition) (*JoinResult, error) {
	if left == nil || right == nil {
		return nil, fmt.Errorf("both left and right results are required")
	}
	if err := validateJoinCondition(join); err != nil {
		return nil, err
	}

	leftColumn := columnFromQualified(join.Left)
	rightColumn := columnFromQualified(join.Right)
	leftIndex, rightIndex, err := resolveJoinColumnIndexes(left.Columns, right.Columns, leftColumn, rightColumn)
	if err != nil {
		return nil, err
	}
	joinType := normalizeJoinType(join.Type)
	rightMap := buildRightRowIndexMap(right.Rows, rightIndex)
	rows, matchedRight := joinLeftRows(left.Rows, right.Rows, leftIndex, rightMap, joinType, len(right.Columns))
	if requiresUnmatchedRightRows(joinType) {
		rows = append(rows, unmatchedRightRows(right.Rows, matchedRight, len(left.Columns))...)
	}
	columns := append(append([]string(nil), left.Columns...), right.Columns...)
	return &JoinResult{Columns: columns, Rows: rows}, nil
}

// NormalizeDataTypes converts driver-specific values to stable JSON-friendly types.
func NormalizeDataTypes(results []SubQueryResult) []SubQueryResult {
	normalized := make([]SubQueryResult, len(results))
	for idx, result := range results {
		copied := SubQueryResult{
			Profile: result.Profile,
			Alias:   result.Alias,
			Columns: append([]string(nil), result.Columns...),
			Rows:    make([]Row, len(result.Rows)),
		}
		for rowIdx, row := range result.Rows {
			normalizedRow := make(Row, len(row))
			for colIdx, value := range row {
				normalizedRow[colIdx] = normalizeDataValue(value)
			}
			copied.Rows[rowIdx] = normalizedRow
		}
		normalized[idx] = copied
	}
	return normalized
}

// AggregateResults applies optional aggregate functions across result rows.
func AggregateResults(results []SubQueryResult, aggregations []Aggregation) (*AggregatedResult, error) {
	if len(results) == 0 {
		return nil, fmt.Errorf("at least one result set is required")
	}

	combined := unionWithoutJoin(results)
	if len(aggregations) == 0 {
		return &AggregatedResult{Columns: combined.Columns, Rows: combined.Rows}, nil
	}

	current := AggregatedResult(combined)
	for _, aggregation := range aggregations {
		next, err := applyAggregation(current, aggregation)
		if err != nil {
			return nil, err
		}
		current = *next
	}
	return &current, nil
}

func isFederationReadOnlySQL(sqlText string) bool {
	trimmed := strings.TrimSpace(sqlText)
	if trimmed == "" {
		return false
	}
	lower := strings.ToLower(trimmed)
	if strings.HasSuffix(lower, ";") {
		lower = strings.TrimSpace(strings.TrimSuffix(lower, ";"))
	}
	if strings.Contains(lower, ";") {
		return false
	}

	allowedStarts := []string{"select", "with", "show", "describe", "explain", "pragma"}
	allowed := false
	for _, start := range allowedStarts {
		if strings.HasPrefix(lower, start) {
			allowed = true
			break
		}
	}
	if !allowed {
		return false
	}

	disallowed := []string{
		" insert ", " update ", " delete ", " alter ", " create ", " drop ",
		" truncate ", " grant ", " revoke ", " attach ", " detach ", " vacuum ",
	}
	padded := " " + lower + " "
	for _, token := range disallowed {
		if strings.Contains(padded, token) {
			return false
		}
	}
	return true
}

func applyAggregation(result AggregatedResult, aggregation Aggregation) (*AggregatedResult, error) {
	function, alias, columnIndex, err := resolveAggregationSpec(result.Columns, aggregation)
	if err != nil {
		return nil, err
	}
	if function == "COUNT" {
		return aggregationResult(alias, aggregationCount(result.Rows, columnIndex)), nil
	}
	if !isNumericAggregation(function) {
		return nil, fmt.Errorf("unsupported aggregation function: %s", aggregation.Function)
	}
	if columnIndex == -1 {
		return nil, fmt.Errorf("%s requires a numeric column", function)
	}
	values := collectNumericValues(result.Rows, columnIndex)
	if len(values) == 0 {
		return aggregationResult(alias, 0), nil
	}
	return aggregationResult(alias, applyNumericAggregation(function, values)), nil
}

func resolveJoinColumnIndexes(leftColumns, rightColumns []string, leftColumn, rightColumn string) (int, int, error) {
	leftIndex := findColumnIndex(leftColumns, leftColumn)
	rightIndex := findColumnIndex(rightColumns, rightColumn)
	if leftIndex == -1 || rightIndex == -1 {
		return -1, -1, fmt.Errorf("join columns not found (left=%s right=%s)", leftColumn, rightColumn)
	}
	return leftIndex, rightIndex, nil
}

func buildRightRowIndexMap(rows []Row, rightIndex int) map[string][]int {
	rightMap := make(map[string][]int, len(rows))
	for idx, row := range rows {
		key := normalizeJoinKey(rowValue(row, rightIndex))
		rightMap[key] = append(rightMap[key], idx)
	}
	return rightMap
}

func joinLeftRows(
	leftRows []Row,
	rightRows []Row,
	leftIndex int,
	rightMap map[string][]int,
	joinType string,
	rightColumnCount int,
) ([]Row, map[int]struct{}) {
	rows := make([]Row, 0)
	matchedRight := make(map[int]struct{}, len(rightRows))
	rightNulls := nilRow(rightColumnCount)

	for _, leftRow := range leftRows {
		key := normalizeJoinKey(rowValue(leftRow, leftIndex))
		rightMatches := rightMap[key]
		if len(rightMatches) == 0 {
			if joinType == FederationJoinLeft || joinType == FederationJoinFull {
				rows = append(rows, mergeRows(leftRow, rightNulls))
			}
			continue
		}
		for _, rightRowIdx := range rightMatches {
			rows = append(rows, mergeRows(leftRow, rightRows[rightRowIdx]))
			matchedRight[rightRowIdx] = struct{}{}
		}
	}

	return rows, matchedRight
}

func requiresUnmatchedRightRows(joinType string) bool {
	return joinType == FederationJoinRight || joinType == FederationJoinFull
}

func unmatchedRightRows(rightRows []Row, matchedRight map[int]struct{}, leftColumnCount int) []Row {
	leftNulls := nilRow(leftColumnCount)
	rows := make([]Row, 0)
	for idx, rightRow := range rightRows {
		if _, alreadyMatched := matchedRight[idx]; alreadyMatched {
			continue
		}
		rows = append(rows, mergeRows(leftNulls, rightRow))
	}
	return rows
}

func resolveAggregationSpec(columns []string, aggregation Aggregation) (string, string, int, error) {
	function := strings.ToUpper(strings.TrimSpace(aggregation.Function))
	if function == "" {
		return "", "", -1, fmt.Errorf("aggregation function is required")
	}
	if len(aggregation.GroupBy) > 0 {
		return "", "", -1, fmt.Errorf("group_by aggregations are not supported yet")
	}

	alias := strings.TrimSpace(aggregation.Alias)
	if alias == "" {
		alias = strings.ToLower(function)
	}

	columnName := columnFromQualified(aggregation.Column)
	columnIndex := -1
	if columnName != "" && columnName != "*" {
		columnIndex = findColumnIndex(columns, columnName)
		if columnIndex == -1 {
			return "", "", -1, fmt.Errorf("aggregation column not found: %s", columnName)
		}
	}
	return function, alias, columnIndex, nil
}

func isNumericAggregation(function string) bool {
	return function == "SUM" || function == "AVG" || function == "MIN" || function == "MAX"
}

func aggregationResult(alias string, value interface{}) *AggregatedResult {
	return &AggregatedResult{
		Columns: []string{alias},
		Rows:    []Row{{value}},
	}
}

func applyNumericAggregation(function string, values []float64) interface{} {
	switch function {
	case "SUM":
		return sumFloat64(values)
	case "AVG":
		return sumFloat64(values) / float64(len(values))
	case "MIN":
		return minValue(values)
	case "MAX":
		return maxValue(values)
	default:
		return 0
	}
}

func sumFloat64(values []float64) float64 {
	sum := 0.0
	for _, value := range values {
		sum += value
	}
	return sum
}

func minValue(values []float64) float64 {
	min := values[0]
	for _, value := range values[1:] {
		if value < min {
			min = value
		}
	}
	return min
}

func maxValue(values []float64) float64 {
	max := values[0]
	for _, value := range values[1:] {
		if value > max {
			max = value
		}
	}
	return max
}

func aggregationCount(rows []Row, columnIndex int) int {
	if columnIndex < 0 {
		return len(rows)
	}
	count := 0
	for _, row := range rows {
		value := rowValue(row, columnIndex)
		if value != nil {
			count++
		}
	}
	return count
}

func collectNumericValues(rows []Row, columnIndex int) []float64 {
	values := make([]float64, 0, len(rows))
	for _, row := range rows {
		value := rowValue(row, columnIndex)
		floatValue, ok := federationToFloat64(value)
		if ok {
			values = append(values, floatValue)
		}
	}
	return values
}

func findColumnIndex(columns []string, column string) int {
	target := strings.TrimSpace(column)
	for idx, name := range columns {
		if strings.EqualFold(name, target) {
			return idx
		}
	}
	return -1
}

func columnFromQualified(expression string) string {
	trimmed := strings.TrimSpace(expression)
	parts := strings.Split(trimmed, ".")
	if len(parts) == 0 {
		return trimmed
	}
	return parts[len(parts)-1]
}

func normalizeJoinKey(value interface{}) string {
	if value == nil {
		return "__nil__"
	}
	return fmt.Sprintf("%v", normalizeDataValue(value))
}

func normalizeDataValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case nil:
		return nil
	case []byte:
		return string(typed)
	case int:
		return int64(typed)
	case int8:
		return int64(typed)
	case int16:
		return int64(typed)
	case int32:
		return int64(typed)
	case int64:
		return typed
	case uint:
		return normalizeUnsigned(uint64(typed))
	case uint8:
		return normalizeUnsigned(uint64(typed))
	case uint16:
		return normalizeUnsigned(uint64(typed))
	case uint32:
		return normalizeUnsigned(uint64(typed))
	case uint64:
		return normalizeUnsigned(typed)
	case float32:
		return float64(typed)
	case float64:
		return typed
	case time.Time:
		return typed.UTC().Format(time.RFC3339Nano)
	default:
		return typed
	}
}

func normalizeUnsigned(value uint64) interface{} {
	if value > math.MaxInt64 {
		return float64(value)
	}
	return int64(value)
}

func rowValue(row Row, index int) interface{} {
	if index < 0 || index >= len(row) {
		return nil
	}
	return row[index]
}

func nilRow(size int) Row {
	row := make(Row, size)
	for idx := range row {
		row[idx] = nil
	}
	return row
}

func mergeRows(left, right Row) Row {
	merged := make(Row, 0, len(left)+len(right))
	merged = append(merged, left...)
	merged = append(merged, right...)
	return merged
}

func federationToFloat64(value interface{}) (float64, bool) {
	switch typed := normalizeDataValue(value).(type) {
	case int64:
		return float64(typed), true
	case float64:
		return typed, true
	case string:
		parsed, err := strconv.ParseFloat(typed, 64)
		if err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}

func unionWithoutJoin(results []SubQueryResult) JoinResult {
	if len(results) == 0 {
		return JoinResult{}
	}
	if len(results) == 1 {
		return JoinResult{
			Columns: append([]string(nil), results[0].Columns...),
			Rows:    append([]Row(nil), results[0].Rows...),
		}
	}

	columnsAligned := true
	baseColumns := results[0].Columns
	for _, result := range results[1:] {
		if !equalColumns(baseColumns, result.Columns) {
			columnsAligned = false
			break
		}
	}

	if columnsAligned {
		rows := make([]Row, 0)
		for _, result := range results {
			rows = append(rows, result.Rows...)
		}
		return JoinResult{
			Columns: append([]string(nil), baseColumns...),
			Rows:    rows,
		}
	}

	rows := make([]Row, 0)
	for _, result := range results {
		for _, row := range result.Rows {
			serializedRow := make(map[string]interface{}, len(result.Columns))
			for colIdx, column := range result.Columns {
				serializedRow[column] = rowValue(row, colIdx)
			}
			payload, _ := json.Marshal(serializedRow)
			rows = append(rows, Row{result.Profile, result.Alias, string(payload)})
		}
	}

	return JoinResult{
		Columns: []string{"source_profile", "source_alias", federationSerializedRowColumn},
		Rows:    rows,
	}
}

func equalColumns(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for idx := range left {
		if !strings.EqualFold(left[idx], right[idx]) {
			return false
		}
	}
	return true
}
