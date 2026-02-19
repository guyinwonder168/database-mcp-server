package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestToolDescriptionsCompactModeAreSingleLine(t *testing.T) {
	configPath := writeSchemaModeConfig(t, "compact")
	server := NewMCPServerWithConfig(configPath)

	if len(server.toolsRegistry) == 0 {
		t.Fatal("expected tools to be registered")
	}

	for _, tool := range server.toolsRegistry {
		if strings.Contains(tool.Description, "\n") || strings.Contains(tool.Description, "\t") {
			t.Fatalf("expected compact description to be single-line for tool %q, got %q", tool.Name, tool.Description)
		}
	}
}

func TestToolDescriptionsStandardModePreserveVerboseContent(t *testing.T) {
	configPath := writeSchemaModeConfig(t, "standard")
	server := NewMCPServerWithConfig(configPath)

	if len(server.toolsRegistry) == 0 {
		t.Fatal("expected tools to be registered")
	}

	var executeSQLDescription string
	for _, tool := range server.toolsRegistry {
		if tool.Name == "execute-sql" {
			executeSQLDescription = tool.Description
			break
		}
	}

	if executeSQLDescription == "" {
		t.Fatal("expected execute-sql tool in registry")
	}

	if !strings.Contains(executeSQLDescription, "Example:") {
		t.Fatalf("expected standard mode to preserve verbose content, got %q", executeSQLDescription)
	}
}

func writeSchemaModeConfig(t *testing.T, mode string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := "profiles: []\nmax_pool_size: 5\naes_key: \"\"\nschema_mode: " + mode + "\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	return path
}
