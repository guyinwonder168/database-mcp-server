package mcp

import (
	"context"
	"database-mcp-provider/internal/config"
	"database-mcp-provider/internal/log"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// DiscoverInsightsParams represents the input parameters for discovering insights
type DiscoverInsightsParams struct {
	ProfileName  string        `json:"profile_name" jsonschema:"profile to use for connection"`
	TableName    string        `json:"table_name" jsonschema:"table to analyze for insights"`
	Columns      []string      `json:"columns,omitempty" jsonschema:"specific columns to analyze (optional)"`
	InsightTypes []InsightType `json:"insight_types,omitempty" jsonschema:"filter by insight type: kpi, trend, anomaly, distribution"`
	MaxResults   int           `json:"max_results,omitempty" jsonschema:"maximum number of insights to return"`
}

// handleDiscoverInsights implements the discover-insights MCP tool
func (s *MCPServer) handleDiscoverInsights(ctx context.Context, _ *mcp.CallToolRequest, input DiscoverInsightsParams) (*mcp.CallToolResult, any, error) {
	p := input

	// Validate required parameters
	if strings.TrimSpace(p.ProfileName) == "" || strings.TrimSpace(p.TableName) == "" {
		structErr := NewStructuredError(
			ErrorCodeMissingParameter,
			"Missing required parameters",
			"profile_name and table_name are required for discover-insights",
		)
		return errorResult(structErr), nil, nil
	}

	log.JSONLog("info", "Starting insight discovery",
		map[string]interface{}{
			"table":   p.TableName,
			"profile": p.ProfileName,
		})

	// Get database connection
	conn, prof, err := s.openConnection(ctx, p.ProfileName, "")
	if err != nil {
		if prof == nil {
			return nil, nil, fmt.Errorf("profile not found: %s", p.ProfileName)
		}
		return nil, nil, err
	}
	defer conn.Close() //nolint:errcheck // Standard pattern: error in deferred close is not critical

	// Get table columns
	columns, err := s.getTableColumns(ctx, conn, prof, p.TableName)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get table columns: %w", err)
	}

	// Filter columns if specified
	if len(p.Columns) > 0 {
		columns = filterColumns(columns, p.Columns)
	}

	if len(columns) == 0 {
		return errorResult(NewStructuredError(
			ErrorCodeMissingParameter,
			"No columns to analyze",
			fmt.Sprintf("No columns found for table %s", p.TableName),
		)), nil, nil
	}

	// Sample data from table
	rows, err := s.sampleTableData(ctx, conn, prof, p.TableName, columns, 1000)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to sample data: %w", err)
	}

	if len(rows) == 0 {
		// Return empty result for empty table (not an error)
		result := DiscoverInsightsResult{
			TableName: p.TableName,
			Insights:  []Insight{},
			Summary: InsightsSummary{
				TotalInsights: 0,
				ByType:        map[string]int{},
				HighPriority:  0,
			},
		}
		b, _ := json.Marshal(result)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
		}, nil, nil
	}

	// Discover insights
	insights, err := s.discoverInsights(columns, rows, p.InsightTypes)
	if err != nil {
		return nil, nil, fmt.Errorf("insight discovery failed: %w", err)
	}

	// Prioritize and limit results
	if p.MaxResults > 0 && len(insights) > p.MaxResults {
		insights = prioritizeInsights(insights, p.MaxResults)
	}

	// Build result
	result := DiscoverInsightsResult{
		TableName: p.TableName,
		Insights:  insights,
		Summary:   buildInsightsSummary(insights),
	}

	b, err := json.Marshal(result)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	log.JSONLog("info", "Insight discovery complete",
		map[string]interface{}{
			"table":          p.TableName,
			"insights_count": len(insights),
		})

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
	}, nil, nil
}

// getTableColumns retrieves column information for a table
func (s *MCPServer) getTableColumns(ctx context.Context, conn *sql.DB, prof *config.Profile, tableName string) ([]ColumnInfo, error) {
	// Use DESCRIBE for MySQL/MariaDB
	// Use INFORMATION_SCHEMA for PostgreSQL
	// Use PRAGMA for SQLite

	var columns []ColumnInfo

	switch prof.DBType {
	case "mysql", "mariadb":
		rows, err := conn.QueryContext(ctx, fmt.Sprintf("DESCRIBE %s", tableName))
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		for rows.Next() {
			var col ColumnInfo
			var nullStr, key, defaultStr, extra string
			if err := rows.Scan(&col.Name, &col.Type, &nullStr, &key, &defaultStr, &extra); err == nil {
				col.Nullable = nullStr == "YES"
				col.Key = key
				col.Extra = extra
				columns = append(columns, col)
			}
		}

	case "postgres":
		rows, err := conn.QueryContext(ctx, `
			SELECT column_name, data_type, is_nullable
			FROM information_schema.columns
			WHERE table_name = $1 AND table_schema = 'public'
		`, tableName)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		for rows.Next() {
			var col ColumnInfo
			var nullStr string
			if err := rows.Scan(&col.Name, &col.Type, &nullStr); err == nil {
				col.Nullable = nullStr == "YES"
				columns = append(columns, col)
			}
		}

	case "sqlite":
		rows, err := conn.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info('%s')", tableName))
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		for rows.Next() {
			var col ColumnInfo
			var cid int
			var notNull int
			var dfltValue interface{}
			if err := rows.Scan(&cid, &col.Name, &col.Type, &notNull, &dfltValue, &col.Key); err == nil {
				col.Nullable = notNull == 0
				columns = append(columns, col)
			}
		}

	default:
		return nil, fmt.Errorf("unsupported db_type: %s", prof.DBType)
	}

	return columns, nil
}

// sampleTableData samples data from a table
func (s *MCPServer) sampleTableData(ctx context.Context, conn *sql.DB, prof *config.Profile, tableName string, columns []ColumnInfo, limit int) ([]map[string]interface{}, error) {
	if limit <= 0 {
		limit = 1000
	}

	// Build column list
	colNames := make([]string, len(columns))
	for i, col := range columns {
		colNames[i] = col.Name
	}

	var query string
	switch prof.DBType {
	case "mysql", "mariadb":
		query = fmt.Sprintf("SELECT %s FROM %s LIMIT %d", strings.Join(colNames, ", "), tableName, limit)
	case "postgres":
		query = fmt.Sprintf("SELECT %s FROM %s LIMIT %d", strings.Join(colNames, ", "), tableName, limit)
	case "sqlite":
		query = fmt.Sprintf("SELECT %s FROM %s LIMIT %d", strings.Join(colNames, ", "), tableName, limit)
	default:
		return nil, fmt.Errorf("unsupported db_type: %s", prof.DBType)
	}

	rows, err := conn.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Get column names from result
	colNamesFromDB, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var result []map[string]interface{}

	for rows.Next() {
		// Create a slice of interface{} to hold the values
		values := make([]interface{}, len(colNamesFromDB))
		valuePtrs := make([]interface{}, len(colNamesFromDB))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			continue
		}

		// Create map from column names to values
		rowMap := make(map[string]interface{})
		for i, name := range colNamesFromDB {
			rowMap[name] = values[i]
		}
		result = append(result, rowMap)
	}

	return result, nil
}

// discoverInsights analyzes data and discovers insights
func (s *MCPServer) discoverInsights(columns []ColumnInfo, rows []map[string]interface{}, insightTypes []InsightType) ([]Insight, error) {
	var insights []Insight

	// If no insight types specified, use all
	if len(insightTypes) == 0 {
		insightTypes = []InsightType{InsightTypeKPI, InsightTypeTrend, InsightTypeAnomaly, InsightTypeDistribution}
	}

	// Build set of requested insight types
	requestedTypes := make(map[InsightType]bool)
	for _, it := range insightTypes {
		requestedTypes[it] = true
	}

	// Find time series columns
	var timeCol *ColumnInfo
	for i := range columns {
		sampleValues := make([]interface{}, 0, min(len(rows), 10))
		for j, row := range rows {
			if j >= 10 {
				break
			}
			if val, ok := row[columns[i].Name]; ok {
				sampleValues = append(sampleValues, val)
			}
		}
		if IsTimeSeriesColumn(columns[i], sampleValues) {
			timeCol = &columns[i]
			break
		}
	}

	// Analyze each column
	for _, col := range columns {
		// Skip time columns for most analyses
		if timeCol != nil && col.Name == timeCol.Name {
			continue
		}

		// KPI Insights
		if requestedTypes[InsightTypeKPI] && isNumericType(col.Type) {
			colInsights, err := s.calculateColumnInsights(col, rows)
			if err == nil {
				insights = append(insights, colInsights...)
			}
		}

		// Trend Insights
		if requestedTypes[InsightTypeTrend] && timeCol != nil && isNumericType(col.Type) {
			trends, err := DetectTrends(timeCol.Name, col.Name, rows)
			if err == nil && len(trends) > 0 {
				for _, trend := range trends {
					insights = append(insights, Insight{
						Type:        InsightTypeTrend,
						Column:      col.Name,
						Description: fmt.Sprintf("Trend detected in %s: %s", col.Name, trend.Direction),
						Trend:       &trend,
					})
				}
			}
		}

		// Anomaly Insights
		if requestedTypes[InsightTypeAnomaly] && isNumericType(col.Type) {
			anomalies, err := DetectAnomalies(col.Name, rows, 2.0)
			if err == nil {
				for _, anomaly := range anomalies {
					insights = append(insights, Insight{
						Type:        InsightTypeAnomaly,
						Column:      col.Name,
						Description: fmt.Sprintf("Anomaly in %s: expected %.2f, got %.2f", col.Name, anomaly.Expected, anomaly.Actual),
						Anomaly:     &anomaly,
					})
				}
			}
		}

		// Distribution Insights
		if requestedTypes[InsightTypeDistribution] && isNumericType(col.Type) {
			dist, err := AnalyzeDistributions(col.Name, rows)
			if err == nil {
				insights = append(insights, Insight{
					Type:         InsightTypeDistribution,
					Column:       col.Name,
					Description:  fmt.Sprintf("Distribution of %s: %s", col.Name, dist.Type),
					Distribution: dist,
				})
			}
		}
	}

	return insights, nil
}

// calculateColumnInsights calculates KPIs for a column
func (s *MCPServer) calculateColumnInsights(col ColumnInfo, rows []map[string]interface{}) ([]Insight, error) {
	values := make([]float64, 0, len(rows))
	for _, row := range rows {
		if val, ok := row[col.Name]; ok && val != nil {
			if f, err := toFloat64(val); err == nil {
				values = append(values, f)
			}
		}
	}

	if len(values) == 0 {
		return nil, fmt.Errorf("no numeric values found")
	}

	kpis := calculateColumnKPIs(col.Name, values)

	var insights []Insight
	for _, kpi := range kpis {
		kpiCopy := kpi // Create a copy to avoid pointer issues
		insights = append(insights, Insight{
			Type:        InsightTypeKPI,
			Column:      col.Name,
			Description: fmt.Sprintf("%s: %.2f %s", kpi.Name, kpi.Value, kpi.Unit),
			KPI:         &kpiCopy,
		})
	}

	return insights, nil
}

// filterColumns filters columns to only include specified names
func filterColumns(columns []ColumnInfo, names []string) []ColumnInfo {
	nameSet := make(map[string]bool)
	for _, name := range names {
		nameSet[name] = true
	}

	var filtered []ColumnInfo
	for _, col := range columns {
		if nameSet[col.Name] {
			filtered = append(filtered, col)
		}
	}
	return filtered
}

// prioritizeInsights sorts insights by significance and returns top N
func prioritizeInsights(insights []Insight, limit int) []Insight {
	// Sort by priority (anomalies first, then trends, then KPIs, then distributions)
	priority := map[InsightType]int{
		InsightTypeAnomaly:      4,
		InsightTypeTrend:        3,
		InsightTypeKPI:          2,
		InsightTypeDistribution: 1,
	}

	// Simple bubble sort for small lists
	for i := 0; i < len(insights); i++ {
		for j := i + 1; j < len(insights); j++ {
			if priority[insights[j].Type] > priority[insights[i].Type] {
				insights[i], insights[j] = insights[j], insights[i]
			}
		}
	}

	if limit > 0 && limit < len(insights) {
		return insights[:limit]
	}
	return insights
}

// buildInsightsSummary creates a summary of insights
func buildInsightsSummary(insights []Insight) InsightsSummary {
	summary := InsightsSummary{
		TotalInsights: len(insights),
		ByType:        make(map[string]int),
	}

	for _, insight := range insights {
		summary.ByType[string(insight.Type)]++

		// Count high priority (anomalies and critical trends)
		if insight.Type == InsightTypeAnomaly {
			if insight.Anomaly != nil && (insight.Anomaly.Severity == "high" || insight.Anomaly.Severity == "critical") {
				summary.HighPriority++
			}
		}
	}

	return summary
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
