//go:build cgo

package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestProviderSmoke_GeminiCompactPath(t *testing.T) {
	server := NewMCPServerWithConfig(writeSmokeConfig(t, "compact"))
	ctx := context.Background()

	listRes, _, err := server.handleListTools(ctx, nil, ListToolsParams{})
	if err != nil {
		t.Fatalf("list-tools failed in compact mode: %v", err)
	}
	tools := decodeListToolsResult(t, listRes)
	if len(tools.Tools) == 0 {
		t.Fatal("expected non-empty tool list in compact mode")
	}

	for _, tool := range tools.Tools {
		if strings.Contains(tool.Description, "\n") || strings.Contains(tool.Description, "\t") {
			t.Fatalf("compact mode description contains multiline escape for %q", tool.Name)
		}
	}

	helpRes, _, err := server.handleGetToolHelp(ctx, nil, GetToolHelpParams{ToolName: "execute-sql", Topic: "minimal_example"})
	if err != nil {
		t.Fatalf("get-tool-help failed in compact mode: %v", err)
	}
	helpBody := decodeToolHelpResponse(t, helpRes)
	if !helpBody.Found || len(helpBody.MinimalExample) == 0 {
		t.Fatal("expected helper response with minimal example in compact mode")
	}
}

func TestProviderSmoke_ClaudeStandardPath(t *testing.T) {
	server := NewMCPServerWithConfig(writeSmokeConfig(t, "standard"))
	ctx := context.Background()

	listRes, _, err := server.handleListTools(ctx, nil, ListToolsParams{})
	if err != nil {
		t.Fatalf("list-tools failed in standard mode: %v", err)
	}
	tools := decodeListToolsResult(t, listRes)
	if len(tools.Tools) == 0 {
		t.Fatal("expected non-empty tool list in standard mode")
	}

	executeSQL := findToolByName(tools.Tools, "execute-sql")
	if executeSQL == nil {
		t.Fatal("expected execute-sql in standard tool list")
	}
	if !strings.Contains(executeSQL.Description, "Example:") {
		t.Fatalf("expected verbose description in standard mode, got: %q", executeSQL.Description)
	}

	helpRes, _, err := server.handleGetToolHelp(ctx, nil, GetToolHelpParams{ToolName: "execute-sql", Topic: "errors"})
	if err != nil {
		t.Fatalf("get-tool-help failed in standard mode: %v", err)
	}
	helpBody := decodeToolHelpResponse(t, helpRes)
	if !helpBody.Found || len(helpBody.CommonErrors) == 0 {
		t.Fatal("expected helper response with common errors in standard mode")
	}
}

func writeSmokeConfig(t *testing.T, schemaMode string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := "profiles: []\nmax_pool_size: 5\naes_key: \"\"\nschema_mode: " + schemaMode + "\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write smoke config: %v", err)
	}
	return path
}

func decodeListToolsResult(t *testing.T, res *mcp.CallToolResult) ListToolsResult {
	t.Helper()
	if res == nil || len(res.Content) == 0 {
		t.Fatal("expected non-empty list-tools response")
	}
	textContent, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", res.Content[0])
	}
	var body ListToolsResult
	if err := json.Unmarshal([]byte(textContent.Text), &body); err != nil {
		t.Fatalf("failed to decode list-tools response: %v", err)
	}
	return body
}

func findToolByName(tools []ToolInfo, name string) *ToolInfo {
	for i := range tools {
		if tools[i].Name == name {
			return &tools[i]
		}
	}
	return nil
}
