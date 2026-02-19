package mcp

import (
	"encoding/json"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
)

type toolDescriptionMetric struct {
	Name              string
	DescriptionLength int
}

func TestSchemaContractBaselineMetrics(t *testing.T) {
	server := NewMCPServerWithConfig("nonexistent_config.yaml")
	if len(server.toolsRegistry) == 0 {
		t.Fatal("expected tools to be registered")
	}

	metrics := make([]toolDescriptionMetric, 0, len(server.toolsRegistry))
	for _, tool := range server.toolsRegistry {
		metrics = append(metrics, toolDescriptionMetric{
			Name:              tool.Name,
			DescriptionLength: len(tool.Description),
		})
	}

	sort.Slice(metrics, func(i, j int) bool {
		return metrics[i].Name < metrics[j].Name
	})

	payload, err := json.Marshal(ListToolsResult{Tools: server.toolsRegistry})
	if err != nil {
		t.Fatalf("failed to marshal list-tools payload: %v", err)
	}

	toolBlocks, missingInputSchema, err := scanToolStructsFromSource()
	if err != nil {
		t.Fatalf("failed to scan tool structs: %v", err)
	}

	t.Logf("SCHEMA_BASELINE tool_count=%d", len(server.toolsRegistry))
	t.Logf("SCHEMA_BASELINE list_tools_payload_bytes=%d", len(payload))
	t.Logf("SCHEMA_BASELINE tool_struct_blocks=%d", toolBlocks)
	t.Logf("SCHEMA_BASELINE tool_structs_missing_input_schema=%d", missingInputSchema)
	for _, metric := range metrics {
		t.Logf("SCHEMA_BASELINE description_length tool=%s bytes=%d", metric.Name, metric.DescriptionLength)
	}
}

func scanToolStructsFromSource() (int, int, error) {
	src, err := os.ReadFile("server.go")
	if err != nil {
		return 0, 0, err
	}

	toolBlockPattern := regexp.MustCompile(`tool := &mcp\.Tool\{(?s:(.*?))\n\t\t\}`)
	matches := toolBlockPattern.FindAllStringSubmatch(string(src), -1)
	if len(matches) == 0 {
		return 0, 0, nil
	}

	missingInputSchema := 0
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		if !strings.Contains(match[1], "InputSchema:") {
			missingInputSchema++
		}
	}

	return len(matches), missingInputSchema, nil
}
