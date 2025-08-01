// Author: guyinwonder
// Version: v1.0.0
// Project created using OpenAI GPT-4.1 via VSCode Kilocode AI code assistant extension.

package main

import (
	"database-mcp-provider/internal/config"
	"database-mcp-provider/internal/log"
	"database-mcp-provider/internal/mcp"
	"os"
)

func main() {
	// Initialize structured JSON logger with rotation
	if err := log.Init("mcp-provider.log"); err != nil {
		log.JSONLog("fatal", "Failed to initialize logger", map[string]interface{}{"error": err.Error()})
		os.Exit(1)
	}

	log.JSONLog("info", "Database MCP Provider starting...", nil)

	const configPath = "config.yaml"
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		profiles, maxPoolSize, aesKey := config.PromptForProfiles()
		cfg := &config.Config{Profiles: profiles, MaxPoolSize: maxPoolSize, AESKey: aesKey}
		if err := config.SaveConfig(configPath, cfg); err != nil {
			log.JSONLog("fatal", "Failed to save config.yaml", map[string]interface{}{"error": err.Error()})
			os.Exit(1)
		}
		log.JSONLog("info", "config.yaml created", nil)
	}

	// Start MCP server
	server := mcp.NewMCPServer()
	if err := server.Start(); err != nil {
		log.JSONLog("fatal", "Failed to start MCP server", map[string]interface{}{"error": err.Error()})
		os.Exit(1)
	}
}
