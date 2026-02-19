package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfigDefaultsSchemaModeToCompact(t *testing.T) {
	path := writeTestConfigYAML(t, "profiles: []\nmax_pool_size: 5\naes_key: \"\"\n")

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}

	if cfg.SchemaMode != SchemaModeCompact {
		t.Fatalf("expected schema mode %q, got %q", SchemaModeCompact, cfg.SchemaMode)
	}
}

func TestLoadConfigAcceptsValidSchemaModes(t *testing.T) {
	tests := []struct {
		name string
		mode string
	}{
		{name: "compact", mode: string(SchemaModeCompact)},
		{name: "standard", mode: string(SchemaModeStandard)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			content := "profiles: []\nmax_pool_size: 5\naes_key: \"\"\nschema_mode: " + tc.mode + "\n"
			path := writeTestConfigYAML(t, content)

			cfg, err := LoadConfig(path)
			if err != nil {
				t.Fatalf("LoadConfig returned error: %v", err)
			}

			if string(cfg.SchemaMode) != tc.mode {
				t.Fatalf("expected schema mode %q, got %q", tc.mode, cfg.SchemaMode)
			}
		})
	}
}

func TestLoadConfigRejectsInvalidSchemaMode(t *testing.T) {
	path := writeTestConfigYAML(t, "profiles: []\nmax_pool_size: 5\naes_key: \"\"\nschema_mode: invalid\n")

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for invalid schema_mode")
	}

	if !strings.Contains(err.Error(), "schema_mode") {
		t.Fatalf("expected schema_mode in error, got: %v", err)
	}
}

func writeTestConfigYAML(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}
	return path
}
