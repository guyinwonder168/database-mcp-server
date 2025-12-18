package lineage

import "testing"

func TestAnalyzeBoth(t *testing.T) {
	edges := []Edge{
		{From: "orders", To: "customers"},
		{From: "shipments", To: "orders"},
	}
	res := Analyze("orders", edges, "both")
	if len(res.Upstream) != 1 || res.Upstream[0] != "customers" {
		t.Fatalf("unexpected upstream: %v", res.Upstream)
	}
	if len(res.Downstream) != 1 || res.Downstream[0] != "shipments" {
		t.Fatalf("unexpected downstream: %v", res.Downstream)
	}
	if res.Summary == "" {
		t.Fatalf("expected summary")
	}
}
