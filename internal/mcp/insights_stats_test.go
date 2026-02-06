package mcp

import (
	"fmt"
	"math"
	"testing"
	"time"
)

func TestDetectTrends(t *testing.T) {
	tests := []struct {
		name     string
		rows     []map[string]interface{}
		expected []TrendInsight
		wantErr  bool
	}{
		{
			name: "upward_trend",
			rows: []map[string]interface{}{
				{"date": "2024-01-01", "sales": 100.0},
				{"date": "2024-02-01", "sales": 150.0},
				{"date": "2024-03-01", "sales": 200.0},
			},
			expected: []TrendInsight{
				{Direction: "upward", Slope: 50.0, Confidence: 1.0},
			},
		},
		{
			name: "downward_trend",
			rows: []map[string]interface{}{
				{"date": "2024-01-01", "sales": 300.0},
				{"date": "2024-02-01", "sales": 200.0},
				{"date": "2024-03-01", "sales": 100.0},
			},
			expected: []TrendInsight{
				{Direction: "downward", Slope: -100.0, Confidence: 1.0},
			},
		},
		{
			name: "no_clear_trend",
			rows: []map[string]interface{}{
				{"date": "2024-01-01", "sales": 100.0},
				{"date": "2024-02-01", "sales": 95.0},
				{"date": "2024-03-01", "sales": 105.0},
				{"date": "2024-04-01", "sales": 98.0},
			},
			expected: []TrendInsight{
				{Direction: "stable", Slope: 0.0},
			},
		},
		{
			name: "insufficient_data",
			rows: []map[string]interface{}{
				{"date": "2024-01-01", "sales": 100.0},
			},
			expected: []TrendInsight{},
			wantErr:  false,
		},
		{
			name:     "empty_data",
			rows:     []map[string]interface{}{},
			expected: []TrendInsight{},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DetectTrends("date", "sales", tt.rows)
			if (err != nil) != tt.wantErr {
				t.Errorf("DetectTrends() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(got) != len(tt.expected) {
				t.Errorf("DetectTrends() returned %d trends, want %d", len(got), len(tt.expected))
				return
			}
			for i, trend := range got {
				if trend.Direction != tt.expected[i].Direction {
					t.Errorf("Trend[%d].Direction = %v, want %v", i, trend.Direction, tt.expected[i].Direction)
				}
			}
		})
	}
}

func TestDetectAnomalies(t *testing.T) {
	tests := []struct {
		name      string
		column    string
		threshold float64
		rows      []map[string]interface{}
		expected  int // number of expected anomalies
	}{
		{
			name:      "detects_high_outliers",
			column:    "revenue",
			threshold: 1.5, // Lower threshold for small datasets
			rows: []map[string]interface{}{
				{"revenue": 100.0},
				{"revenue": 105.0},
				{"revenue": 98.0},
				{"revenue": 102.0},
				{"revenue": 500.0}, // outlier
			},
			expected: 1,
		},
		{
			name:      "no_anomalies",
			column:    "score",
			threshold: 2.0,
			rows: []map[string]interface{}{
				{"score": 90.0},
				{"score": 85.0},
				{"score": 92.0},
				{"score": 88.0},
			},
			expected: 0,
		},
		{
			name:      "empty_data",
			column:    "value",
			threshold: 2.0,
			rows:      []map[string]interface{}{},
			expected:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DetectAnomalies(tt.column, tt.rows, tt.threshold)
			if err != nil {
				t.Errorf("DetectAnomalies() unexpected error = %v", err)
				return
			}
			if len(got) != tt.expected {
				t.Errorf("DetectAnomalies() returned %d anomalies, want %d", len(got), tt.expected)
			}
		})
	}
}

func TestCalculateKPIs(t *testing.T) {
	columns := []ColumnInfo{
		{Name: "revenue", Type: "DECIMAL"},
		{Name: "quantity", Type: "INT"},
	}

	rows := []map[string]interface{}{
		{"revenue": 100.0, "quantity": 5},
		{"revenue": 200.0, "quantity": 10},
		{"revenue": 150.0, "quantity": 7},
	}

	kpis, err := CalculateKPIs("orders", columns, rows)
	if err != nil {
		t.Fatalf("CalculateKPIs() error = %v", err)
	}

	if len(kpis) == 0 {
		t.Error("CalculateKPIs() returned no KPIs")
	}

	t.Logf("Got %d KPIs:", len(kpis))
	for _, kpi := range kpis {
		t.Logf("  - %s: %f %s", kpi.Name, kpi.Value, kpi.Unit)
	}
}

func TestAnalyzeDistributions(t *testing.T) {
	tests := []struct {
		name    string
		column  string
		rows    []map[string]interface{}
		wantErr bool
	}{
		{
			name:   "normal_distribution",
			column: "value",
			rows: []map[string]interface{}{
				{"value": 10.0},
				{"value": 12.0},
				{"value": 11.0},
				{"value": 13.0},
				{"value": 10.5},
				{"value": 11.5},
				{"value": 12.5},
				{"value": 11.0},
				{"value": 10.0},
				{"value": 12.0},
			},
			wantErr: false,
		},
		{
			name:    "empty_data",
			column:  "value",
			rows:    []map[string]interface{}{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := AnalyzeDistributions(tt.column, tt.rows)
			if (err != nil) != tt.wantErr {
				t.Errorf("AnalyzeDistributions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				if got.Column != tt.column {
					t.Errorf("Column = %v, want %v", got.Column, tt.column)
				}
				if len(got.Buckets) != 10 {
					t.Errorf("Expected 10 buckets, got %d", len(got.Buckets))
				}
			}
		})
	}
}

func TestIsTimeSeriesColumn(t *testing.T) {
	tests := []struct {
		name         string
		column       ColumnInfo
		sampleValues []interface{}
		expected     bool
	}{
		{
			name: "timestamp_type",
			column: ColumnInfo{
				Name: "created_at",
				Type: "timestamp",
			},
			expected: true,
		},
		{
			name: "datetime_type",
			column: ColumnInfo{
				Name: "event_time",
				Type: "datetime",
			},
			expected: true,
		},
		{
			name: "date_type",
			column: ColumnInfo{
				Name: "order_date",
				Type: "date",
			},
			expected: true,
		},
		{
			name: "created_pattern",
			column: ColumnInfo{
				Name: "created_timestamp",
				Type: "varchar",
			},
			expected: true,
		},
		{
			name: "sample_values",
			column: ColumnInfo{
				Name: "event_date",
				Type: "varchar",
			},
			sampleValues: []interface{}{
				"2024-01-01",
				"2024-02-01",
				"2024-03-01",
			},
			expected: true,
		},
		{
			name: "not_time_series",
			column: ColumnInfo{
				Name: "name",
				Type: "varchar",
			},
			sampleValues: []interface{}{
				"Alice",
				"Bob",
				"Charlie",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsTimeSeriesColumn(tt.column, tt.sampleValues)
			if got != tt.expected {
				t.Errorf("IsTimeSeriesColumn() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestAnalyzeColumnStats(t *testing.T) {
	column := ColumnInfo{
		Name: "price",
		Type: "DECIMAL",
	}

	rows := []map[string]interface{}{
		{"price": 10.0},
		{"price": 20.0},
		{"price": 30.0},
		{"price": nil},
		{"price": 25.0},
	}

	stats, err := AnalyzeColumnStats(column, rows)
	if err != nil {
		t.Fatalf("AnalyzeColumnStats() error = %v", err)
	}

	if stats.Name != "price" {
		t.Errorf("Name = %v, want price", stats.Name)
	}

	if stats.Count != 5 {
		t.Errorf("Count = %v, want 5", stats.Count)
	}

	if stats.NullCount != 1 {
		t.Errorf("NullCount = %v, want 1", stats.NullCount)
	}

	if stats.UniqueCount != 4 {
		t.Errorf("UniqueCount = %v, want 4", stats.UniqueCount)
	}

	// Check min/max
	if min, ok := stats.Min.(float64); !ok || min != 10.0 {
		t.Errorf("Min = %v, want 10.0", stats.Min)
	}

	if max, ok := stats.Max.(float64); !ok || max != 30.0 {
		t.Errorf("Max = %v, want 30.0", stats.Max)
	}
}

func TestStatisticalHelpers(t *testing.T) {
	t.Run("calculateMean", func(t *testing.T) {
		values := []float64{1, 2, 3, 4, 5}
		mean := calculateMean(values)
		if math.Abs(mean-3.0) > 0.0001 {
			t.Errorf("calculateMean() = %v, want 3.0", mean)
		}
	})

	t.Run("calculateMedian_odd", func(t *testing.T) {
		values := []float64{1, 2, 3, 4, 5}
		median := calculateMedian(values)
		if median != 3.0 {
			t.Errorf("calculateMedian() = %v, want 3.0", median)
		}
	})

	t.Run("calculateMedian_even", func(t *testing.T) {
		values := []float64{1, 2, 3, 4}
		median := calculateMedian(values)
		if median != 2.5 {
			t.Errorf("calculateMedian() = %v, want 2.5", median)
		}
	})

	t.Run("calculateStdDev", func(t *testing.T) {
		values := []float64{2, 4, 4, 4, 5, 5, 7, 9}
		mean := calculateMean(values)
		stdDev := calculateStdDev(values, mean)
		// Expected std dev is approximately 2.138
		if stdDev < 2.0 || stdDev > 2.5 {
			t.Errorf("calculateStdDev() = %v, expected around 2.138", stdDev)
		}
	})

	t.Run("toFloat64", func(t *testing.T) {
		tests := []struct {
			input    interface{}
			expected float64
			wantErr  bool
		}{
			{float64(10.5), 10.5, false},
			{int(10), 10.0, false},
			{int64(10), 10.0, false},
			{"10.5", 10.5, false},
			{"invalid", 0, true},
			{true, 0, true},
		}

		for _, tt := range tests {
			got, err := toFloat64(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("toFloat64(%v) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				continue
			}
			if !tt.wantErr && math.Abs(got-tt.expected) > 0.0001 {
				t.Errorf("toFloat64(%v) = %v, want %v", tt.input, got, tt.expected)
			}
		}
	})

	t.Run("parseTime", func(t *testing.T) {
		tests := []struct {
			input   interface{}
			wantErr bool
		}{
			{time.Now(), false},
			{"2024-01-01", false},
			{"2024-01-01 12:30:45", false},
			{"invalid", true},
			{123, true},
		}

		for _, tt := range tests {
			_, err := parseTime(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseTime(%v) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		}
	})
}

func TestClassifyDistribution(t *testing.T) {
	tests := []struct {
		name   string
		values []float64
	}{
		{
			name: "normal_distribution",
			values: func() []float64 {
				v := make([]float64, 100)
				for i := range v {
					v[i] = float64(i%50) + 50
				}
				return v
			}(),
		},
		{
			name:   "constant_distribution",
			values: []float64{5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5},
		},
		{
			name:   "uniform_distribution",
			values: []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12},
		},
		{
			name:   "skewed_distribution",
			values: []float64{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 100},
		},
		{
			name:   "insufficient_data",
			values: []float64{1, 2, 3, 4, 5},
		},
	}

	validTypes := []string{"normal", "uniform", "skewed", "mixed", "constant", "insufficient_data"}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if len(tt.values) < 2 {
				return
			}
			mean := calculateMean(tt.values)
			stdDev := calculateStdDev(tt.values, mean)

			distType := classifyDistribution(tt.values, mean, stdDev)
			t.Logf("%s classified as: %s", tt.name, distType)

			found := false
			for _, vt := range validTypes {
				if distType == vt {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("classifyDistribution returned invalid type: %s", distType)
			}
		})
	}
}

func TestMinFloat64(t *testing.T) {
	tests := []struct {
		name     string
		values   []float64
		expected float64
	}{
		{
			name:     "normal",
			values:   []float64{5, 2, 8, 1, 9},
			expected: 1,
		},
		{
			name:     "empty",
			values:   []float64{},
			expected: 0,
		},
		{
			name:     "single_value",
			values:   []float64{42},
			expected: 42,
		},
		{
			name:     "negative",
			values:   []float64{-5, -2, -8, -1, -9},
			expected: -9,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := minFloat64(tt.values)
			if result != tt.expected {
				t.Errorf("minFloat64() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestMaxFloat64(t *testing.T) {
	tests := []struct {
		name     string
		values   []float64
		expected float64
	}{
		{
			name:     "normal",
			values:   []float64{5, 2, 8, 1, 9},
			expected: 9,
		},
		{
			name:     "empty",
			values:   []float64{},
			expected: 0,
		},
		{
			name:     "single_value",
			values:   []float64{42},
			expected: 42,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := maxFloat64(tt.values)
			if result != tt.expected {
				t.Errorf("maxFloat64() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		s      string
		substr string
		want   bool
	}{
		{"hello world", "world", true},
		{"hello world", "foo", false},
		{"hello world", "", true},
		{"", "foo", false},
		{"", "", true},
		{"test", "test", true},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("contains_%s_in_%s", tt.substr, tt.s), func(t *testing.T) {
			if got := contains(tt.s, tt.substr); got != tt.want {
				t.Errorf("contains(%q, %q) = %v, want %v", tt.s, tt.substr, got, tt.want)
			}
		})
	}
}

func TestCalculateColumnKPIs(t *testing.T) {
	tests := []struct {
		name       string
		columnName string
		values     []float64
		checkKPIs  []string
	}{
		{
			name:       "revenue_column",
			columnName: "revenue",
			values:     []float64{100, 200, 300},
			checkKPIs:  []string{"total_revenue", "avg_revenue"},
		},
		{
			name:       "quantity_column",
			columnName: "quantity",
			values:     []float64{5, 10, 15},
			checkKPIs:  []string{"total_quantity", "avg_quantity"},
		},
		{
			name:       "generic_column",
			columnName: "score",
			values:     []float64{80, 90, 100},
			checkKPIs:  []string{"total_score", "avg_score", "min_score", "max_score"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kpis := calculateColumnKPIs(tt.columnName, tt.values)

			kpiNames := make(map[string]bool)
			for _, kpi := range kpis {
				kpiNames[kpi.Name] = true
			}

			for _, expectedKPI := range tt.checkKPIs {
				if !kpiNames[expectedKPI] {
					t.Errorf("Expected KPI %s not found", expectedKPI)
				}
			}
		})
	}
}
