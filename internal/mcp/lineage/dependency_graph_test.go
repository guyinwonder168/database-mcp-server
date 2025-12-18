package lineage

import "testing"

func TestBuildGraphAndTraversal(t *testing.T) {
	edges := []Edge{
		{From: "orders", To: "customers"},
		{From: "order_items", To: "orders"},
		{From: "payments", To: "orders"},
	}
	g := BuildGraph(edges)
	up := g.Upstream("order_items")
	if !containsAll(up, []string{"orders", "customers"}) {
		t.Fatalf("expected upstream to include orders and customers, got %v", up)
	}
	down := g.Downstream("customers")
	if !containsAll(down, []string{"orders", "order_items", "payments"}) {
		t.Fatalf("expected downstream chain from customers, got %v", down)
	}
}

func TestAnalyzeScope(t *testing.T) {
	edges := []Edge{{From: "orders", To: "customers"}}
	res := Analyze("orders", edges, "upstream")
	if len(res.Upstream) == 0 || len(res.Downstream) != 0 {
		t.Fatalf("unexpected upstream/downstream: %+v", res)
	}
}

func containsAll(set []string, want []string) bool {
	m := map[string]bool{}
	for _, s := range set {
		m[s] = true
	}
	for _, w := range want {
		if !m[w] {
			return false
		}
	}
	return true
}

func TestBuildGraphDedup(t *testing.T) {
	edges := []Edge{{From: "a", To: "b"}, {From: "a", To: "b"}}
	g := BuildGraph(edges)
	if len(g.Out["a"]) != 2 { // we keep duplicates; behavior documented
		t.Fatalf("expected duplicates preserved, got %v", g.Out["a"])
	}
}
