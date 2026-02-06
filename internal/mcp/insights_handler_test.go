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
