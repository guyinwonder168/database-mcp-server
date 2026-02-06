package mcp

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// AnalyzeColumnStats calculates statistics for a column
func AnalyzeColumnStats(column ColumnInfo, rows []map[string]interface{}) (*ColumnStats, error) {
	if len(rows) == 0 {
		return &ColumnStats{
			Name:     column.Name,
			DataType: column.Type,
			Count:    0,
		}, nil
	}

	stats := &ColumnStats{
		Name:     column.Name,
		DataType: column.Type,
		Count:    int64(len(rows)),
	}

	// Extract values and calculate basic stats
	values := make([]interface{}, 0, len(rows))
	uniqueValues := make(map[interface{}]struct{})

	for _, row := range rows {
		if val, ok := row[column.Name]; ok && val != nil {
			values = append(values, val)
			uniqueValues[val] = struct{}{}
		} else {
			stats.NullCount++
		}
	}

	stats.UniqueCount = int64(len(uniqueValues))

	// Calculate numeric stats if applicable
	if isNumericType(column.Type) && len(values) > 0 {
		numericValues := make([]float64, 0, len(values))
		for _, v := range values {
			if f, err := toFloat64(v); err == nil {
				numericValues = append(numericValues, f)
			}
		}

		if len(numericValues) > 0 {
			stats.Min = minFloat64(numericValues)
			stats.Max = maxFloat64(numericValues)
			stats.Mean = calculateMean(numericValues)
			stats.Median = calculateMedian(numericValues)
			stats.StdDev = calculateStdDev(numericValues, stats.Mean)
		}
	}

	return stats, nil
}

// DetectTrends identifies trends in time-series data
func DetectTrends(timeColumn string, valueColumn string, rows []map[string]interface{}) ([]TrendInsight, error) {
	if len(rows) < 2 {
		return []TrendInsight{}, nil
	}

	// Extract time-value pairs
	type dataPoint struct {
		time  time.Time
		value float64
	}

	points := make([]dataPoint, 0, len(rows))
	var minTime, maxTime time.Time

	for _, row := range rows {
		timeVal, ok := row[timeColumn]
		if !ok || timeVal == nil {
			continue
		}

		valueVal, ok := row[valueColumn]
		if !ok || valueVal == nil {
			continue
		}

		t, err := parseTime(timeVal)
		if err != nil {
			continue
		}

		v, err := toFloat64(valueVal)
		if err != nil {
			continue
		}

		points = append(points, dataPoint{time: t, value: v})

		if minTime.IsZero() || t.Before(minTime) {
			minTime = t
		}
		if maxTime.IsZero() || t.After(maxTime) {
			maxTime = t
		}
	}

	if len(points) < 2 {
		return []TrendInsight{}, nil
	}

	// Sort by time
	sort.Slice(points, func(i, j int) bool {
		return points[i].time.Before(points[j].time)
	})

	// Calculate linear regression
	n := float64(len(points))
	sumX, sumY, sumXY, sumX2 := 0.0, 0.0, 0.0, 0.0

	for i, p := range points {
		x := float64(i)
		y := p.value
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	// Slope = (n*sumXY - sumX*sumY) / (n*sumX2 - sumX*sumX)
	denominator := n*sumX2 - sumX*sumX
	if denominator == 0 {
		return []TrendInsight{}, nil
	}

	slope := (n*sumXY - sumX*sumY) / denominator

	// Calculate R-squared for confidence
	meanY := sumY / n
	ssTotal, ssResidual := 0.0, 0.0
	for i, p := range points {
		x := float64(i)
		predicted := (sumY / n) + slope*(x-(sumX/n))
		ssTotal += (p.value - meanY) * (p.value - meanY)
		ssResidual += (p.value - predicted) * (p.value - predicted)
	}

	var rSquared float64
	if ssTotal > 0 {
		rSquared = 1 - (ssResidual / ssTotal)
	}
	confidence := math.Max(0, math.Min(1, rSquared))

	// Determine direction
	direction := "stable"
	threshold := 0.001 * (points[len(points)-1].value - points[0].value)
	if math.Abs(threshold) < 0.0001 {
		threshold = 0.001
	}

	if slope > threshold {
		direction = "upward"
	} else if slope < -threshold {
		direction = "downward"
	}

	// If confidence is very low, mark as stable
	if confidence < 0.3 {
		direction = "stable"
		slope = 0
	}

	return []TrendInsight{
		{
			Direction:  direction,
			Slope:      slope,
			Confidence: confidence,
			TimeRange: TimeRange{
				Start: minTime,
				End:   maxTime,
			},
		},
	}, nil
}

// DetectAnomalies finds statistical outliers using Z-score
func DetectAnomalies(column string, rows []map[string]interface{}, threshold float64) ([]AnomalyInsight, error) {
	if len(rows) == 0 {
		return []AnomalyInsight{}, nil
	}

	// Extract numeric values
	values := make([]float64, 0, len(rows))
	rowMap := make(map[int]map[string]interface{})

	for i, row := range rows {
		rowMap[i] = row
		if val, ok := row[column]; ok && val != nil {
			if f, err := toFloat64(val); err == nil {
				values = append(values, f)
			}
		}
	}

	if len(values) == 0 {
		return []AnomalyInsight{}, nil
	}

	// Calculate mean and standard deviation
	mean := calculateMean(values)
	stdDev := calculateStdDev(values, mean)

	if stdDev == 0 {
		return []AnomalyInsight{}, nil
	}

	// Find anomalies using Z-score
	var anomalies []AnomalyInsight
	for i, row := range rows {
		val, ok := row[column]
		if !ok || val == nil {
			continue
		}

		f, err := toFloat64(val)
		if err != nil {
			continue
		}

		zScore := math.Abs((f - mean) / stdDev)
		if zScore > threshold {
			// Determine severity based on Z-score
			severity := "low"
			if zScore > 4.0 {
				severity = "critical"
			} else if zScore > 3.0 {
				severity = "high"
			} else if zScore > 2.5 {
				severity = "medium"
			}

			anomalies = append(anomalies, AnomalyInsight{
				Column:   column,
				Expected: mean,
				Actual:   f,
				Severity: severity,
			})
		}
		_ = i // Use i to avoid unused variable warning
		_ = rowMap[i]
	}

	return anomalies, nil
}

// CalculateKPIs computes key performance indicators
func CalculateKPIs(table string, columns []ColumnInfo, rows []map[string]interface{}) ([]KPIInsight, error) {
	if len(rows) == 0 {
		return []KPIInsight{}, nil
	}

	var kpis []KPIInsight

	for _, col := range columns {
		if !isNumericType(col.Type) {
			continue
		}

		// Extract values
		values := make([]float64, 0, len(rows))
		for _, row := range rows {
			if val, ok := row[col.Name]; ok && val != nil {
				if f, err := toFloat64(val); err == nil {
					values = append(values, f)
				}
			}
		}

		if len(values) == 0 {
			continue
		}

		// Calculate KPIs based on column name patterns
		kpis = append(kpis, calculateColumnKPIs(col.Name, values)...)
	}

	return kpis, nil
}

// AnalyzeDistributions determines data distribution patterns
func AnalyzeDistributions(column string, rows []map[string]interface{}) (*DistributionInsight, error) {
	if len(rows) == 0 {
		return nil, fmt.Errorf("no data to analyze")
	}

	// Extract numeric values
	values := make([]float64, 0, len(rows))
	for _, row := range rows {
		if val, ok := row[column]; ok && val != nil {
			if f, err := toFloat64(val); err == nil {
				values = append(values, f)
			}
		}
	}

	if len(values) == 0 {
		return nil, fmt.Errorf("no numeric values found")
	}

	// Calculate statistics
	mean := calculateMean(values)
	median := calculateMedian(values)
	stdDev := calculateStdDev(values, mean)

	// Create buckets (using 10 buckets)
	min := minFloat64(values)
	max := maxFloat64(values)
	bucketSize := (max - min) / 10
	if bucketSize == 0 {
		bucketSize = 1
	}

	buckets := make([]DistributionBucket, 10)
	for i := range buckets {
		buckets[i].Min = min + float64(i)*bucketSize
		buckets[i].Max = min + float64(i+1)*bucketSize
	}

	for _, v := range values {
		bucketIdx := int((v - min) / bucketSize)
		if bucketIdx >= 10 {
			bucketIdx = 9
		}
		if bucketIdx < 0 {
			bucketIdx = 0
		}
		buckets[bucketIdx].Count++
	}

	// Determine distribution type
	distType := classifyDistribution(values, mean, median, stdDev)

	return &DistributionInsight{
		Column:  column,
		Type:    distType,
		Buckets: buckets,
		Stats: DistributionStats{
			Mean:   mean,
			Median: median,
			StdDev: stdDev,
		},
	}, nil
}

// IsTimeSeriesColumn detects if a column contains time-series data
func IsTimeSeriesColumn(column ColumnInfo, sampleValues []interface{}) bool {
	// Check data type
	timeTypes := []string{"timestamp", "datetime", "date", "time"}
	for _, tt := range timeTypes {
		if containsIgnoreCase(column.Type, tt) {
			return true
		}
	}

	// Check column name patterns
	timePatterns := []string{"time", "date", "timestamp", "created", "updated", "at"}
	for _, pattern := range timePatterns {
		if containsIgnoreCase(column.Name, pattern) {
			return true
		}
	}

	// Check sample values
	if len(sampleValues) > 0 {
		timeCount := 0
		for _, val := range sampleValues {
			if _, err := parseTime(val); err == nil {
				timeCount++
			}
		}
		// If more than 50% parse as time, consider it a time column
		if float64(timeCount)/float64(len(sampleValues)) > 0.5 {
			return true
		}
	}

	return false
}

// Helper functions

func isNumericType(dataType string) bool {
	numericTypes := []string{"int", "float", "decimal", "numeric", "double", "real", "bigint", "smallint"}
	for _, nt := range numericTypes {
		if containsIgnoreCase(dataType, nt) {
			return true
		}
	}
	return false
}

func parseTime(v interface{}) (time.Time, error) {
	switch val := v.(type) {
	case time.Time:
		return val, nil
	case string:
		formats := []string{
			time.RFC3339,
			"2006-01-02",
			"2006-01-02 15:04:05",
			"2006-01-02T15:04:05",
		}
		for _, format := range formats {
			if t, err := time.Parse(format, val); err == nil {
				return t, nil
			}
		}
		return time.Time{}, fmt.Errorf("cannot parse time: %s", val)
	default:
		return time.Time{}, fmt.Errorf("cannot convert %T to time", v)
	}
}

func calculateMean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func calculateMedian(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	n := len(sorted)
	if n%2 == 0 {
		return (sorted[n/2-1] + sorted[n/2]) / 2
	}
	return sorted[n/2]
}

func calculateStdDev(values []float64, mean float64) float64 {
	if len(values) < 2 {
		return 0
	}
	sumSquaredDiff := 0.0
	for _, v := range values {
		diff := v - mean
		sumSquaredDiff += diff * diff
	}
	variance := sumSquaredDiff / float64(len(values)-1) // Sample standard deviation
	return math.Sqrt(variance)
}

func minFloat64(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	min := values[0]
	for _, v := range values[1:] {
		if v < min {
			min = v
		}
	}
	return min
}

func maxFloat64(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	max := values[0]
	for _, v := range values[1:] {
		if v > max {
			max = v
		}
	}
	return max
}

func containsIgnoreCase(s, substr string) bool {
	return len(substr) <= len(s) &&
		(s == substr ||
			containsSubstringIgnoreCase(s, substr))
}

func containsSubstringIgnoreCase(s, substr string) bool {
	sLower := toLower(s)
	substrLower := toLower(substr)
	return contains(sLower, substrLower)
}

func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c = c + ('a' - 'A')
		}
		result[i] = c
	}
	return string(result)
}

func contains(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func classifyDistribution(values []float64, mean, median, stdDev float64) string {
	if len(values) < 10 {
		return "insufficient_data"
	}

	// Check for uniform distribution
	min := minFloat64(values)
	max := maxFloat64(values)
	range_val := max - min
	if range_val == 0 {
		return "constant"
	}

	// Coefficient of variation
	cv := stdDev / mean
	if mean == 0 {
		cv = stdDev
	}

	// Skewness (simplified calculation)
	skewness := 0.0
	if stdDev > 0 {
		for _, v := range values {
			skewness += math.Pow((v-mean)/stdDev, 3)
		}
		skewness /= float64(len(values))
	}

	// Classify based on characteristics
	if math.Abs(skewness) < 0.5 && cv < 0.3 {
		return "normal"
	}
	if cv > 0.5 && math.Abs(skewness) > 1.0 {
		return "skewed"
	}
	if cv < 0.2 && math.Abs(skewness) < 0.5 {
		return "uniform"
	}

	return "mixed"
}

func calculateColumnKPIs(columnName string, values []float64) []KPIInsight {
	var kpis []KPIInsight

	// Calculate basic KPIs
	sum := 0.0
	min := values[0]
	max := values[0]
	for _, v := range values {
		sum += v
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	avg := sum / float64(len(values))

	// Add KPIs based on column name patterns
	switch {
	case containsIgnoreCase(columnName, "revenue") || containsIgnoreCase(columnName, "sales"):
		kpis = append(kpis, KPIInsight{
			Name:  "total_" + columnName,
			Value: sum,
			Unit:  "currency",
		})
		kpis = append(kpis, KPIInsight{
			Name:  "avg_" + columnName,
			Value: avg,
			Unit:  "currency",
		})
	case containsIgnoreCase(columnName, "quantity") || containsIgnoreCase(columnName, "count"):
		kpis = append(kpis, KPIInsight{
			Name:  "total_" + columnName,
			Value: sum,
			Unit:  "count",
		})
		kpis = append(kpis, KPIInsight{
			Name:  "avg_" + columnName,
			Value: avg,
			Unit:  "count",
		})
	default:
		kpis = append(kpis, KPIInsight{
			Name:  "total_" + columnName,
			Value: sum,
		})
		kpis = append(kpis, KPIInsight{
			Name:  "avg_" + columnName,
			Value: avg,
		})
		kpis = append(kpis, KPIInsight{
			Name:  "min_" + columnName,
			Value: min,
		})
		kpis = append(kpis, KPIInsight{
			Name:  "max_" + columnName,
			Value: max,
		})
	}

	return kpis
}
