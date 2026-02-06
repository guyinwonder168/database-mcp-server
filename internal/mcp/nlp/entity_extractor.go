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
	result := EntityExtractionResult{
		Tables:  extractTableHints(t),
		Columns: extractColumnHints(t),
	}
	if len(result.Tables) == 0 {
		result.Tables = inferPluralTableTokens(t, result.Tables)
	}
	return result
}

func extractTableHints(text string) []string {
	tables := make([]string, 0)
	for _, match := range tableHintRe.FindAllStringSubmatch(text, -1) {
		if len(match) >= 3 {
			tables = appendIfMissing(tables, match[2])
		}
	}
	return tables
}

func extractColumnHints(text string) []string {
	match := columnHintRe.FindStringSubmatch(text)
	if len(match) < 2 {
		return nil
	}
	columns := make([]string, 0)
	for _, column := range strings.Split(match[1], ",") {
		token := strings.TrimSpace(column)
		if token == "" {
			continue
		}
		columns = appendIfMissing(columns, cleanToken(token))
	}
	return columns
}

func inferPluralTableTokens(text string, tables []string) []string {
	for _, token := range wordRe.FindAllString(text, -1) {
		if token == "table" || token == "tables" {
			continue
		}
		if strings.HasSuffix(token, "s") && len(token) > 4 {
			tables = appendIfMissing(tables, token)
		}
	}
	return tables
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
