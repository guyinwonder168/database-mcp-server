package context

import (
	"testing"
	"time"
)

func TestManagerAppendAndHistory(t *testing.T) {
	mgr := NewManager(1*time.Minute, 5)
	mgr.Append("session1", "user", "hello")
	mgr.Append("session1", "assistant", "hi")
	h := mgr.History("session1")
	if len(h) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(h))
	}
	if h[0].Role != "user" || h[1].Role != "assistant" {
		t.Fatalf("unexpected roles in history: %+v", h)
	}
}

func TestManagerTTLPrune(t *testing.T) {
	mgr := NewManager(10*time.Millisecond, 5)
	mgr.Append("session1", "user", "hello")
	time.Sleep(20 * time.Millisecond)
	if len(mgr.History("session1")) != 0 {
		t.Fatalf("expected history to be pruned after TTL")
	}
}
