package mcp

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

const (
	maxCompactDescriptionLength = 160
	maxCompactToolsPayloadBytes = 12 * 1024
)

func TestSchemaGate001_AllToolsDefineInputSchema(t *testing.T) {
	toolBlocks, missingInputSchema, err := scanToolStructsFromSource()
	if err != nil {
		t.Fatalf("failed to scan tool structs: %v", err)
	}
	if toolBlocks == 0 {
		t.Fatal("expected tool blocks in registration")
	}
	if missingInputSchema != 0 {
		t.Fatalf("expected all %d tool blocks to define InputSchema, missing=%d", toolBlocks, missingInputSchema)
	}
}

func TestSchemaGate002_MaxCompactDescriptionLength(t *testing.T) {
	for _, tool := range compactModeRegistry(t) {
		if len(tool.Description) > maxCompactDescriptionLength {
			t.Fatalf("tool %q description too long: %d > %d", tool.Name, len(tool.Description), maxCompactDescriptionLength)
		}
	}
}

func TestSchemaGate003_NoMultilineEscapesInCompactDescriptions(t *testing.T) {
	for _, tool := range compactModeRegistry(t) {
		if strings.Contains(tool.Description, "\n") || strings.Contains(tool.Description, "\t") {
			t.Fatalf("tool %q has multiline escape in compact description", tool.Name)
		}
	}
}

func TestSchemaGate004_NoEmbeddedJSONExamplesInCompactDescriptions(t *testing.T) {
	jsonLike := regexp.MustCompile(`\{\s*"[A-Za-z0-9_\-]+"\s*:`)
	for _, tool := range compactModeRegistry(t) {
		if jsonLike.MatchString(tool.Description) {
			t.Fatalf("tool %q contains embedded JSON-like example in compact description: %q", tool.Name, tool.Description)
		}
	}
}

func TestSchemaGate005_MaxCompactToolsListPayloadBytes(t *testing.T) {
	payload, err := json.Marshal(ListToolsResult{Tools: compactModeRegistry(t)})
	if err != nil {
		t.Fatalf("failed to marshal compact tools payload: %v", err)
	}
	if len(payload) > maxCompactToolsPayloadBytes {
		t.Fatalf("compact tools payload too large: %d > %d", len(payload), maxCompactToolsPayloadBytes)
	}
}

func compactModeRegistry(t *testing.T) []ToolInfo {
	t.Helper()
	server := NewMCPServerWithConfig("nonexistent_config.yaml")
	if len(server.toolsRegistry) == 0 {
		t.Fatal("expected tools to be registered")
	}
	return server.toolsRegistry
}
