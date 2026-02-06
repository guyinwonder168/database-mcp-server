package mcp

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
)

const minimumPatternCoverage = 0.5

// ProfileColumn computes advanced profiling metrics for one column using sampled rows.
func ProfileColumn(column SchemaColumnInfo, sampleRows []map[string]interface{}) (*ColumnProfile, error) {
	values := collectColumnValues(column.ColumnName, sampleRows)
	stats, err := CalculateStatistics(values)
	if err != nil {
		return nil, err
	}
	patterns := DetectPatterns(values)
	quality := AssessDataQuality(column, values)
	return &ColumnProfile{
		ColumnName:     column.ColumnName,
		DataType:       column.DataType,
		Statistics:     *stats,
		Patterns:       patterns,
		QualityScore:   CalculateQualityScore(quality),
		QualityMetrics: quality,
	}, nil
}

// CalculateStatistics computes count/null/unique and numeric descriptive statistics.
func CalculateStatistics(values []interface{}) (*ColumnStatistics, error) {
	stats := &ColumnStatistics{
		Count: int64(len(values)),
	}
	if len(values) == 0 {
		return stats, nil
	}

	uniqueValues := make(map[string]struct{})
	numericValues := make([]float64, 0, len(values))
	stringValues := make([]string, 0, len(values))

	for _, value := range values {
		if value == nil {
			stats.NullCount++
			continue
		}

		strVal := fmt.Sprintf("%v", value)
		uniqueValues[strVal] = struct{}{}
		stringValues = append(stringValues, strVal)

		if f, ok := profileToFloat64(value); ok {
			numericValues = append(numericValues, f)
		}
	}

	stats.UniqueCount = int64(len(uniqueValues))
	if len(numericValues) > 0 {
		sort.Float64s(numericValues)
		stats.Min = numericValues[0]
		stats.Max = numericValues[len(numericValues)-1]
		stats.Mean = profileCalculateMean(numericValues)
		stats.Median = profileCalculateMedian(numericValues)
		stats.StdDev = profileCalculateStdDev(numericValues, stats.Mean)
		return stats, nil
	}

	if len(stringValues) > 0 {
		sort.Strings(stringValues)
		stats.Min = stringValues[0]
		stats.Max = stringValues[len(stringValues)-1]
	}
	return stats, nil
}

// DetectPatterns identifies known patterns from sampled values.
func DetectPatterns(values []interface{}) []PatternMatch {
	nonNullValues := nonNullStringValues(values)
	if len(nonNullValues) == 0 {
		return nil
	}

	matches := make([]PatternMatch, 0)
	for _, pattern := range KnownPatterns {
		compiled, err := regexp.Compile(pattern.Regex)
		if err != nil {
			continue
		}

		matchCount := 0
		example := ""
		for _, value := range nonNullValues {
			if compiled.MatchString(value) {
				matchCount++
				if example == "" {
					example = value
				}
			}
		}

		coverage := float64(matchCount) / float64(len(nonNullValues))
		if coverage >= minimumPatternCoverage {
			matches = append(matches, PatternMatch{
				Pattern:  pattern.Name,
				Regex:    pattern.Regex,
				Coverage: coverage,
				Example:  example,
			})
		}
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Coverage > matches[j].Coverage
	})
	return matches
}

// AssessDataQuality computes completeness/uniqueness/validity/consistency percentages.
func AssessDataQuality(column SchemaColumnInfo, values []interface{}) DataQualityMetrics {
	if len(values) == 0 {
		return DataQualityMetrics{}
	}

	nonNull := 0
	uniqueValues := make(map[string]struct{})
	stringSamples := make([]string, 0, len(values))
	for _, value := range values {
		if value == nil {
			continue
		}
		nonNull++
		strVal := fmt.Sprintf("%v", value)
		uniqueValues[strVal] = struct{}{}
		stringSamples = append(stringSamples, strVal)
	}

	completeness := percentage(nonNull, len(values))
	uniqueness := 0.0
	if nonNull > 0 {
		uniqueness = percentage(len(uniqueValues), nonNull)
	}

	validity := 100.0
	patterns := DetectPatterns(values)
	if len(patterns) > 0 {
		validity = patterns[0].Coverage * 100.0
	}

	consistency := assessFormattingConsistency(stringSamples, column.DataType)

	return DataQualityMetrics{
		Completeness: completeness,
		Uniqueness:   uniqueness,
		Validity:     validity,
		Consistency:  consistency,
	}
}

// CalculateQualityScore calculates a single 0-100 quality score from component metrics.
func CalculateQualityScore(metrics DataQualityMetrics) float64 {
	score := (metrics.Completeness + metrics.Uniqueness + metrics.Validity + metrics.Consistency) / 4.0
	return math.Round(score*100) / 100
}

func collectColumnValues(columnName string, sampleRows []map[string]interface{}) []interface{} {
	values := make([]interface{}, 0, len(sampleRows))
	for _, row := range sampleRows {
		if value, ok := row[columnName]; ok {
			values = append(values, value)
			continue
		}
		values = append(values, nil)
	}
	return values
}

func nonNullStringValues(values []interface{}) []string {
	results := make([]string, 0, len(values))
	for _, value := range values {
		if value == nil {
			continue
		}
		results = append(results, fmt.Sprintf("%v", value))
	}
	return results
}

func profileToFloat64(value interface{}) (float64, bool) {
	switch typed := value.(type) {
	case int:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case float32:
		return float64(typed), true
	case float64:
		return typed, true
	default:
		return 0, false
	}
}

func profileCalculateMean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, value := range values {
		sum += value
	}
	return sum / float64(len(values))
}

func profileCalculateMedian(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	mid := len(values) / 2
	if len(values)%2 == 0 {
		return (values[mid-1] + values[mid]) / 2
	}
	return values[mid]
}

func profileCalculateStdDev(values []float64, mean float64) float64 {
	if len(values) == 0 {
		return 0
	}
	varianceSum := 0.0
	for _, value := range values {
		diff := value - mean
		varianceSum += diff * diff
	}
	variance := varianceSum / float64(len(values))
	return math.Sqrt(variance)
}

func percentage(numerator, denominator int) float64 {
	if denominator == 0 {
		return 0
	}
	return math.Round((float64(numerator)/float64(denominator))*10000) / 100
}

func assessFormattingConsistency(values []string, dataType string) float64 {
	if len(values) == 0 {
		return 0
	}

	if isNumericDataType(dataType) {
		return 100
	}

	formatBuckets := make(map[string]int)
	for _, value := range values {
		formatBuckets[stringFormatBucket(value)]++
	}

	maxBucket := 0
	for _, count := range formatBuckets {
		if count > maxBucket {
			maxBucket = count
		}
	}
	return percentage(maxBucket, len(values))
}

func isNumericDataType(dataType string) bool {
	normalized := strings.ToLower(dataType)
	return strings.Contains(normalized, "int") ||
		strings.Contains(normalized, "float") ||
		strings.Contains(normalized, "double") ||
		strings.Contains(normalized, "decimal") ||
		strings.Contains(normalized, "numeric")
}

func stringFormatBucket(value string) string {
	switch {
	case value == "":
		return "empty"
	case strings.Contains(value, " "):
		return "contains_space"
	case value == strings.ToLower(value):
		return "lowercase"
	case value == strings.ToUpper(value):
		return "uppercase"
	default:
		return "mixed"
	}
}
