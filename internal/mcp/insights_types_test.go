package mcp

import (
	"testing"
	"time"
)

func TestInsightType_String(t *testing.T) {
	tests := []struct {
		input    InsightType
		expected string
	}{
		{InsightTypeKPI, "kpi"},
		{InsightTypeTrend, "trend"},
		{InsightTypeAnomaly, "anomaly"},
		{InsightTypeDistribution, "distribution"},
	}

	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			if got := tt.input.String(); got != tt.expected {
				t.Errorf("InsightType.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestDiscoverInsightsRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     DiscoverInsightsRequest
		wantErr bool
	}{
		{
			name: "valid_request",
			req: DiscoverInsightsRequest{
				ProfileName: "test-profile",
				TableName:   "users",
			},
			wantErr: false,
		},
		{
			name: "valid_request_with_insight_types",
			req: DiscoverInsightsRequest{
				ProfileName:  "test-profile",
				TableName:    "users",
				InsightTypes: []InsightType{InsightTypeKPI, InsightTypeTrend},
			},
			wantErr: false,
		},
		{
			name: "missing_profile",
			req: DiscoverInsightsRequest{
				TableName: "users",
			},
			wantErr: true,
		},
		{
			name: "missing_table",
			req: DiscoverInsightsRequest{
				ProfileName: "test-profile",
			},
			wantErr: true,
		},
		{
			name: "invalid_insight_type",
			req: DiscoverInsightsRequest{
				ProfileName:  "test-profile",
				TableName:    "users",
				InsightTypes: []InsightType{"invalid"},
			},
			wantErr: true,
		},
		{
			name:    "empty_request",
			req:     DiscoverInsightsRequest{},
			wantErr: true,
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

func TestInsight_Struct(t *testing.T) {
	// Test that Insight struct can hold different insight types
	tests := []struct {
		name     string
		insight  Insight
		wantType InsightType
	}{
		{
			name: "kpi_insight",
			insight: Insight{
				Type:        InsightTypeKPI,
				Column:      "revenue",
				Description: "Total revenue",
				KPI: &KPIInsight{
					Name:  "total_revenue",
					Value: 1000000.0,
					Unit:  "USD",
				},
			},
			wantType: InsightTypeKPI,
		},
		{
			name: "trend_insight",
			insight: Insight{
				Type:        InsightTypeTrend,
				Column:      "sales",
				Description: "Sales trend",
				Trend: &TrendInsight{
					Direction:  "upward",
					Slope:      50.0,
					Confidence: 0.95,
					TimeRange: TimeRange{
						Start: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
						End:   time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC),
					},
				},
			},
			wantType: InsightTypeTrend,
		},
		{
			name: "anomaly_insight",
			insight: Insight{
				Type:        InsightTypeAnomaly,
				Column:      "temperature",
				Description: "Temperature anomaly detected",
				Anomaly: &AnomalyInsight{
					Column:   "temperature",
					Expected: 22.0,
					Actual:   45.0,
					Severity: "high",
				},
			},
			wantType: InsightTypeAnomaly,
		},
		{
			name: "distribution_insight",
			insight: Insight{
				Type:        InsightTypeDistribution,
				Column:      "age",
				Description: "Age distribution",
				Distribution: &DistributionInsight{
					Column: "age",
					Type:   "normal",
					Buckets: []DistributionBucket{
						{Min: 0, Max: 18, Count: 100},
						{Min: 18, Max: 35, Count: 500},
						{Min: 35, Max: 50, Count: 300},
						{Min: 50, Max: 100, Count: 100},
					},
					Stats: DistributionStats{
						Mean:   35.5,
						Median: 32.0,
						StdDev: 12.5,
					},
				},
			},
			wantType: InsightTypeDistribution,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.insight.Type != tt.wantType {
				t.Errorf("Insight.Type = %v, want %v", tt.insight.Type, tt.wantType)
			}
		})
	}
}

func TestInsightsSummary(t *testing.T) {
	summary := InsightsSummary{
		TotalInsights: 10,
		ByType: map[string]int{
			"kpi":          3,
			"trend":        2,
			"anomaly":      1,
			"distribution": 4,
		},
		HighPriority: 2,
	}

	if summary.TotalInsights != 10 {
		t.Errorf("TotalInsights = %v, want 10", summary.TotalInsights)
	}

	if summary.ByType["kpi"] != 3 {
		t.Errorf("ByType[kpi] = %v, want 3", summary.ByType["kpi"])
	}

	if summary.HighPriority != 2 {
		t.Errorf("HighPriority = %v, want 2", summary.HighPriority)
	}
}
