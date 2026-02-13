package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type GetToolHelpParams struct {
	ToolName string `json:"tool_name"`
	Topic    string `json:"topic,omitempty"`
}

type ToolHelpError struct {
	Error string `json:"error"`
	Cause string `json:"cause"`
	Fix   string `json:"fix"`
}

type GetToolHelpResult struct {
	ToolName        string          `json:"tool_name"`
	Found           bool            `json:"found"`
	Summary         string          `json:"summary,omitempty"`
	MinimalExample  map[string]any  `json:"minimal_example,omitempty"`
	AdvancedExample map[string]any  `json:"advanced_example,omitempty"`
	CommonErrors    []ToolHelpError `json:"common_errors,omitempty"`
	Notes           []string        `json:"notes,omitempty"`
	Topics          []string        `json:"topics"`
}

type toolHelpEntry struct {
	Summary         string
	MinimalExample  map[string]any
	AdvancedExample map[string]any
	CommonErrors    []ToolHelpError
	Notes           []string
}

var supportedToolHelpTopics = []string{"summary", "minimal_example", "advanced_example", "errors", "all"}

var toolHelpCatalog = map[string]toolHelpEntry{
	"configure-profile": {
		Summary:        "Create or update a database profile used by all DB tools.",
		MinimalExample: map[string]any{"profile_name": "analytics_db", "db_type": "sqlite", "database_name": "/tmp/demo.sqlite", "readonly": true},
		CommonErrors:   []ToolHelpError{{Error: "Profile not found", Cause: "Using unknown profile_name", Fix: "Create profile first with configure-profile"}},
	},
	"list-profiles": {
		Summary:        "List configured profiles.",
		MinimalExample: map[string]any{},
	},
	"execute-sql": {
		Summary:         "Execute SQL on a selected profile and database.",
		MinimalExample:  map[string]any{"profile_name": "analytics_db", "database_name": "analytics_db", "sql": "SELECT 1"},
		AdvancedExample: map[string]any{"profile_name": "analytics_db", "database_name": "analytics_db", "sql": "SELECT * FROM orders WHERE customer_id = ?", "params": []any{123}},
		CommonErrors:    []ToolHelpError{{Error: "Missing required parameters", Cause: "profile_name/database_name/sql omitted", Fix: "Provide all required fields"}},
	},
	"list-tables": {
		Summary:        "List tables in a database.",
		MinimalExample: map[string]any{"profile_name": "analytics_db", "database_name": "analytics_db"},
	},
	"describe-table": {
		Summary:        "Describe columns and metadata for one table.",
		MinimalExample: map[string]any{"profile_name": "analytics_db", "database_name": "analytics_db", "table_name": "orders"},
	},
	"list-databases": {
		Summary:        "List databases/schemas visible to a profile.",
		MinimalExample: map[string]any{"profile_name": "analytics_db"},
	},
	"analyze-schema": {
		Summary:        "Analyze schema metadata with selectable analysis depth.",
		MinimalExample: map[string]any{"profile_name": "analytics_db", "analysis_level": "basic"},
	},
	"smart-query-builder": {
		Summary:        "Generate SQL from natural-language intent.",
		MinimalExample: map[string]any{"profile_name": "analytics_db", "intent": "monthly sales"},
	},
	"optimize-query": {
		Summary:        "Run EXPLAIN-based optimization analysis.",
		MinimalExample: map[string]any{"profile_name": "analytics_db", "database_name": "analytics_db", "sql": "SELECT * FROM orders WHERE customer_id = 123"},
	},
	"validate-query": {
		Summary:        "Validate SQL syntax and risky patterns without execution.",
		MinimalExample: map[string]any{"profile_name": "analytics_db", "sql": "SELECT * FROM users"},
	},
	"analyze-data-lineage": {
		Summary:        "Trace upstream/downstream relationships for a table.",
		MinimalExample: map[string]any{"profile_name": "analytics_db", "table_name": "orders", "scope": "both"},
	},
	"discover-insights": {
		Summary:        "Discover KPI, trend, anomaly, and distribution insights.",
		MinimalExample: map[string]any{"profile_name": "analytics_db", "database_name": "analytics_db", "table_name": "orders"},
	},
	"track-schema-changes": {
		Summary:        "Track schema snapshots and detect drift/migrations.",
		MinimalExample: map[string]any{"profile_name": "analytics_db", "operation": "track"},
	},
	"federated-query": {
		Summary:        "Execute read-only cross-profile queries with optional joins.",
		MinimalExample: map[string]any{"sub_queries": []any{map[string]any{"profile": "analytics_db", "sql": "SELECT 1", "alias": "q1"}}},
	},
	"discover-joins": {
		Summary:        "Suggest joins based on foreign key relationships.",
		MinimalExample: map[string]any{"profile_name": "analytics_db"},
	},
	"sample-data": {
		Summary:        "Fetch sample rows from a table.",
		MinimalExample: map[string]any{"profile_name": "analytics_db", "database_name": "analytics_db", "table_name": "users"},
	},
	"mcp-info": {
		Summary:        "Return MCP provider metadata.",
		MinimalExample: map[string]any{},
	},
	"list-tools": {
		Summary:        "Return tool catalog and descriptions.",
		MinimalExample: map[string]any{},
	},
	"get-tool-help": {
		Summary:        "Return examples and troubleshooting for a tool.",
		MinimalExample: map[string]any{"tool_name": "execute-sql", "topic": "all"},
	},
}

func (s *MCPServer) handleGetToolHelp(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input GetToolHelpParams,
) (*mcp.CallToolResult, any, error) {
	_ = ctx
	if input.ToolName == "" {
		return nil, nil, fmt.Errorf("tool_name is required")
	}
	if !isSupportedToolHelpTopic(input.Topic) {
		return nil, nil, fmt.Errorf("invalid topic %q: must be one of summary|minimal_example|advanced_example|errors|all", input.Topic)
	}

	entry, found := toolHelpCatalog[input.ToolName]
	if !found {
		result := GetToolHelpResult{
			ToolName: input.ToolName,
			Found:    false,
			Summary:  "Tool not found. Use list-tools to discover available tool names.",
			Notes: []string{
				"Use exact tool name from list-tools output.",
				"Try get-tool-help with topic=all after selecting a valid tool.",
			},
			Topics: supportedToolHelpTopics,
		}
		return marshalToolHelpResult(result)
	}

	result := topicFilteredToolHelpResult(input.ToolName, input.Topic, entry)
	return marshalToolHelpResult(result)
}

func isSupportedToolHelpTopic(topic string) bool {
	if topic == "" {
		return true
	}
	for _, allowed := range supportedToolHelpTopics {
		if topic == allowed {
			return true
		}
	}
	return false
}

func topicFilteredToolHelpResult(toolName string, topic string, entry toolHelpEntry) GetToolHelpResult {
	if topic == "" {
		topic = "all"
	}
	result := GetToolHelpResult{ToolName: toolName, Found: true, Topics: supportedToolHelpTopics}
	switch topic {
	case "summary":
		result.Summary = entry.Summary
	case "minimal_example":
		result.MinimalExample = entry.MinimalExample
	case "advanced_example":
		result.AdvancedExample = entry.AdvancedExample
	case "errors":
		result.CommonErrors = entry.CommonErrors
	default:
		result.Summary = entry.Summary
		result.MinimalExample = entry.MinimalExample
		result.AdvancedExample = entry.AdvancedExample
		result.CommonErrors = entry.CommonErrors
		result.Notes = entry.Notes
	}
	return result
}

func marshalToolHelpResult(result GetToolHelpResult) (*mcp.CallToolResult, any, error) {
	b, err := json.Marshal(result)
	if err != nil {
		return nil, nil, err
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
	}, nil, nil
}
