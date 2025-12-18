package lineage

// LineageResult captures upstream/downstream dependencies and traversal paths.
type LineageResult struct {
	Upstream   []string            `json:"upstream,omitempty"`
	Downstream []string            `json:"downstream,omitempty"`
	Edges      []Edge              `json:"edges,omitempty"`
	Summary    string              `json:"summary"`
	Scope      string              `json:"scope"`
	Target     string              `json:"target"`
	Graph      map[string][]string `json:"graph,omitempty"`
}

// Analyze computes lineage for the given target table and edges.
func Analyze(target string, edges []Edge, scope string) LineageResult {
	if scope == "" {
		scope = "both"
	}
	g := BuildGraph(edges)
	res := LineageResult{Edges: edges, Scope: scope, Target: target}
	switch scope {
	case "upstream":
		res.Upstream = g.Upstream(target)
		res.Summary = summarize(target, res.Upstream, nil)
	case "downstream":
		res.Downstream = g.Downstream(target)
		res.Summary = summarize(target, nil, res.Downstream)
	default:
		res.Upstream = g.Upstream(target)
		res.Downstream = g.Downstream(target)
		res.Summary = summarize(target, res.Upstream, res.Downstream)
	}
	return res
}

func summarize(target string, up, down []string) string {
	if len(up) == 0 && len(down) == 0 {
		return "No lineage dependencies found."
	}
	return "Lineage computed for " + target
}
