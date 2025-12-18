package nlp

import "testing"

func TestClassifyIntentOptimize(t *testing.T) {
	res := ClassifyIntent("Why is this query slow? optimize the plan")
	if res.Intent != "optimize" {
		t.Fatalf("expected optimize intent, got %s", res.Intent)
	}
	if res.Confidence < 0.7 {
		t.Fatalf("expected confidence >=0.7, got %.2f", res.Confidence)
	}
}

func TestClassifyIntentValidate(t *testing.T) {
	res := ClassifyIntent("validate this SQL syntax")
	if res.Intent != "validate" {
		t.Fatalf("expected validate intent, got %s", res.Intent)
	}
}

func TestClassifyIntentFallback(t *testing.T) {
	res := ClassifyIntent("build me a dashboard query")
	if res.Intent == "" {
		t.Fatalf("expected some intent")
	}
}
