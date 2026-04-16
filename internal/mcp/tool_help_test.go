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

// TestToolHelpCatalogEntryCompleteness verifies that every catalog entry has
// the required fields populated: Description, Summary, MinimalExample,
// AdvancedExample, and CommonErrors. Parameters may be empty for no-param tools.
// This is a regression test for BUG-003.
func TestToolHelpCatalogEntryCompleteness(t *testing.T) {
	for toolName, entry := range toolHelpCatalog {
		t.Run(toolName, func(t *testing.T) {
			if entry.Summary == "" {
				t.Errorf("catalog entry %q: Summary is empty", toolName)
			}
			if entry.Description == "" {
				t.Errorf("catalog entry %q: Description is empty", toolName)
			}
			if len(entry.MinimalExample) == 0 {
				t.Errorf("catalog entry %q: MinimalExample is empty", toolName)
			}
			if len(entry.AdvancedExample) == 0 {
				t.Errorf("catalog entry %q: AdvancedExample is empty", toolName)
			}
			if len(entry.CommonErrors) == 0 {
				t.Errorf("catalog entry %q: CommonErrors is empty", toolName)
			}
		})
	}
}

// TestHandleGetToolHelp_TopicReturnsContent verifies that requesting specific
// topics returns actual content, not empty/nil values. Regression test for BUG-003.
func TestHandleGetToolHelp_TopicReturnsContent(t *testing.T) {
	testConfig := setupTestConfig(t)
	defer os.Remove(testConfig)

	server := NewMCPServerWithConfig(testConfig)
	topics := []string{"advanced_example", "errors"}
	sampleTools := []string{"list-tables", "describe-table", "analyze-schema", "sample-data"}

	for _, toolName := range sampleTools {
		for _, topic := range topics {
			t.Run(toolName+"/"+topic, func(t *testing.T) {
				res, _, err := server.handleGetToolHelp(context.Background(), nil, GetToolHelpParams{
					ToolName: toolName,
					Topic:    topic,
				})
				if err != nil {
					t.Fatalf("handleGetToolHelp(%q, %q) error: %v", toolName, topic, err)
				}

				body := decodeToolHelpResponse(t, res)
				if !body.Found {
					t.Fatalf("expected found=true for %q", toolName)
				}

				switch topic {
				case "advanced_example":
					if len(body.AdvancedExample) == 0 {
						t.Errorf("topic=advanced_example for %q returned empty AdvancedExample", toolName)
					}
				case "errors":
					if len(body.CommonErrors) == 0 {
						t.Errorf("topic=errors for %q returned empty CommonErrors", toolName)
					}
				}
			})
		}
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
