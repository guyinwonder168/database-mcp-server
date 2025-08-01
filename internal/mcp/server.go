// server.go
// Author: guyinwonder
// Version: v1.0.0
// Project created using OpenAI GPT-4.1 via VSCode AI code assistant extension.
// MCP server implementation for the database-mcp-provider project.
// Provides MCP actions for profile management, SQL execution, table/DB listing, and uses structured JSON logging.

package mcp

import (
	"context"
	"database-mcp-provider/internal/log"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCPServer represents the MCP server and its registered tools.
type MCPServer struct {
	server     *mcp.Server
	ConfigPath string // <--- Add this field for testability
}

const MCPVersion = "v1.0.0"
const MCPAuthor = "guyinwonder"

// NewMCPServer creates a new MCPServer instance using the MCP SDK.
func NewMCPServer() *MCPServer {
	return NewMCPServerWithConfig("config.yaml")
}

// NewMCPServerWithConfig allows specifying a config path (for testing).
func NewMCPServerWithConfig(configPath string) *MCPServer {
	srv := mcp.NewServer(&mcp.Implementation{Name: "database-mcp-provider", Version: MCPVersion}, nil)
	return &MCPServer{server: srv, ConfigPath: configPath}
}

// Start launches the MCP server, registers all tools, and starts listening for MCP requests.
func (s *MCPServer) Start() error {
	// Register all MCP tools/actions
	mcp.AddTool(s.server, &mcp.Tool{Name: "configure-profile", Description: "Configure a DB profile"}, s.handleConfigureProfile)
	mcp.AddTool(s.server, &mcp.Tool{Name: "list-profiles", Description: "List DB profiles"}, s.handleListProfiles)
	mcp.AddTool(s.server, &mcp.Tool{Name: "execute-sql", Description: "Execute SQL"}, s.handleExecuteSQL)
	mcp.AddTool(s.server, &mcp.Tool{Name: "list-tables", Description: "List tables"}, s.handleListTables)
	mcp.AddTool(s.server, &mcp.Tool{Name: "describe-table", Description: "Describe table"}, s.handleDescribeTable)
	mcp.AddTool(s.server, &mcp.Tool{Name: "list-databases", Description: "List databases/schemas"}, s.handleListDatabases)
	// Add version/author info tool
	mcp.AddTool(s.server, &mcp.Tool{Name: "mcp-info", Description: "Show MCP provider version and author"}, s.handleMCPInfo)
	log.JSONLog("info", "MCP server (SDK) running on stdio", nil)
	return s.server.Run(context.Background(), mcp.NewStdioTransport())
}

// handleMCPInfo returns author and version information.
func (s *MCPServer) handleMCPInfo(ctx context.Context, session *mcp.ServerSession, params *mcp.CallToolParamsFor[any]) (*mcp.CallToolResultFor[any], error) {
	return &mcp.CallToolResultFor[any]{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: "Database MCP Provider\nAuthor: " + MCPAuthor + "\nVersion: " + MCPVersion + "\nCreated using OpenAI GPT-4.1 via VSCode Kilocode AI code assistant extension.",
			},
		},
	}, nil
}
