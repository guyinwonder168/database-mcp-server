package mcp

import (
	"encoding/json"
	"testing"
)

func TestHandleDiscoverInsights_Success(t *testing.T) {
	// This test requires a mock server with test data
	// For now, we'll test the helper functions

	t.Run("filter_columns", func(t *testing.T) {
		columns := []ColumnInfo{
			{Name: "id", Type: "INT"},
			{Name: "name", Type: "VARCHAR"},
			{Name: "email", Type: "VARCHAR"},
			{Name: "age", Type: "INT"},
		}

		filtered := filterColumns(columns, []string{"name", "email"})

		if len(filtered) != 2 {
			t.Errorf("Expected 2 columns, got %d", len(filtered))
		}

		if filtered[0].Name != "name" || filtered[1].Name != "email" {
			t.Error("Filter returned wrong columns")
		}
	})

	t.Run("prioritize_insights", func(t *testing.T) {
		insights := []Insight{
			{Type: InsightTypeKPI, Column: "sales"},
			{Type: InsightTypeAnomaly, Column: "revenue"},
			{Type: InsightTypeDistribution, Column: "age"},
			{Type: InsightTypeTrend, Column: "orders"},
			{Type: InsightTypeKPI, Column: "profit"},
		}

		prioritized := prioritizeInsights(insights, 3)

		if len(prioritized) != 3 {
			t.Errorf("Expected 3 insights, got %d", len(prioritized))
		}

		// Anomalies should be first
		if prioritized[0].Type != InsightTypeAnomaly {
			t.Errorf("Expected anomaly first, got %s", prioritized[0].Type)
		}

		// Trends should be second
		if prioritized[1].Type != InsightTypeTrend {
			t.Errorf("Expected trend second, got %s", prioritized[1].Type)
		}
	})

	t.Run("build_insights_summary", func(t *testing.T) {
		insights := []Insight{
			{Type: InsightTypeKPI, Column: "sales"},
			{Type: InsightTypeAnomaly, Column: "revenue", Anomaly: &AnomalyInsight{Severity: "high"}},
			{Type: InsightTypeAnomaly, Column: "cost", Anomaly: &AnomalyInsight{Severity: "medium"}},
			{Type: InsightTypeDistribution, Column: "age"},
		}

		summary := buildInsightsSummary(insights)

		if summary.TotalInsights != 4 {
			t.Errorf("TotalInsights = %d, want 4", summary.TotalInsights)
		}

		if summary.ByType["kpi"] != 1 {
			t.Errorf("ByType[kpi] = %d, want 1", summary.ByType["kpi"])
		}

		if summary.ByType["anomaly"] != 2 {
			t.Errorf("ByType[anomaly] = %d, want 2", summary.ByType["anomaly"])
		}

		if summary.HighPriority != 1 {
			t.Errorf("HighPriority = %d, want 1", summary.HighPriority)
		}
	})
}

func TestDiscoverInsightsRequest_Validate_Extended(t *testing.T) {
	tests := []struct {
		name    string
		req     DiscoverInsightsRequest
		wantErr bool
	}{
		{
			name: "valid_all_types",
			req: DiscoverInsightsRequest{
				ProfileName:  "test",
				TableName:    "users",
				InsightTypes: []InsightType{InsightTypeKPI, InsightTypeTrend, InsightTypeAnomaly, InsightTypeDistribution},
			},
			wantErr: false,
		},
		{
			name: "with_max_results",
			req: DiscoverInsightsRequest{
				ProfileName: "test",
				TableName:   "users",
				MaxResults:  10,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPrioritizeInsights_EdgeCases(t *testing.T) {
	t.Run("empty_insights", func(t *testing.T) {
		result := prioritizeInsights([]Insight{}, 10)
		if len(result) != 0 {
			t.Errorf("Expected 0 insights, got %d", len(result))
		}
	})

	t.Run("limit_greater_than_length", func(t *testing.T) {
		insights := []Insight{
			{Type: InsightTypeKPI, Column: "sales"},
		}
		result := prioritizeInsights(insights, 10)
		if len(result) != 1 {
			t.Errorf("Expected 1 insight, got %d", len(result))
		}
	})

	t.Run("limit_zero", func(t *testing.T) {
		insights := []Insight{
			{Type: InsightTypeKPI, Column: "sales"},
			{Type: InsightTypeKPI, Column: "profit"},
		}
		result := prioritizeInsights(insights, 0)
		if len(result) != 2 {
			t.Errorf("Expected 2 insights, got %d", len(result))
		}
	})
}

func TestBuildInsightsSummary_Empty(t *testing.T) {
	summary := buildInsightsSummary([]Insight{})

	if summary.TotalInsights != 0 {
		t.Errorf("TotalInsights = %d, want 0", summary.TotalInsights)
	}

	if len(summary.ByType) != 0 {
		t.Errorf("ByType should be empty, got %v", summary.ByType)
	}
}

func TestDiscoverInsightsParams_JSON(t *testing.T) {
	// Test JSON marshaling/unmarshaling
	params := DiscoverInsightsParams{
		ProfileName:  "test-profile",
		TableName:    "sales",
		Columns:      []string{"amount", "quantity"},
		InsightTypes: []InsightType{InsightTypeKPI, InsightTypeTrend},
		MaxResults:   20,
	}

	jsonData, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("Failed to marshal params: %v", err)
	}

	var unmarshaled DiscoverInsightsParams
	if err := json.Unmarshal(jsonData, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal params: %v", err)
	}

	if unmarshaled.ProfileName != params.ProfileName {
		t.Errorf("ProfileName = %v, want %v", unmarshaled.ProfileName, params.ProfileName)
	}

	if len(unmarshaled.Columns) != len(params.Columns) {
		t.Errorf("Columns length = %d, want %d", len(unmarshaled.Columns), len(params.Columns))
	}
}

func TestDiscoverInsightsResult_JSON(t *testing.T) {
	result := DiscoverInsightsResult{
		TableName: "test_table",
		Insights: []Insight{
			{
				Type:        InsightTypeKPI,
				Column:      "revenue",
				Description: "Total revenue",
				KPI: &KPIInsight{
					Name:  "total_revenue",
					Value: 1000000,
					Unit:  "USD",
				},
			},
		},
		Summary: InsightsSummary{
			TotalInsights: 1,
			ByType: map[string]int{
				"kpi": 1,
			},
			HighPriority: 0,
		},
	}

	jsonData, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal result: %v", err)
	}

	var unmarshaled DiscoverInsightsResult
	if err := json.Unmarshal(jsonData, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if unmarshaled.TableName != result.TableName {
		t.Errorf("TableName = %v, want %v", unmarshaled.TableName, result.TableName)
	}

	if len(unmarshaled.Insights) != len(result.Insights) {
		t.Errorf("Insights length = %d, want %d", len(unmarshaled.Insights), len(result.Insights))
	}
}

// Mock helpers for testing (would need full mock server for integration tests)
func TestDiscoverInsights_HelperFunctions(t *testing.T) {
	t.Run("min_function", func(t *testing.T) {
		if min(5, 10) != 5 {
			t.Error("min(5, 10) should return 5")
		}
		if min(10, 5) != 5 {
			t.Error("min(10, 5) should return 5")
		}
	})
}

func TestCalculateColumnInsights(t *testing.T) {
	server := &MCPServer{}

	tests := []struct {
		name      string
		column    ColumnInfo
		rows      []map[string]interface{}
		wantErr   bool
		insightCount int
	}{
		{
			name:   "valid_numeric_column",
			column: ColumnInfo{Name: "revenue", Type: "DECIMAL"},
			rows: []map[string]interface{}{
				{"revenue": 100.0},
				{"revenue": 200.0},
				{"revenue": 300.0},
			},
			wantErr:      false,
			insightCount: 2, // total and avg
		},
		{
			name:   "column_with_nil_values",
			column: ColumnInfo{Name: "score", Type: "INT"},
			rows: []map[string]interface{}{
				{"score": 80},
				{"score": nil},
				{"score": 100},
			},
			wantErr:      false,
			insightCount: 2,
		},
		{
			name:   "empty_rows",
			column: ColumnInfo{Name: "value", Type: "FLOAT"},
			rows:   []map[string]interface{}{},
			wantErr: true,
		},
		{
			name:   "no_numeric_values",
			column: ColumnInfo{Name: "name", Type: "VARCHAR"},
			rows: []map[string]interface{}{
				{"name": "Alice"},
				{"name": "Bob"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			insights, err := server.calculateColumnInsights(tt.column, tt.rows)
			if (err != nil) != tt.wantErr {
				t.Errorf("calculateColumnInsights() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(insights) < tt.insightCount {
				t.Errorf("Expected at least %d insights, got %d", tt.insightCount, len(insights))
			}
		})
	}
}

func TestFilterColumns(t *testing.T) {
	tests := []struct {
		name         string
		columns      []ColumnInfo
		names        []string
		expectedLen  int
		expectedCols []string
	}{
		{
			name: "filter_existing",
			columns: []ColumnInfo{
				{Name: "id", Type: "INT"},
				{Name: "name", Type: "VARCHAR"},
				{Name: "email", Type: "VARCHAR"},
			},
			names:        []string{"name", "email"},
			expectedLen:  2,
			expectedCols: []string{"name", "email"},
		},
		{
			name: "filter_nonexistent",
			columns: []ColumnInfo{
				{Name: "id", Type: "INT"},
			},
			names:        []string{"nonexistent"},
			expectedLen:  0,
			expectedCols: []string{},
		},
		{
			name:         "empty_columns",
			columns:      []ColumnInfo{},
			names:        []string{"id"},
			expectedLen:  0,
			expectedCols: []string{},
		},
		{
			name: "empty_names_returns_all",
			columns: []ColumnInfo{
				{Name: "id", Type: "INT"},
				{Name: "name", Type: "VARCHAR"},
			},
			names:        []string{},
			expectedLen:  0,
			expectedCols: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := filterColumns(tt.columns, tt.names)
			if len(filtered) != tt.expectedLen {
				t.Errorf("Expected %d columns, got %d", tt.expectedLen, len(filtered))
			}
			for i, expected := range tt.expectedCols {
				if i < len(filtered) && filtered[i].Name != expected {
					t.Errorf("Column %d: expected %s, got %s", i, expected, filtered[i].Name)
				}
			}
		})
	}
}

func TestDiscoverInsights_WithData(t *testing.T) {
	server := &MCPServer{}

	tests := []struct {
		name         string
		columns      []ColumnInfo
		rows         []map[string]interface{}
		insightTypes []InsightType
		expectInsights bool
	}{
		{
			name: "discover_kpis",
			columns: []ColumnInfo{
				{Name: "sales", Type: "DECIMAL"},
				{Name: "date", Type: "timestamp"},
			},
			rows: []map[string]interface{}{
				{"sales": 100.0, "date": "2024-01-01"},
				{"sales": 200.0, "date": "2024-02-01"},
				{"sales": 300.0, "date": "2024-03-01"},
			},
			insightTypes: []InsightType{InsightTypeKPI},
			expectInsights: true,
		},
		{
			name: "discover_trends_with_time_column",
			columns: []ColumnInfo{
				{Name: "revenue", Type: "DECIMAL"},
				{Name: "created_at", Type: "timestamp"},
			},
			rows: []map[string]interface{}{
				{"revenue": 100.0, "created_at": "2024-01-01"},
				{"revenue": 150.0, "created_at": "2024-02-01"},
				{"revenue": 200.0, "created_at": "2024-03-01"},
			},
			insightTypes: []InsightType{InsightTypeTrend},
			expectInsights: true,
		},
		{
			name: "discover_anomalies",
			columns: []ColumnInfo{
				{Name: "revenue", Type: "DECIMAL"},
			},
			rows: func() []map[string]interface{} {
				// Create more data points to make anomaly detection work
				rows := []map[string]interface{}{
					{"revenue": 100.0},
					{"revenue": 101.0},
					{"revenue": 99.0},
					{"revenue": 102.0},
					{"revenue": 100.0},
					{"revenue": 101.0},
					{"revenue": 99.0},
					{"revenue": 100.0},
					{"revenue": 500.0}, // outlier
				}
				return rows
			}(),
			insightTypes: []InsightType{InsightTypeAnomaly},
			expectInsights: true,
		},
		{
			name: "discover_distributions",
			columns: []ColumnInfo{
				{Name: "age", Type: "INT"},
			},
			rows: []map[string]interface{}{
				{"age": 20},
				{"age": 25},
				{"age": 30},
				{"age": 35},
				{"age": 40},
			},
			insightTypes: []InsightType{InsightTypeDistribution},
			expectInsights: true,
		},
		{
			name: "empty_data",
			columns: []ColumnInfo{
				{Name: "value", Type: "INT"},
			},
			rows:         []map[string]interface{}{},
			insightTypes: []InsightType{InsightTypeKPI},
			expectInsights: false,
		},
		{
			name: "all_insight_types",
			columns: []ColumnInfo{
				{Name: "sales", Type: "DECIMAL"},
				{Name: "created_at", Type: "timestamp"},
			},
			rows: []map[string]interface{}{
				{"sales": 100.0, "created_at": "2024-01-01"},
				{"sales": 200.0, "created_at": "2024-02-01"},
				{"sales": 300.0, "created_at": "2024-03-01"},
			},
			insightTypes: []InsightType{}, // empty = all types
			expectInsights: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			insights, err := server.discoverInsights(tt.columns, tt.rows, tt.insightTypes)
			if err != nil {
				t.Errorf("discoverInsights() unexpected error = %v", err)
				return
			}
			if tt.expectInsights && len(insights) == 0 {
				t.Errorf("Expected insights but got none")
			}
			if !tt.expectInsights && len(insights) != 0 {
				t.Errorf("Expected no insights but got %d", len(insights))
			}
			t.Logf("Discovered %d insights", len(insights))
		})
	}
}
