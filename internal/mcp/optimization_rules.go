package mcp

import "strings"

// OptimizationFinding represents a detected optimization opportunity.
type OptimizationFinding struct {
	Rule       string                 `json:"rule"`
	Severity   string                 `json:"severity"`   // info | warn | critical
	Message    string                 `json:"message"`    // human-readable explanation
	Suggestion string                 `json:"suggestion"` // concrete next action
	Evidence   map[string]interface{} `json:"evidence,omitempty"`
}

// OptimizationRule is a predicate that inspects an execution plan and returns findings.
type OptimizationRule func(plan *ExplainPlan) []OptimizationFinding

// OptimizationRuleEngine runs a set of rules on an ExplainPlan.
type OptimizationRuleEngine struct {
	rules []OptimizationRule
}

// NewDefaultOptimizationRuleEngine builds the rule engine with database-specific heuristics.
func NewDefaultOptimizationRuleEngine() *OptimizationRuleEngine {
	return &OptimizationRuleEngine{
		rules: []OptimizationRule{
			missingIndexRule,
			inefficientJoinRule,
			fullScanWithFilterRule,
		},
	}
}

// Evaluate applies all rules and returns aggregated findings.
func (e *OptimizationRuleEngine) Evaluate(plan *ExplainPlan) []OptimizationFinding {
	if plan == nil {
		return nil
	}
	var findings []OptimizationFinding
	for _, rule := range e.rules {
		findings = append(findings, rule(plan)...)
	}
	return findings
}

// --- Rule Implementations ---

// missingIndexRule flags table scans that also have filter conditions, indicating likely missing indexes.
func missingIndexRule(plan *ExplainPlan) []OptimizationFinding {
	var findings []OptimizationFinding
	for _, step := range plan.Steps {
		if step.Target == "" {
			continue
		}
		op := strings.ToUpper(step.Operation)
		access := strings.ToUpper(step.Detail)
		if stepUsesIndex(step, op, access) {
			continue
		}
		if isScanStep(op, access) && stepHasFilter(step, access) {
			findings = append(findings, missingIndexFinding(step))
		}
	}
	return findings
}

func stepUsesIndex(step PlanStep, op, access string) bool {
	if strings.Contains(op, "INDEX") || strings.Contains(access, "USING INDEX") {
		return true
	}
	if step.Extra == nil {
		return false
	}
	return step.Extra["index_name"] != nil || step.Extra["used_index"] != nil
}

func isScanStep(op, access string) bool {
	return op == "SEQ SCAN" || op == "SCAN" || strings.Contains(access, "ALL")
}

func stepHasFilter(step PlanStep, access string) bool {
	if step.Extra != nil {
		if condition, ok := step.Extra["attached_condition"]; ok && condition != nil {
			return true
		}
		if filter, ok := step.Extra["filter"]; ok && filter != nil {
			return true
		}
	}
	return strings.Contains(access, "USING WHERE")
}

func missingIndexFinding(step PlanStep) OptimizationFinding {
	return OptimizationFinding{
		Rule:       "missing_index",
		Severity:   "warn",
		Message:    "Full table scan with filter detected; an index may be missing.",
		Suggestion: "Add an index on the columns used in the filter for table " + step.Target,
		Evidence: map[string]interface{}{
			"operation": step.Operation,
			"target":    step.Target,
			"detail":    step.Detail,
		},
	}
}

// inefficientJoinRule flags joins where multiple scans are combined without indexed access.
func inefficientJoinRule(plan *ExplainPlan) []OptimizationFinding {
	if len(plan.Steps) < 2 {
		return nil
	}
	var scans int
	var targets []string
	for _, step := range plan.Steps {
		op := strings.ToUpper(step.Operation)
		access := strings.ToUpper(step.Detail)
		if op == "SEQ SCAN" || op == "SCAN" || strings.Contains(access, "ALL") {
			scans++
			if step.Target != "" {
				targets = append(targets, step.Target)
			}
		}
	}
	if scans >= 2 {
		return []OptimizationFinding{
			{
				Rule:       "inefficient_join",
				Severity:   "warn",
				Message:    "Joins rely on full scans; join keys likely need indexes or rewritten join strategy.",
				Suggestion: "Create indexes on join columns and consider rewriting joins to reduce full scans.",
				Evidence: map[string]interface{}{
					"scan_tables": targets,
					"scan_count":  scans,
				},
			},
		}
	}
	return nil
}

// fullScanWithFilterRule captures SQLite SEARCH/SCAN cases where an index could help.
func fullScanWithFilterRule(plan *ExplainPlan) []OptimizationFinding {
	var findings []OptimizationFinding
	for _, step := range plan.Steps {
		if strings.EqualFold(step.Operation, "SEARCH") ||
			(step.Extra != nil && step.Extra["index_name"] != nil) {
			continue // already using an index in SQLite wording
		}
		if strings.EqualFold(step.Operation, "SCAN") && step.Target != "" {
			findings = append(findings, OptimizationFinding{
				Rule:       "sqlite_full_scan",
				Severity:   "info",
				Message:    "SQLite reported a full scan; check for indexes on filter or join columns.",
				Suggestion: "Add an index on relevant columns or adjust the query to enable SEARCH instead of SCAN.",
				Evidence: map[string]interface{}{
					"target": step.Target,
					"detail": step.Detail,
				},
			})
		}
	}
	return findings
}
