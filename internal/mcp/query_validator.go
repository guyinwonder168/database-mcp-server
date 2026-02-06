package mcp

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/blastrain/vitess-sqlparser/sqlparser"
)

// ValidationIssue represents a single validation or security finding.
type ValidationIssue struct {
	Rule       string `json:"rule"`
	Severity   string `json:"severity"`   // info | warn | error
	Message    string `json:"message"`    // human-readable description
	Suggestion string `json:"suggestion"` // recommended fix
}

// ValidationResult holds the aggregated validation output.
type ValidationResult struct {
	IsValid  bool              `json:"is_valid"`
	Issues   []ValidationIssue `json:"issues,omitempty"`
	Summary  string            `json:"summary"`
	SQL      string            `json:"sql"`
	Profile  string            `json:"profile_name,omitempty"`
	Database string            `json:"database_name,omitempty"`
}

// validateSQL runs syntax, logic, and security checks on the provided SQL.
func validateSQL(sqlText string) ValidationResult {
	issues := []ValidationIssue{}

	stmt, err := sqlparser.Parse(sqlText)
	if err != nil {
		issues = append(issues, ValidationIssue{
			Rule:       "syntax_error",
			Severity:   "error",
			Message:    fmt.Sprintf("SQL syntax error: %v", err),
			Suggestion: "Review SQL syntax; ensure keywords and clauses are spelled correctly.",
		})
		return ValidationResult{
			IsValid: false,
			Issues:  issues,
			Summary: "Validation failed: syntax error.",
			SQL:     sqlText,
		}
	}

	issues = append(issues, analyzeLogic(stmt)...)
	issues = append(issues, scanSecurity(sqlText)...)

	isValid := true
	for _, i := range issues {
		if i.Severity == "error" {
			isValid = false
			break
		}
	}
	summary := "Validation passed."
	if !isValid {
		summary = "Validation failed."
	} else if len(issues) > 0 {
		summary = "Validation passed with warnings."
	}
	return ValidationResult{
		IsValid: isValid,
		Issues:  issues,
		Summary: summary,
		SQL:     sqlText,
	}
}

// analyzeLogic performs lightweight semantic checks based on the AST.
func analyzeLogic(stmt sqlparser.Statement) []ValidationIssue {
	switch node := stmt.(type) {
	case *sqlparser.Select:
		return analyzeSelectLogic(node)
	case *sqlparser.Update:
		return analyzeUpdateLogic(node)
	case *sqlparser.Delete:
		return analyzeDeleteLogic(node)
	}
	return nil
}

func analyzeSelectLogic(selectStmt *sqlparser.Select) []ValidationIssue {
	issues := make([]ValidationIssue, 0)
	if len(selectStmt.From) == 0 {
		issues = append(issues, ValidationIssue{
			Rule:       "missing_from",
			Severity:   "warn",
			Message:    "SELECT has no FROM clause; may return database-dependent constant result.",
			Suggestion: "Add a FROM clause when querying tables or views.",
		})
	}
	for _, expr := range selectStmt.From {
		joinTable, ok := expr.(*sqlparser.JoinTableExpr)
		if !ok || hasExplicitJoinCondition(joinTable) {
			continue
		}
		issues = append(issues, ValidationIssue{
			Rule:       "join_without_condition",
			Severity:   "warn",
			Message:    "JOIN without ON/USING condition detected.",
			Suggestion: "Specify join conditions to avoid Cartesian products.",
		})
	}
	return issues
}

func hasExplicitJoinCondition(joinTable *sqlparser.JoinTableExpr) bool {
	if joinTable.On != nil {
		return true
	}
	return strings.Contains(strings.ToLower(joinTable.Join), "natural")
}

func analyzeUpdateLogic(updateStmt *sqlparser.Update) []ValidationIssue {
	if updateStmt.Where != nil {
		return nil
	}
	return []ValidationIssue{
		{
			Rule:       "update_without_where",
			Severity:   "warn",
			Message:    "UPDATE without WHERE may affect all rows.",
			Suggestion: "Add a WHERE clause to target specific rows.",
		},
	}
}

func analyzeDeleteLogic(deleteStmt *sqlparser.Delete) []ValidationIssue {
	if deleteStmt.Where != nil {
		return nil
	}
	return []ValidationIssue{
		{
			Rule:       "delete_without_where",
			Severity:   "warn",
			Message:    "DELETE without WHERE may remove all rows.",
			Suggestion: "Add a WHERE clause to target specific rows.",
		},
	}
}

// scanSecurity flags common SQL injection patterns in raw SQL text.
func scanSecurity(sqlText string) []ValidationIssue {
	text := strings.ToLower(sqlText)
	var issues []ValidationIssue

	patterns := []struct {
		re         *regexp.Regexp
		rule       string
		message    string
		suggestion string
	}{
		{regexp.MustCompile(`\bor\b\s+1\s*=\s*1`), "tautology", "Potential tautology detected (OR 1=1).", "Use parameterized queries; avoid concatenating untrusted input."},
		{regexp.MustCompile(`union\s+select`), "union_injection", "UNION SELECT found; ensure this is intentional and parameterized.", "Validate allowed statements or remove UNION from user-supplied input."},
		{regexp.MustCompile(`;--`), "statement_termination", "Statement termination (;) followed by comment (--) detected.", "Remove chained statements; use a single parameterized statement."},
		{regexp.MustCompile(`/\*`), "comment_block", "Block comment start detected; could be used to bypass filters.", "Strip or reject SQL containing block comments from untrusted sources."},
	}

	for _, p := range patterns {
		if p.re.FindStringIndex(text) != nil {
			issues = append(issues, ValidationIssue{
				Rule:       p.rule,
				Severity:   "warn",
				Message:    p.message,
				Suggestion: p.suggestion,
			})
		}
	}

	return issues
}
