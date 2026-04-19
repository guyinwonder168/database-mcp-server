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

type dataPoint struct {
	time  time.Time
	value float64
}

// DetectTrends identifies trends in time-series data
func DetectTrends(timeColumn string, valueColumn string, rows []map[string]interface{}) ([]TrendInsight, error) {
	if len(rows) < 2 {
		return []TrendInsight{}, nil
	}

	points, minTime, maxTime := extractDataPoints(timeColumn, valueColumn, rows)
	if len(points) < 2 {
		return []TrendInsight{}, nil
	}

	sort.Slice(points, func(i, j int) bool {
		return points[i].time.Before(points[j].time)
	})

	slope, confidence := calculateLinearRegression(points)
	direction := determineTrendDirection(slope, confidence, points)

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

	values := extractAnomalyValues(column, rows)
	if len(values) == 0 {
		return []AnomalyInsight{}, nil
	}

	mean := calculateMean(values)
	stdDev := calculateStdDev(values, mean)

	if stdDev == 0 {
		return []AnomalyInsight{}, nil
	}

	return findAnomalies(column, rows, mean, stdDev, threshold)
}

// CalculateKPIs computes key performance indicators
func CalculateKPIs(table string, columns []ColumnInfo, rows []map[string]interface{}) ([]KPIInsight, error) {
	if len(rows) == 0 {
		return []KPIInsight{}, nil
	}

	var kpis []KPIInsight
	for _, col := range columns {
		if isNumericType(col.Type) {
			values := extractColumnValues(col.Name, rows)
			if len(values) > 0 {
				kpis = append(kpis, calculateColumnKPIs(col.Name, values)...)
			}
		}
	}

	return kpis, nil
}

// AnalyzeDistributions determines data distribution patterns
func AnalyzeDistributions(column string, rows []map[string]interface{}) (*DistributionInsight, error) {
	if len(rows) == 0 {
		return nil, fmt.Errorf("no data to analyze")
	}

	values := extractColumnValues(column, rows)
	if len(values) == 0 {
		return nil, fmt.Errorf("no numeric values found")
	}

	mean := calculateMean(values)
	median := calculateMedian(values)
	stdDev := calculateStdDev(values, mean)

	buckets := createDistributionBuckets(values)

	distType := classifyDistribution(values, mean, stdDev)

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

func classifyDistribution(values []float64, mean, stdDev float64) string {
	if len(values) < 10 {
		return "insufficient_data"
	}

	// Check for uniform distribution
	min := minFloat64(values)
	max := maxFloat64(values)
	valueRange := max - min
	if valueRange == 0 {
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
			z := (v - mean) / stdDev
			skewness += z * z * z
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

// Helper functions for DetectTrends

func extractDataPoints(timeColumn, valueColumn string, rows []map[string]interface{}) ([]dataPoint, time.Time, time.Time) {
	points := make([]dataPoint, 0, len(rows))
	var minTime, maxTime time.Time

	for _, row := range rows {
		point, ok := dataPointFromRow(timeColumn, valueColumn, row)
		if !ok {
			continue
		}
		points = append(points, point)
		minTime, maxTime = updateTimeBounds(minTime, maxTime, point.time)
	}

	return points, minTime, maxTime
}

func dataPointFromRow(timeColumn, valueColumn string, row map[string]interface{}) (dataPoint, bool) {
	timeVal, ok := row[timeColumn]
	if !ok || timeVal == nil {
		return dataPoint{}, false
	}
	valueVal, ok := row[valueColumn]
	if !ok || valueVal == nil {
		return dataPoint{}, false
	}
	parsedTime, err := parseTime(timeVal)
	if err != nil {
		return dataPoint{}, false
	}
	parsedValue, err := toFloat64(valueVal)
	if err != nil {
		return dataPoint{}, false
	}
	return dataPoint{time: parsedTime, value: parsedValue}, true
}

func updateTimeBounds(minTime, maxTime, current time.Time) (time.Time, time.Time) {
	if minTime.IsZero() || current.Before(minTime) {
		minTime = current
	}
	if maxTime.IsZero() || current.After(maxTime) {
		maxTime = current
	}
	return minTime, maxTime
}

func calculateLinearRegression(points []dataPoint) (slope float64, confidence float64) {
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

	denominator := n*sumX2 - sumX*sumX
	if denominator == 0 {
		return 0, 0
	}

	slope = (n*sumXY - sumX*sumY) / denominator
	confidence = calculateRSquared(points, slope, sumX, sumY, n)

	return slope, confidence
}

func calculateRSquared(points []dataPoint, slope, sumX, sumY, n float64) float64 {
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
	return math.Max(0, math.Min(1, rSquared))
}

func determineTrendDirection(slope, confidence float64, points []dataPoint) string {
	direction := "stable"

	if confidence < 0.3 {
		return direction
	}

	threshold := 0.001 * (points[len(points)-1].value - points[0].value)
	if math.Abs(threshold) < 0.0001 {
		threshold = 0.001
	}

	if slope > threshold {
		direction = "upward"
	} else if slope < -threshold {
		direction = "downward"
	}

	return direction
}

// Helper functions for DetectAnomalies

func extractAnomalyValues(column string, rows []map[string]interface{}) []float64 {
	values := make([]float64, 0, len(rows))
	for _, row := range rows {
		if val, ok := row[column]; ok && val != nil {
			if f, err := toFloat64(val); err == nil {
				values = append(values, f)
			}
		}
	}
	return values
}

func findAnomalies(column string, rows []map[string]interface{}, mean, stdDev, threshold float64) ([]AnomalyInsight, error) {
	var anomalies []AnomalyInsight
	for _, row := range rows {
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
			severity := determineAnomalySeverity(zScore)
			anomalies = append(anomalies, AnomalyInsight{
				Column:   column,
				Expected: mean,
				Actual:   f,
				Severity: severity,
			})
		}
	}
	return anomalies, nil
}

func determineAnomalySeverity(zScore float64) string {
	if zScore > 4.0 {
		return "critical"
	}
	if zScore > 3.0 {
		return "high"
	}
	if zScore > 2.5 {
		return "medium"
	}
	return "low"
}

// Helper functions for CalculateKPIs

func extractColumnValues(columnName string, rows []map[string]interface{}) []float64 {
	values := make([]float64, 0, len(rows))
	for _, row := range rows {
		if val, ok := row[columnName]; ok && val != nil {
			if f, err := toFloat64(val); err == nil {
				values = append(values, f)
			}
		}
	}
	return values
}

// Helper functions for AnalyzeDistributions

func createDistributionBuckets(values []float64) []DistributionBucket {
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
		bucketIndex := int((v - min) / bucketSize)
		if bucketIndex >= 10 {
			bucketIndex = 9
		}
		if bucketIndex < 0 {
			bucketIndex = 0
		}
		// #nosec G602 -- bucketIndex is bounded to [0,9] by the checks above; buckets slice has length 10
		buckets[bucketIndex].Count++
	}

	return buckets
}
