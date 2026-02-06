package mcp

import "time"

// ColumnProfile contains statistical and quality profiling for a column.
type ColumnProfile struct {
	ColumnName     string             `json:"column_name"`
	DataType       string             `json:"data_type"`
	Statistics     ColumnStatistics   `json:"statistics"`
	Patterns       []PatternMatch     `json:"patterns,omitempty"`
	QualityScore   float64            `json:"quality_score"`
	QualityMetrics DataQualityMetrics `json:"quality_metrics"`
}

// ColumnStatistics holds descriptive statistics for sampled column values.
type ColumnStatistics struct {
	Count       int64       `json:"count"`
	NullCount   int64       `json:"null_count"`
	UniqueCount int64       `json:"unique_count"`
	Min         interface{} `json:"min,omitempty"`
	Max         interface{} `json:"max,omitempty"`
	Mean        float64     `json:"mean,omitempty"`
	Median      float64     `json:"median,omitempty"`
	StdDev      float64     `json:"std_dev,omitempty"`
}

// PatternMatch describes recognized value patterns in sampled data.
type PatternMatch struct {
	Pattern  string  `json:"pattern"`
	Regex    string  `json:"regex"`
	Coverage float64 `json:"coverage"`
	Example  string  `json:"example,omitempty"`
}

// DataQualityMetrics captures quality dimensions in percentage (0-100).
type DataQualityMetrics struct {
	Completeness float64 `json:"completeness"`
	Uniqueness   float64 `json:"uniqueness"`
	Validity     float64 `json:"validity"`
	Consistency  float64 `json:"consistency"`
}

// PatternDefinition defines a known pattern and matching regex.
type PatternDefinition struct {
	Name  string
	Regex string
}

// KnownPatterns lists the built-in semantic value patterns.
var KnownPatterns = []PatternDefinition{
	{Name: "email", Regex: `^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`},
	{Name: "phone", Regex: `^[\+]?[(]?[0-9]{3}[)]?[-\s\.]?[0-9]{3}[-\s\.]?[0-9]{4,6}$`},
	{Name: "uuid", Regex: `^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`},
	{Name: "date_iso", Regex: `^\d{4}-\d{2}-\d{2}`},
	{Name: "url", Regex: `^https?://`},
}

// TableProfile contains profiling output for one table.
type TableProfile struct {
	TableName    string          `json:"table_name"`
	Columns      []ColumnProfile `json:"columns"`
	SampleRowCnt int             `json:"sample_row_count"`
}

// EnhancedSchemaAnalysis wraps advanced profiling output.
type EnhancedSchemaAnalysis struct {
	Enabled       bool           `json:"enabled"`
	MaxWorkers    int            `json:"max_workers"`
	ProfiledAt    time.Time      `json:"profiled_at"`
	TableProfiles []TableProfile `json:"table_profiles,omitempty"`
}
