package mcp

// PerformanceEstimation provides a simple cost/row baseline plus improvement prediction.
type PerformanceEstimation struct {
	BaselineCost float64          `json:"baseline_cost,omitempty"` // sum of estimated costs from the plan
	BaselineRows float64          `json:"baseline_rows,omitempty"` // sum of estimated rows from the plan
	Confidence   float64          `json:"confidence"`              // 0..1 confidence in the estimation
	Improvement  ImprovementRange `json:"improvement"`             // expected relative improvement if suggestions are applied
	Notes        []string         `json:"notes,omitempty"`
	Suggestions  []string         `json:"suggestions,omitempty"` // surfaced from findings
}

// ImprovementRange represents an estimated improvement interval (as percent gain).
type ImprovementRange struct {
	LowerPercent float64 `json:"lower_percent"`
	UpperPercent float64 `json:"upper_percent"`
	Confidence   float64 `json:"confidence"` // confidence in this interval
	Explanation  string  `json:"explanation,omitempty"`
}

// EstimatePerformance produces a lightweight estimation using plan statistics and findings.
// This is heuristic and intentionally conservative; it prefers interval estimates over point guesses.
func EstimatePerformance(plan *ExplainPlan, findings []OptimizationFinding) PerformanceEstimation {
	var baselineCost float64
	var baselineRows float64
	for _, step := range plan.Steps {
		baselineCost += step.EstimatedCost
		baselineRows += step.EstimatedRows
	}

	improvement := ImprovementRange{
		LowerPercent: 0,
		UpperPercent: 10,
		Confidence:   0.6,
		Explanation:  "Minor gains expected; no high-impact findings detected.",
	}

	notes := []string{}
	suggestions := []string{}
	confidence := 0.6

	hasCost := baselineCost > 0
	if hasCost {
		confidence = 0.75
		notes = append(notes, "Cost estimates sourced from EXPLAIN plan.")
	} else {
		notes = append(notes, "Plan did not expose numeric costs; using heuristic defaults.")
	}

	// Adjust intervals based on findings
	for _, f := range findings {
		switch f.Rule {
		case "missing_index":
			improvement.LowerPercent = maxFloat(improvement.LowerPercent, 25)
			improvement.UpperPercent = maxFloat(improvement.UpperPercent, 60)
			improvement.Confidence = maxFloat(improvement.Confidence, 0.7)
			improvement.Explanation = "Indexing filter/join columns typically yields large gains."
		case "inefficient_join":
			improvement.LowerPercent = maxFloat(improvement.LowerPercent, 15)
			improvement.UpperPercent = maxFloat(improvement.UpperPercent, 40)
			improvement.Confidence = maxFloat(improvement.Confidence, 0.7)
			improvement.Explanation = "Optimizing join strategy/indexes reduces full scans."
		case "sqlite_full_scan":
			improvement.LowerPercent = maxFloat(improvement.LowerPercent, 5)
			improvement.UpperPercent = maxFloat(improvement.UpperPercent, 25)
			improvement.Confidence = maxFloat(improvement.Confidence, 0.65)
			improvement.Explanation = "Adding indexes converts SCAN to SEARCH in SQLite."
		}
		suggestions = append(suggestions, f.Suggestion)
	}

	return PerformanceEstimation{
		BaselineCost: baselineCost,
		BaselineRows: baselineRows,
		Confidence:   clamp01(confidence),
		Improvement:  improvement,
		Notes:        notes,
		Suggestions:  suggestions,
	}
}

func maxFloat(a, b float64) float64 {
	if b > a {
		return b
	}
	return a
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
