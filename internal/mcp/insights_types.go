package mcp

import (
	"errors"
	"fmt"
	"time"
)

// InsightType represents the type of insight discovered
type InsightType string

const (
	InsightTypeKPI          InsightType = "kpi"
	InsightTypeTrend        InsightType = "trend"
	InsightTypeAnomaly      InsightType = "anomaly"
	InsightTypeDistribution InsightType = "distribution"
)

// String returns the string representation of InsightType
func (it InsightType) String() string {
	return string(it)
}

// TimeRange represents a time period for trend analysis
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// DistributionBucket represents a single bucket in a distribution
type DistributionBucket struct {
	Min   float64 `json:"min"`
	Max   float64 `json:"max"`
	Count int64   `json:"count"`
}

// DistributionStats contains statistical measures for a distribution
type DistributionStats struct {
	Mean     float64 `json:"mean"`
	Median   float64 `json:"median"`
	StdDev   float64 `json:"std_dev"`
	Skewness float64 `json:"skewness"`
	Kurtosis float64 `json:"kurtosis"`
}

// KPIInsight represents a key performance indicator
type KPIInsight struct {
	Name      string  `json:"name"`
	Value     float64 `json:"value"`
	Unit      string  `json:"unit,omitempty"`
	Benchmark float64 `json:"benchmark,omitempty"`
}

// TrendInsight represents a trend detected in time-series data
type TrendInsight struct {
	Direction  string    `json:"direction"` // upward, downward, stable
	Slope      float64   `json:"slope"`
	Confidence float64   `json:"confidence"` // 0.0 - 1.0
	TimeRange  TimeRange `json:"time_range"`
}

// AnomalyInsight represents a statistical anomaly
type AnomalyInsight struct {
	Column   string  `json:"column"`
	Expected float64 `json:"expected"`
	Actual   float64 `json:"actual"`
	Severity string  `json:"severity"` // low, medium, high, critical
}

// DistributionInsight represents data distribution patterns
type DistributionInsight struct {
	Column  string               `json:"column"`
	Type    string               `json:"type"` // normal, uniform, skewed, bimodal
	Buckets []DistributionBucket `json:"buckets"`
	Stats   DistributionStats    `json:"stats"`
}

// Insight is a union type that can hold any insight type
type Insight struct {
	Type         InsightType          `json:"type"`
	Column       string               `json:"column,omitempty"`
	Description  string               `json:"description"`
	KPI          *KPIInsight          `json:"kpi,omitempty"`
	Trend        *TrendInsight        `json:"trend,omitempty"`
	Anomaly      *AnomalyInsight      `json:"anomaly,omitempty"`
	Distribution *DistributionInsight `json:"distribution,omitempty"`
}

// InsightsSummary provides an overview of discovered insights
type InsightsSummary struct {
	TotalInsights int            `json:"total_insights"`
	ByType        map[string]int `json:"by_type"`
	HighPriority  int            `json:"high_priority"`
}

// DiscoverInsightsRequest represents the input for discovering insights
type DiscoverInsightsRequest struct {
	ProfileName  string        `json:"profile_name"`
	TableName    string        `json:"table_name"`
	Columns      []string      `json:"columns,omitempty"`       // specific columns or all
	InsightTypes []InsightType `json:"insight_types,omitempty"` // filter by type
	MaxResults   int           `json:"max_results,omitempty"`
}

// Validate checks if the request is valid
func (r *DiscoverInsightsRequest) Validate() error {
	if r.ProfileName == "" {
		return errors.New("profile_name is required")
	}
	if r.TableName == "" {
		return errors.New("table_name is required")
	}
	// Validate insight types if specified
	for _, it := range r.InsightTypes {
		switch it {
		case InsightTypeKPI, InsightTypeTrend, InsightTypeAnomaly, InsightTypeDistribution:
			// valid
		default:
			return fmt.Errorf("invalid insight type: %s", it)
		}
	}
	return nil
}

// DiscoverInsightsResult represents the output of insight discovery
type DiscoverInsightsResult struct {
	TableName string          `json:"table_name"`
	Insights  []Insight       `json:"insights"`
	Summary   InsightsSummary `json:"summary"`
}

// ColumnStats contains statistics for a column
type ColumnStats struct {
	Name        string      `json:"name"`
	DataType    string      `json:"data_type"`
	Count       int64       `json:"count"`
	NullCount   int64       `json:"null_count"`
	UniqueCount int64       `json:"unique_count"`
	Min         interface{} `json:"min,omitempty"`
	Max         interface{} `json:"max,omitempty"`
	Mean        float64     `json:"mean,omitempty"`
	Median      float64     `json:"median,omitempty"`
	StdDev      float64     `json:"std_dev,omitempty"`
}
