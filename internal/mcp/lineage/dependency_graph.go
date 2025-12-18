package lineage

// Node represents a table or view in the lineage graph.
type Node struct {
	Name string `json:"name"`
}

// Edge represents a dependency (foreign key) from one node to another.
type Edge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// Graph holds adjacency lists for lineage traversal.
type Graph struct {
	Nodes map[string]Node
	Out   map[string][]string // from -> to
	In    map[string][]string // to -> from
}

// BuildGraph builds a graph from edges.
func BuildGraph(edges []Edge) Graph {
	g := Graph{
		Nodes: map[string]Node{},
		Out:   map[string][]string{},
		In:    map[string][]string{},
	}
	for _, e := range edges {
		if e.From == "" || e.To == "" {
			continue
		}
		if _, ok := g.Nodes[e.From]; !ok {
			g.Nodes[e.From] = Node{Name: e.From}
		}
		if _, ok := g.Nodes[e.To]; !ok {
			g.Nodes[e.To] = Node{Name: e.To}
		}
		g.Out[e.From] = append(g.Out[e.From], e.To)
		g.In[e.To] = append(g.In[e.To], e.From)
	}
	return g
}

// Upstream returns all tables that the start depends on (follow From -> To).
func (g Graph) Upstream(start string) []string {
	return bfs(start, g.Out)
}

// Downstream returns all tables that depend on start (follow reverse edges).
func (g Graph) Downstream(start string) []string {
	return bfs(start, g.In)
}

func bfs(start string, adj map[string][]string) []string {
	seen := map[string]bool{}
	var q []string
	q = append(q, start)
	for len(q) > 0 {
		n := q[0]
		q = q[1:]
		for _, nxt := range adj[n] {
			if !seen[nxt] {
				seen[nxt] = true
				q = append(q, nxt)
			}
		}
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	return out
}
