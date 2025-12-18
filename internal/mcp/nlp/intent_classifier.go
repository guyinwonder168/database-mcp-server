package nlp

import "strings"

// IntentResult holds a simple intent classification outcome.
type IntentResult struct {
	Intent     string  `json:"intent"`
	Confidence float64 `json:"confidence"`
	Reason     string  `json:"reason"`
}

// ClassifyIntent classifies a natural-language request into coarse categories.
func ClassifyIntent(text string) IntentResult {
	t := strings.ToLower(text)
	switch {
	case containsAny(t, []string{"explain plan", "cost", "optimize", "slow query"}):
		return IntentResult{"optimize", 0.82, "keywords: optimize/plan/cost"}
	case containsAny(t, []string{"validate", "syntax", "is this query", "safe to run"}):
		return IntentResult{"validate", 0.8, "keywords: validate/syntax/safe"}
	case containsAny(t, []string{"join", "relationship", "foreign key"}):
		return IntentResult{"discover_joins", 0.75, "keywords: join/relationship"}
	case containsAny(t, []string{"sample", "example rows", "preview"}):
		return IntentResult{"sample_data", 0.7, "keywords: sample/preview"}
	case containsAny(t, []string{"list tables", "show tables", "schema", "describe"}):
		return IntentResult{"schema", 0.7, "keywords: schema/list/describe"}
	default:
		return IntentResult{"generate_sql", 0.6, "fallback intent"}
	}
}

func containsAny(text string, keys []string) bool {
	for _, k := range keys {
		if strings.Contains(text, k) {
			return true
		}
	}
	return false
}
