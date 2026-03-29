//go:build cgo

package mcp

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestRegisterAllTools_IncludesGetToolHelp(t *testing.T) {
	testConfig := setupTestConfig(t)
	defer os.Remove(testConfig)

	server := NewMCPServerWithConfig(testConfig)
	for _, tool := range server.toolsRegistry {
		if tool.Name == "get-tool-help" {
			return
		}
	}
	t.Fatal("get-tool-help tool not found in tools registry")
}

func TestToolHelpCatalog_CoversAllRegisteredTools(t *testing.T) {
	testConfig := setupTestConfig(t)
	defer os.Remove(testConfig)

	server := NewMCPServerWithConfig(testConfig)
	for _, tool := range server.toolsRegistry {
		if _, ok := toolHelpCatalog[tool.Name]; !ok {
			t.Fatalf("missing tool help catalog entry for %q", tool.Name)
		}
	}
}

func TestHandleGetToolHelp_KnownTool(t *testing.T) {
	testConfig := setupTestConfig(t)
	defer os.Remove(testConfig)

	server := NewMCPServerWithConfig(testConfig)
	res, _, err := server.handleGetToolHelp(context.Background(), nil, GetToolHelpParams{
		ToolName: "execute-sql",
		Topic:    "all",
	})
	if err != nil {
		t.Fatalf("handleGetToolHelp returned error: %v", err)
	}

	body := decodeToolHelpResponse(t, res)
	if body.ToolName != "execute-sql" {
		t.Fatalf("expected tool_name execute-sql, got %q", body.ToolName)
	}
	if body.Summary == "" {
		t.Fatal("expected non-empty summary")
	}
	if len(body.MinimalExample) == 0 {
		t.Fatal("expected minimal_example for known tool")
	}
}

func TestHandleGetToolHelp_UnknownTool(t *testing.T) {
	testConfig := setupTestConfig(t)
	defer os.Remove(testConfig)

	server := NewMCPServerWithConfig(testConfig)
	res, _, err := server.handleGetToolHelp(context.Background(), nil, GetToolHelpParams{
		ToolName: "unknown-tool",
		Topic:    "all",
	})
	if err != nil {
		t.Fatalf("handleGetToolHelp returned error: %v", err)
	}

	body := decodeToolHelpResponse(t, res)
	if body.ToolName != "unknown-tool" {
		t.Fatalf("expected echoed tool_name unknown-tool, got %q", body.ToolName)
	}
	if body.Found {
		t.Fatal("expected found=false for unknown tool")
	}
	if body.Summary == "" {
		t.Fatal("expected fallback summary for unknown tool")
	}
}

func decodeToolHelpResponse(t *testing.T, res *mcp.CallToolResult) GetToolHelpResult {
	t.Helper()
	if res == nil || len(res.Content) == 0 {
		t.Fatal("expected non-empty tool result content")
	}
	textContent, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", res.Content[0])
	}

	var body GetToolHelpResult
	if err := json.Unmarshal([]byte(textContent.Text), &body); err != nil {
		t.Fatalf("failed to decode helper response: %v", err)
	}
	return body
}
