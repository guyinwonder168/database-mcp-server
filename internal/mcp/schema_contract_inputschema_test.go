package mcp

import "testing"

func TestAllRegisteredToolsHaveInputSchema(t *testing.T) {
	toolBlocks, missingInputSchema, err := scanToolStructsFromSource()
	if err != nil {
		t.Fatalf("failed to scan tool structs: %v", err)
	}

	if toolBlocks == 0 {
		t.Fatal("expected at least one registered tool block")
	}

	if missingInputSchema != 0 {
		t.Fatalf("expected all %d tool blocks to define InputSchema, missing=%d", toolBlocks, missingInputSchema)
	}
}
