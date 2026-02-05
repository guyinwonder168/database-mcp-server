// Author: guyinwonder
// Version: v1.0.0
// Project created using OpenAI GPT-4.1 via VSCode Kilocode AI code assistant extension.

package main

import (
	"database-mcp-provider/internal/log"
	"database-mcp-provider/internal/mcp"
	"net/http"
	"os"
	"path/filepath"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	// Ensure log directory exists in the same folder as the binary
	exePath, err := os.Executable()
	if err != nil {
		log.JSONLog("fatal", "Failed to get executable path", map[string]interface{}{"error": err.Error()})
		os.Exit(1)
	}
	exeDir := filepath.Dir(exePath)
	logDir := filepath.Join(exeDir, "log")
	if err := os.MkdirAll(logDir, 0750); err != nil {
		log.JSONLog("fatal", "Failed to create log directory", map[string]interface{}{"error": err.Error()})
		os.Exit(1)
	}
	logPath := logDir + "/mcp-provider.log"
	// Initialize structured JSON logger with rotation
	if err := log.Init(logPath); err != nil {
		log.JSONLog("fatal", "Failed to initialize logger", map[string]interface{}{"error": err.Error()})
		os.Exit(1)
	}

	log.JSONLog("info", "Database MCP Provider starting...", nil)

	// Use config.yaml in the same directory as the binary
	configPath := filepath.Join(exeDir, "config.yaml")
	log.JSONLog("debug", "Resolved config.yaml path", map[string]interface{}{"configPath": configPath})
	// Do not block on interactive setup at startup.
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		log.JSONLog("warn", "config.yaml not found at startup. MCP server will start and allow configuration via MCP actions only; no interactive prompt will be triggered.", map[string]interface{}{"configPath": configPath})
		// Self-healing: create a minimal config.yaml if missing, so server always has a valid config file
		// Generate a random 32-character ASCII AES key for config.yaml
		const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
		aesKey := make([]byte, 32)
		{
			f, err := os.Open("/dev/urandom")
			if err == nil {
				defer f.Close()
				b := make([]byte, 32)
				_, _ = f.Read(b)
				for i := range aesKey {
					aesKey[i] = charset[int(b[i])%len(charset)]
				}
			} else {
				// fallback: deterministic but not secure, for environments without urandom
				for i := range aesKey {
					aesKey[i] = charset[i%len(charset)]
				}
			}
		}
		minimalConfig := []byte("profiles: []\nmax_pool_size: 5\naes_key: \"" + string(aesKey) + "\"\n")
		if err := os.WriteFile(configPath, minimalConfig, 0600); err != nil {
			log.JSONLog("error", "Failed to auto-create minimal config.yaml", map[string]interface{}{"error": err.Error(), "configPath": configPath})
		} else {
			log.JSONLog("info", "Auto-created minimal config.yaml for self-healing", map[string]interface{}{"configPath": configPath})
		}
	} else {
		log.JSONLog("debug", "config.yaml found, proceeding with startup", map[string]interface{}{"configPath": configPath})
	}

	// Start MCP server
	log.JSONLog("debug", "About to initialize MCP server", nil)
	server := mcp.NewMCPServerWithConfig(configPath)

	// Optional SSE transport (HTTP) for MCP.
	if sseAddr := os.Getenv("MCP_SSE_ADDR"); sseAddr != "" {
		go func() {
			handler := mcpsdk.NewSSEHandler(func(_ *http.Request) *mcpsdk.Server {
				return server.Server()
			}, nil)
			log.JSONLog("info", "Starting MCP SSE server", map[string]interface{}{"addr": sseAddr})
			server := &http.Server{
				Addr:         sseAddr,
				Handler:      handler,
				ReadTimeout:  30 * time.Second,
				WriteTimeout: 30 * time.Second,
				IdleTimeout:  120 * time.Second,
			}
			if err := server.ListenAndServe(); err != nil {
				log.JSONLog("fatal", "Failed to start MCP SSE server", map[string]interface{}{"error": err.Error(), "addr": sseAddr})
				os.Exit(1)
			}
		}()
	}

	log.JSONLog("debug", "MCP server instance created, starting server...", map[string]interface{}{"configPath": configPath})
	if err := server.Start(); err != nil {
		log.JSONLog("fatal", "Failed to start MCP server", map[string]interface{}{"error": err.Error()})
		os.Exit(1)
	}
	log.JSONLog("info", "MCP server exited normally", nil)
}
