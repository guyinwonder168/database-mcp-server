package nlp

import (
	"regexp"
	"strings"
)

// EntityExtractionResult holds extracted tokens.
type EntityExtractionResult struct {
	Tables  []string `json:"tables,omitempty"`
	Columns []string `json:"columns,omitempty"`
}

var (
	tableHintRe  = regexp.MustCompile(`(?i)(from|join)\\s+([a-zA-Z0-9_\\.]+)`)
	columnHintRe = regexp.MustCompile(`(?i)select\\s+(.+?)\\s+from`)
	wordRe       = regexp.MustCompile(`[a-zA-Z0-9_]+`)
)

// ExtractEntities pulls simple table and column hints from natural text or SQL.
func ExtractEntities(text string) EntityExtractionResult {
	t := strings.ToLower(text)
	res := EntityExtractionResult{}

	for _, m := range tableHintRe.FindAllStringSubmatch(t, -1) {
		if len(m) >= 3 {
			res.Tables = appendIfMissing(res.Tables, m[2])
		}
	}

	if m := columnHintRe.FindStringSubmatch(t); len(m) >= 2 {
		cols := strings.Split(m[1], ",")
		for _, c := range cols {
			token := strings.TrimSpace(c)
			if token != "" {
				res.Columns = appendIfMissing(res.Columns, cleanToken(token))
			}
		}
	}

	// Fallback: use keywords like "table <name>" or raw tokens if none found
	if len(res.Tables) == 0 {
		for _, w := range wordRe.FindAllString(t, -1) {
			if w == "table" || w == "tables" {
				continue
			}
			if strings.HasSuffix(w, "s") && len(w) > 4 {
				res.Tables = appendIfMissing(res.Tables, w)
			}
		}
	}
	return res
}

func appendIfMissing(slice []string, val string) []string {
	for _, v := range slice {
		if v == val {
			return slice
		}
	}
	return append(slice, val)
}

func cleanToken(token string) string {
	token = strings.Trim(token, "`\" ")
	return token
}
