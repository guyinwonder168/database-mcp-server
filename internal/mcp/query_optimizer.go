package mcp

import (
	"context"
	"database-mcp-provider/internal/config"
	"database-mcp-provider/internal/db"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ExplainPlan captures a normalized view of an execution plan across engines.
type ExplainPlan struct {
	DBType  string          `json:"db_type"`
	Steps   []PlanStep      `json:"steps,omitempty"`
	Raw     []interface{}   `json:"raw,omitempty"`      // raw rows for debugging or future enrichments
	RawJSON json.RawMessage `json:"raw_json,omitempty"` // JSON plan when available (Postgres/MySQL)
	Notes   []string        `json:"notes,omitempty"`
}

// PlanStep is a minimal, engine-agnostic execution step.
type PlanStep struct {
	Operation     string                 `json:"operation,omitempty"`
	Target        string                 `json:"target,omitempty"`
	Detail        string                 `json:"detail,omitempty"`
	EstimatedCost float64                `json:"estimated_cost,omitempty"`
	EstimatedRows float64                `json:"estimated_rows,omitempty"`
	Extra         map[string]interface{} `json:"extra,omitempty"`
}

// ExplainWithFindings combines an execution plan with derived optimization findings.
type ExplainWithFindings struct {
	Plan     *ExplainPlan          `json:"plan"`
	Findings []OptimizationFinding `json:"findings,omitempty"`
	Estimate PerformanceEstimation `json:"estimate"`
}

// executeExplain runs an EXPLAIN/EXPLAIN QUERY PLAN for the profile and parses the result.
// It is database-agnostic and uses JSON output when the engine supports it.
func (s *MCPServer) executeExplain(ctx context.Context, prof config.Profile, maxPool int, sqlText string, params []interface{}) (*ExplainPlan, error) {
	stmt, err := buildExplainStatement(strings.ToLower(prof.DBType), sqlText)
	if err != nil {
		return nil, err
	}

	dsn := db.DSN(prof.DBType, prof.Host, prof.Port, prof.Username, prof.Password, prof.DatabaseName, prof.SSLMode)
	conn, err := db.OpenConnectionWithPool(prof.DBType, dsn, maxPool)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	rows, err := conn.QueryContext(ctx, stmt, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns, values, err := collectRows(rows)
	if err != nil {
		return nil, err
	}
	return parseExplainRows(strings.ToLower(prof.DBType), columns, values)
}

// explainWithFindings executes EXPLAIN and runs optimization rules, returning structured results.
func (s *MCPServer) explainWithFindings(ctx context.Context, prof config.Profile, maxPool int, sqlText string, params []interface{}) (*ExplainWithFindings, error) {
	plan, err := s.executeExplain(ctx, prof, maxPool, sqlText, params)
	if err != nil {
		return nil, err
	}
	engine := NewDefaultOptimizationRuleEngine()
	findings := engine.Evaluate(plan)
	return &ExplainWithFindings{
		Plan:     plan,
		Findings: findings,
		Estimate: EstimatePerformance(plan, findings),
	}, nil
}

// buildExplainStatement returns the engine-specific EXPLAIN statement.
func buildExplainStatement(dbType, query string) (string, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return "", errors.New("empty query for EXPLAIN")
	}
	switch dbType {
	case "mysql", "mariadb":
		return fmt.Sprintf("EXPLAIN FORMAT=JSON %s", q), nil
	case "postgres":
		return fmt.Sprintf("EXPLAIN (FORMAT JSON) %s", q), nil
	case "sqlite":
		return fmt.Sprintf("EXPLAIN QUERY PLAN %s", q), nil
	default:
		return "", fmt.Errorf("unsupported db_type for explain: %s", dbType)
	}
}

// collectRows copies sql.Rows into a 2D slice for parsing without holding the DB cursor.
func collectRows(rows *sql.Rows) ([]string, [][]interface{}, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, nil, err
	}
	var data [][]interface{}
	for rows.Next() {
		raw := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range raw {
			ptrs[i] = &raw[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, nil, err
		}
		data = append(data, raw)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return cols, data, nil
}

// parseExplainRows normalizes EXPLAIN output into ExplainPlan.
func parseExplainRows(dbType string, columns []string, rows [][]interface{}) (*ExplainPlan, error) {
	plan := &ExplainPlan{DBType: dbType}
	switch dbType {
	case "postgres":
		return parsePostgresPlan(plan, rows)
	case "mysql", "mariadb":
		return parseMySQLPlan(plan, rows)
	case "sqlite":
		return parseSQLitePlan(plan, columns, rows)
	default:
		return nil, fmt.Errorf("unsupported db_type for plan parsing: %s", dbType)
	}
}

func parsePostgresPlan(plan *ExplainPlan, rows [][]interface{}) (*ExplainPlan, error) {
	if len(rows) == 0 || len(rows[0]) == 0 {
		return plan, errors.New("empty EXPLAIN result")
	}
	raw := normalizeJSONCell(rows[0][0])
	if len(raw) == 0 {
		return plan, errors.New("postgres EXPLAIN returned empty JSON")
	}
	plan.RawJSON = raw
	var parsed []map[string]interface{}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return plan, err
	}
	for _, entry := range parsed {
		if node, ok := entry["Plan"].(map[string]interface{}); ok {
			plan.Steps = append(plan.Steps, walkPostgresPlan(node)...)
		}
	}
	plan.Raw = append(plan.Raw, string(raw))
	return plan, nil
}

func walkPostgresPlan(node map[string]interface{}) []PlanStep {
	step := PlanStep{
		Operation:     asString(node["Node Type"]),
		Target:        asString(node["Relation Name"]),
		Detail:        asString(node["Alias"]),
		EstimatedCost: asFloat(node["Total Cost"]),
		EstimatedRows: asFloat(node["Plan Rows"]),
		Extra:         map[string]interface{}{},
	}
	if filter := node["Filter"]; filter != nil {
		step.Extra["filter"] = filter
	}
	if joinType := node["Join Type"]; joinType != nil {
		step.Extra["join_type"] = joinType
	}
	if idx := node["Index Name"]; idx != nil {
		step.Extra["index_name"] = idx
	}
	steps := []PlanStep{step}
	if children, ok := node["Plans"].([]interface{}); ok {
		for _, child := range children {
			if cm, ok := child.(map[string]interface{}); ok {
				steps = append(steps, walkPostgresPlan(cm)...)
			}
		}
	}
	return steps
}

func parseMySQLPlan(plan *ExplainPlan, rows [][]interface{}) (*ExplainPlan, error) {
	if len(rows) == 0 || len(rows[0]) == 0 {
		return plan, errors.New("empty EXPLAIN result")
	}
	raw := normalizeJSONCell(rows[0][0])
	if len(raw) == 0 {
		return plan, errors.New("mysql EXPLAIN returned empty JSON")
	}
	plan.RawJSON = raw
	var parsed map[string]interface{}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return plan, err
	}
	plan.Steps = append(plan.Steps, walkMySQLBlock(parsed, "query_block")...)
	plan.Raw = append(plan.Raw, string(raw))
	return plan, nil
}

func walkMySQLBlock(node map[string]interface{}, key string) []PlanStep {
	var steps []PlanStep
	block, ok := node[key].(map[string]interface{})
	if !ok {
		// Some MySQL JSON plans are already the block
		block = node
	}
	if costInfo, ok := block["cost_info"].(map[string]interface{}); ok {
		steps = append(steps, PlanStep{
			Operation:     "query_block",
			Detail:        "cost_info",
			EstimatedCost: asFloat(costInfo["query_cost"]),
		})
	}
	if tbl, ok := block["table"].(map[string]interface{}); ok {
		steps = append(steps, PlanStep{
			Operation:     "table",
			Target:        asString(tbl["table_name"]),
			Detail:        asString(tbl["access_type"]),
			EstimatedRows: asFloat(tbl["rows_examined_per_scan"]),
			Extra: map[string]interface{}{
				"attached_condition": tbl["attached_condition"],
				"possible_indexes":   tbl["possible_keys"],
				"used_index":         tbl["key"],
			},
		})
	}
	if nested, ok := block["nested_loop"].([]interface{}); ok {
		for _, child := range nested {
			if cm, ok := child.(map[string]interface{}); ok {
				steps = append(steps, walkMySQLBlock(cm, "table")...)
			}
		}
	}
	return steps
}

func parseSQLitePlan(plan *ExplainPlan, columns []string, rows [][]interface{}) (*ExplainPlan, error) {
	if len(rows) == 0 {
		return plan, errors.New("empty EXPLAIN result")
	}
	colIndex := map[string]int{}
	for i, c := range columns {
		colIndex[strings.ToLower(c)] = i
	}
	detailIdx, ok := colIndex["detail"]
	if !ok {
		return plan, errors.New("sqlite EXPLAIN missing detail column")
	}
	for _, row := range rows {
		detail := asString(row[detailIdx])
		operation, target := parseSQLiteDetail(detail)
		step := PlanStep{
			Operation: operation,
			Target:    target,
			Detail:    detail,
		}
		if selIdx, ok := colIndex["selectid"]; ok && selIdx < len(row) {
			step.Extra = map[string]interface{}{"select_id": row[selIdx]}
		}
		plan.Steps = append(plan.Steps, step)
		plan.Raw = append(plan.Raw, row)
	}
	return plan, nil
}

func parseSQLiteDetail(detail string) (string, string) {
	upper := strings.ToUpper(detail)
	op := "SCAN"
	if strings.Contains(upper, "SEARCH") {
		op = "SEARCH"
	}
	target := ""
	tokens := strings.Fields(detail)
	for i, tok := range tokens {
		if strings.EqualFold(tok, "table") || strings.EqualFold(tok, "index") {
			if i+1 < len(tokens) {
				target = strings.Trim(tokens[i+1], "`\"")
				break
			}
		}
	}
	return op, target
}

// normalizeJSONCell converts a cell that may be string or []byte into json bytes.
func normalizeJSONCell(val interface{}) json.RawMessage {
	switch v := val.(type) {
	case []byte:
		return json.RawMessage(v)
	case string:
		return json.RawMessage(v)
	default:
		b, _ := json.Marshal(v)
		return json.RawMessage(b)
	}
}

func asString(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	default:
		return fmt.Sprintf("%v", t)
	}
}

func asFloat(v interface{}) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case float32:
		return float64(t)
	case int:
		return float64(t)
	case int64:
		return float64(t)
	case json.Number:
		f, _ := t.Float64()
		return f
	case string:
		f, _ := strconv.ParseFloat(t, 64)
		return f
	default:
		return 0
	}
}
