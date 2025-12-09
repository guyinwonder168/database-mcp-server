//go:build cgo

package mcp

import (
	"context"
	"os"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestToolsList_Discovery ensures that the SDK-level tools/list call returns
// all registered tools during session initialization. This guards against the
// v0.2.0 SDK regression where tools/list was rejected in init.
func TestToolsList_Discovery(t *testing.T) {
	testConfig := setupTestConfig(t)
	defer func() {
		_ = os.Remove(testConfig)
	}()

	ctx := context.Background()

	server := NewMCPServerWithConfig(testConfig)
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v1.0.0"}, nil)

	serverTransport, clientTransport := mcp.NewInMemoryTransports()

	serverSession, err := server.server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server connect failed: %v", err)
	}
	t.Cleanup(func() { _ = serverSession.Close() })

	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect failed: %v", err)
	}
	t.Cleanup(func() { _ = clientSession.Close() })

	res, err := clientSession.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}

	if len(res.Tools) != len(server.toolsRegistry) {
		t.Fatalf("expected %d tools, got %d", len(server.toolsRegistry), len(res.Tools))
	}

	missing := map[string]bool{}
	for _, tool := range server.toolsRegistry {
		missing[tool.Name] = true
	}
	for _, tool := range res.Tools {
		delete(missing, tool.Name)
	}
	if len(missing) > 0 {
		t.Fatalf("tools/list missing tools: %v", missing)
	}
}

// TestToolsList_Capabilities ensures the server advertises tools/list in capabilities.
func TestToolsList_Capabilities(t *testing.T) {
	testConfig := setupTestConfig(t)
	defer func() { _ = os.Remove(testConfig) }()

	ctx := context.Background()
	server := NewMCPServerWithConfig(testConfig)
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v1.0.0"}, nil)

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server connect failed: %v", err)
	}
	t.Cleanup(func() { _ = serverSession.Close() })

	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect failed: %v", err)
	}
	t.Cleanup(func() { _ = clientSession.Close() })

	init := clientSession.InitializeResult()
	if init == nil || init.Capabilities == nil || init.Capabilities.Tools == nil || !init.Capabilities.Tools.ListChanged {
		t.Fatalf("server capabilities missing tools/list advertisement: %#v", init)
	}
}
