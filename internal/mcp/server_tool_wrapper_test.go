package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestWrapToolHandler_ConvertsErrorsToStructuredResult(t *testing.T) {
	handler := func(context.Context, *mcp.CallToolRequest, map[string]any) (*mcp.CallToolResult, map[string]any, error) {
		return nil, nil, errors.New("boom")
	}

	wrapped := wrapToolHandler("demo-tool", handler)
	res, out, err := wrapped(context.Background(), nil, map[string]any{})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected zero output, got %+v", out)
	}
	if res == nil || !res.IsError {
		t.Fatal("expected IsError=true result")
	}
	if len(res.Content) == 0 {
		t.Fatal("expected error content")
	}

	textContent, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", res.Content[0])
	}

	var payload map[string]any
	if unmarshalErr := json.Unmarshal([]byte(textContent.Text), &payload); unmarshalErr != nil {
		t.Fatalf("failed to decode structured error payload: %v", unmarshalErr)
	}
	if payload["error_code"] != string(ErrorCodeInternalError) {
		t.Fatalf("expected error_code %q, got %v", ErrorCodeInternalError, payload["error_code"])
	}
	ctxMap, ok := payload["context"].(map[string]any)
	if !ok {
		t.Fatalf("expected context map, got %T", payload["context"])
	}
	if ctxMap["tool_name"] != "demo-tool" {
		t.Fatalf("expected tool_name demo-tool, got %v", ctxMap["tool_name"])
	}
}

func TestWrapToolHandler_PassesThroughSuccess(t *testing.T) {
	handler := func(context.Context, *mcp.CallToolRequest, map[string]any) (*mcp.CallToolResult, map[string]any, error) {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "ok"}}}, map[string]any{"status": "ok"}, nil
	}

	wrapped := wrapToolHandler("demo-tool", handler)
	res, out, err := wrapped(context.Background(), nil, map[string]any{})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if res == nil || res.IsError {
		t.Fatal("expected non-error result")
	}
	if out["status"] != "ok" {
		t.Fatalf("expected status ok, got %v", out["status"])
	}
}
