package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

type toolDeclarationMetric struct {
	Name  string
	Bytes int
}

func TestMCPDeclarationBaselineMetrics(t *testing.T) {
	server := NewMCPServerWithConfig("nonexistent_config.yaml")
	if len(server.toolDecls) == 0 {
		t.Fatal("expected MCP tool declarations to be recorded")
	}

	payload, err := json.Marshal(struct {
		Tools []ToolDeclarationInfo `json:"tools"`
	}{Tools: server.toolDecls})
	if err != nil {
		t.Fatalf("failed to marshal MCP declaration payload: %v", err)
	}

	metrics := make([]toolDeclarationMetric, 0, len(server.toolDecls))
	for _, tool := range server.toolDecls {
		b, err := json.Marshal(tool)
		if err != nil {
			t.Fatalf("failed to marshal declaration for tool %q: %v", tool.Name, err)
		}
		metrics = append(metrics, toolDeclarationMetric{Name: tool.Name, Bytes: len(b)})
	}

	sort.Slice(metrics, func(i, j int) bool { return metrics[i].Bytes > metrics[j].Bytes })

	t.Logf("MCP_DECLARATION_BASELINE tool_count=%d", len(server.toolDecls))
	t.Logf("MCP_DECLARATION_BASELINE payload_bytes=%d", len(payload))
	for i, metric := range metrics {
		if i >= 8 {
			break
		}
		t.Logf("MCP_DECLARATION_BASELINE top_tool_bytes rank=%d tool=%s bytes=%d", i+1, metric.Name, metric.Bytes)
	}
}

func TestMCPDeclarationCompactProfileMetrics(t *testing.T) {
	configPath := writeMCPDeclarationConfig(t, "compact")
	server := NewMCPServerWithConfig(configPath)
	if len(server.toolDecls) == 0 {
		t.Fatal("expected MCP tool declarations to be recorded")
	}

	payload, err := declarationPayloadBytes(server.toolDecls)
	if err != nil {
		t.Fatalf("failed to marshal MCP declaration payload: %v", err)
	}

	t.Logf("MCP_DECLARATION_COMPACT tool_count=%d", len(server.toolDecls))
	t.Logf("MCP_DECLARATION_COMPACT payload_bytes=%d", payload)

	if payload > 10*1024 {
		t.Fatalf("compact declaration payload too large: %d > %d", payload, 10*1024)
	}
}

func declarationPayloadBytes(decls []ToolDeclarationInfo) (int, error) {
	payload, err := json.Marshal(struct {
		Tools []ToolDeclarationInfo `json:"tools"`
	}{Tools: decls})
	if err != nil {
		return 0, err
	}
	return len(payload), nil
}

func writeMCPDeclarationConfig(t *testing.T, schemaMode string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := "profiles: []\nmax_pool_size: 5\naes_key: \"\"\nschema_mode: " + schemaMode + "\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write declaration config: %v", err)
	}
	return path
}
