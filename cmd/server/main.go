// Author: guyinwonder
// Version: v1.0.0
// Project created using OpenAI GPT-4.1 via VSCode Kilocode AI code assistant extension.

package main

import (
	"database-mcp-provider/internal/log"
	"database-mcp-provider/internal/mcp"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	exeDir, err := executableDir()
	if err != nil {
		fatalWithError("Failed to resolve executable directory", err, nil)
	}
	if err := initLogger(exeDir); err != nil {
		fatalWithError("Failed to initialize logger", err, nil)
	}

	log.JSONLog("info", "Database MCP Provider starting...", nil)

	configPath := filepath.Join(exeDir, "config.yaml")
	log.JSONLog("debug", "Resolved config.yaml path", map[string]interface{}{"configPath": configPath})
	if err := ensureConfigFile(configPath); err != nil {
		fatalWithError("Failed during config self-healing", err, map[string]interface{}{"configPath": configPath})
	}

	log.JSONLog("debug", "About to initialize MCP server", nil)
	server := mcp.NewMCPServerWithConfig(configPath)
	startSSEIfEnabled(server)

	log.JSONLog("debug", "MCP server instance created, starting server...", map[string]interface{}{"configPath": configPath})
	if err := server.Start(); err != nil {
		fatalWithError("Failed to start MCP server", err, nil)
	}
	log.JSONLog("info", "MCP server exited normally", nil)
}

func executableDir() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Dir(exePath), nil
}

func initLogger(exeDir string) error {
	logDir := filepath.Join(exeDir, "log")
	if err := os.MkdirAll(logDir, 0750); err != nil {
		return err
	}
	return log.Init(filepath.Join(logDir, "mcp-provider.log"))
}

func ensureConfigFile(configPath string) error {
	if _, err := os.Stat(configPath); err == nil {
		log.JSONLog("debug", "config.yaml found, proceeding with startup", map[string]interface{}{"configPath": configPath})
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	log.JSONLog("warn", "config.yaml not found at startup. MCP server will start and allow configuration via MCP actions only; no interactive prompt will be triggered.", map[string]interface{}{"configPath": configPath})
	minimalConfig := buildMinimalConfigPayload()
	if err := os.WriteFile(configPath, minimalConfig, 0600); err != nil {
		return err
	}
	log.JSONLog("info", "Auto-created minimal config.yaml for self-healing", map[string]interface{}{"configPath": configPath})
	return nil
}

func buildMinimalConfigPayload() []byte {
	return []byte(fmt.Sprintf("profiles: []\nmax_pool_size: 5\naes_key: %q\nschema_mode: compact\n", randomASCIIKey(32)))
}

func randomASCIIKey(length int) string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	f, err := os.Open("/dev/urandom")
	if err != nil {
		return deterministicASCIIKey(length, charset)
	}
	defer f.Close() //nolint:errcheck // Standard pattern: error in deferred close is not critical

	buf := make([]byte, length)
	if _, err := f.Read(buf); err != nil {
		return deterministicASCIIKey(length, charset)
	}
	key := make([]byte, length)
	for idx := range key {
		key[idx] = charset[int(buf[idx])%len(charset)]
	}
	return string(key)
}

func deterministicASCIIKey(length int, charset string) string {
	key := make([]byte, length)
	for idx := range key {
		key[idx] = charset[idx%len(charset)]
	}
	return string(key)
}

func startSSEIfEnabled(server *mcp.MCPServer) {
	sseAddr := os.Getenv("MCP_SSE_ADDR")
	if sseAddr == "" {
		return
	}
	go runSSEServer(sseAddr, server)
}

func runSSEServer(sseAddr string, server *mcp.MCPServer) {
	handler := mcpsdk.NewSSEHandler(func(_ *http.Request) *mcpsdk.Server {
		return server.Server()
	}, nil)
	log.JSONLog("info", "Starting MCP SSE server", map[string]interface{}{"addr": sseAddr})
	httpServer := &http.Server{
		Addr:         sseAddr,
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	if err := httpServer.ListenAndServe(); err != nil {
		fatalWithError("Failed to start MCP SSE server", err, map[string]interface{}{"addr": sseAddr})
	}
}

func fatalWithError(message string, err error, fields map[string]interface{}) {
	payload := map[string]interface{}{}
	for key, value := range fields {
		payload[key] = value
	}
	if err != nil {
		payload["error"] = err.Error()
	}
	log.JSONLog("fatal", message, payload)
	os.Exit(1)
}
