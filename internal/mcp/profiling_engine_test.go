package mcp

import (
	"math"
	"testing"
)

func TestDetectPatterns(t *testing.T) {
	tests := []struct {
		name          string
		values        []interface{}
		expectPattern string
		expectCover   float64
	}{
		{
			name: "email_pattern",
			values: []interface{}{
				"user1@example.com",
				"user2@example.com",
				"invalid",
			},
			expectPattern: "email",
			expectCover:   0.67,
		},
		{
			name: "uuid_pattern",
			values: []interface{}{
				"550e8400-e29b-41d4-a716-446655440000",
				"6ba7b810-9dad-11d1-80b4-00c04fd430c8",
			},
			expectPattern: "uuid",
			expectCover:   1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := DetectPatterns(tt.values)
			if len(matches) == 0 {
				t.Fatalf("expected at least one pattern match")
			}

			found := false
			for _, match := range matches {
				if match.Pattern != tt.expectPattern {
					continue
				}
				found = true
				if math.Abs(match.Coverage-tt.expectCover) > 0.01 {
					t.Fatalf("unexpected coverage for %s: got %.2f, want %.2f", tt.expectPattern, match.Coverage, tt.expectCover)
				}
			}

			if !found {
				t.Fatalf("expected pattern %s not found in %+v", tt.expectPattern, matches)
			}
		})
	}
}

func TestCalculateStatistics(t *testing.T) {
	values := []interface{}{1, 2, 3, nil}
	stats, err := CalculateStatistics(values)
	if err != nil {
		t.Fatalf("CalculateStatistics returned error: %v", err)
	}
	if stats.Count != 4 {
		t.Fatalf("unexpected count: got %d, want 4", stats.Count)
	}
	if stats.NullCount != 1 {
		t.Fatalf("unexpected null_count: got %d, want 1", stats.NullCount)
	}
	if stats.UniqueCount != 3 {
		t.Fatalf("unexpected unique_count: got %d, want 3", stats.UniqueCount)
	}
	if stats.Mean != 2 {
		t.Fatalf("unexpected mean: got %.2f, want 2.00", stats.Mean)
	}
	if stats.Median != 2 {
		t.Fatalf("unexpected median: got %.2f, want 2.00", stats.Median)
	}
}

func TestAssessDataQualityAndScore(t *testing.T) {
	column := SchemaColumnInfo{
		ColumnName: "email",
		DataType:   "text",
	}
	values := []interface{}{
		"user1@example.com",
		"user2@example.com",
		"bad",
		nil,
	}

	metrics := AssessDataQuality(column, values)
	if metrics.Completeness != 75 {
		t.Fatalf("unexpected completeness: got %.2f, want 75.00", metrics.Completeness)
	}
	if metrics.Uniqueness != 100 {
		t.Fatalf("unexpected uniqueness: got %.2f, want 100.00", metrics.Uniqueness)
	}
	if math.Abs(metrics.Validity-66.67) > 0.01 {
		t.Fatalf("unexpected validity: got %.2f, want 66.67", metrics.Validity)
	}

	score := CalculateQualityScore(metrics)
	if score <= 0 || score > 100 {
		t.Fatalf("quality score out of range: %.2f", score)
	}
}

func TestProfileColumn(t *testing.T) {
	column := SchemaColumnInfo{
		ColumnName: "website",
		DataType:   "text",
	}
	sampleRows := []map[string]interface{}{
		{"website": "https://example.com"},
		{"website": "https://openai.com"},
		{"website": nil},
	}

	profile, err := ProfileColumn(column, sampleRows)
	if err != nil {
		t.Fatalf("ProfileColumn returned error: %v", err)
	}
	if profile == nil {
		t.Fatalf("ProfileColumn returned nil profile")
	}
	if profile.ColumnName != "website" {
		t.Fatalf("unexpected column name: %s", profile.ColumnName)
	}
	if profile.Statistics.Count != 3 {
		t.Fatalf("unexpected statistics count: %d", profile.Statistics.Count)
	}
	if len(profile.Patterns) == 0 {
		t.Fatalf("expected detected pattern for URL values")
	}
}

func TestProfileHelpers(t *testing.T) {
	t.Run("profile_to_float64", func(t *testing.T) {
		tests := []struct {
			name   string
			value  interface{}
			ok     bool
			expect float64
		}{
			{name: "int", value: int(5), ok: true, expect: 5},
			{name: "int32", value: int32(6), ok: true, expect: 6},
			{name: "int64", value: int64(7), ok: true, expect: 7},
			{name: "float32", value: float32(1.5), ok: true, expect: 1.5},
			{name: "float64", value: float64(2.5), ok: true, expect: 2.5},
			{name: "unsupported", value: "x", ok: false, expect: 0},
		}

		for _, tt := range tests {
			got, ok := profileToFloat64(tt.value)
			if ok != tt.ok {
				t.Fatalf("%s: unexpected ok value: got %v, want %v", tt.name, ok, tt.ok)
			}
			if math.Abs(got-tt.expect) > 0.001 {
				t.Fatalf("%s: unexpected float conversion: got %f, want %f", tt.name, got, tt.expect)
			}
		}
	})

	t.Run("percentage", func(t *testing.T) {
		if got := percentage(1, 3); math.Abs(got-33.33) > 0.01 {
			t.Fatalf("unexpected percentage result: got %.2f, want 33.33", got)
		}
		if got := percentage(0, 0); got != 0 {
			t.Fatalf("expected zero for zero denominator, got %.2f", got)
		}
	})

	t.Run("string_format_bucket", func(t *testing.T) {
		cases := map[string]string{
			"":              "empty",
			"ABC":           "uppercase",
			"abc":           "lowercase",
			"with space":    "contains_space",
			"MixedValue123": "mixed",
		}
		for input, expected := range cases {
			if got := stringFormatBucket(input); got != expected {
				t.Fatalf("unexpected bucket for %q: got %q, want %q", input, got, expected)
			}
		}
	})
}
