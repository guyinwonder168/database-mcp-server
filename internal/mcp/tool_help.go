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

type ToolParamInfo struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Required    bool     `json:"required"`
	Description string   `json:"description"`
	Enum        []string `json:"enum,omitempty"`
	Default     string   `json:"default,omitempty"`
}

type GetToolHelpResult struct {
	ToolName        string          `json:"tool_name"`
	Found           bool            `json:"found"`
	Description     string          `json:"description,omitempty"`
	Summary         string          `json:"summary,omitempty"`
	Parameters      []ToolParamInfo `json:"parameters,omitempty"`
	MinimalExample  map[string]any  `json:"minimal_example,omitempty"`
	AdvancedExample map[string]any  `json:"advanced_example,omitempty"`
	CommonErrors    []ToolHelpError `json:"common_errors,omitempty"`
	Notes           []string        `json:"notes,omitempty"`
	ResponseFormat  string          `json:"response_format,omitempty"`
	Topics          []string        `json:"topics"`
}

type toolHelpEntry struct {
	Description     string
	Parameters      []ToolParamInfo
	Summary         string
	MinimalExample  map[string]any
	AdvancedExample map[string]any
	CommonErrors    []ToolHelpError
	Notes           []string
	ResponseFormat  string
}

var supportedToolHelpTopics = []string{"summary", "minimal_example", "advanced_example", "errors", "all"}

var toolHelpCatalog = map[string]toolHelpEntry{
	"configure-profile": {
		Description: "Create, update, delete, or clone a database connection profile. Profiles store connection details (host, port, credentials) used by all other database tools.",
		Parameters: []ToolParamInfo{
			{Name: "profile_name", Type: "string", Required: true, Description: "Unique name for the database profile"},
			{Name: "action", Type: "string", Required: false, Description: "Operation to perform: create (default), update, delete, or clone", Enum: []string{"create", "update", "delete", "clone"}},
			{Name: "db_type", Type: "string", Required: false, Description: "Database type: postgres, mysql, mariadb, or sqlite", Enum: []string{"postgres", "mysql", "mariadb", "sqlite"}},
			{Name: "host", Type: "string", Required: false, Description: "Database host address"},
			{Name: "port", Type: "integer", Required: false, Description: "Database port number"},
			{Name: "username", Type: "string", Required: false, Description: "Database username"},
			{Name: "password", Type: "string", Required: false, Description: "Database password (stored encrypted with AES-256)"},
			{Name: "database_name", Type: "string", Required: false, Description: "Default database name for the profile"},
			{Name: "readonly", Type: "boolean", Required: false, Description: "If true, restricts the profile to read-only operations", Default: "false"},
			{Name: "sslmode", Type: "string", Required: false, Description: "SSL mode for PostgreSQL connections"},
			{Name: "source_profile", Type: "string", Required: false, Description: "Profile name to clone from (only used with action=clone)"},
		},
		Summary:         "Create, update, delete, or clone a database profile used by all DB tools.",
		MinimalExample:  map[string]any{"profile_name": "analytics_db", "db_type": "sqlite", "database_name": "/tmp/demo.sqlite", "readonly": true},
		AdvancedExample: map[string]any{"action": "clone", "profile_name": "analytics_ro", "source_profile": "analytics_db", "readonly": true},
		CommonErrors: []ToolHelpError{
			{Error: "Profile not found", Cause: "Using unknown profile_name", Fix: "Create profile first with configure-profile"},
			{Error: "Profile not found (delete)", Cause: "Deleting non-existent profile", Fix: "Check profile name with list-profiles"},
			{Error: "Source profile not found (clone)", Cause: "source_profile does not exist", Fix: "Verify source profile name with list-profiles"},
		},
		ResponseFormat: "JSON object with profile details and status",
	},
	"list-profiles": {
		Description:     "List all configured database profiles. Returns profile names and database types. No parameters required.",
		Parameters:      []ToolParamInfo{},
		Summary:         "List configured profiles.",
		MinimalExample:  map[string]any{"_note": "No parameters required"},
		AdvancedExample: map[string]any{"_note": "No additional parameters available"},
		CommonErrors: []ToolHelpError{
			{Error: "No profiles configured", Cause: "No profiles exist in config", Fix: "Use configure-profile to create a profile first"},
		},
		ResponseFormat: "JSON array of profile names and db_types",
	},
	"execute-sql": {
		Description: "Execute arbitrary SQL queries on a selected database profile. Supports parameterized queries to prevent SQL injection. Returns column names, row data, and affected row count.",
		Parameters: []ToolParamInfo{
			{Name: "profile_name", Type: "string", Required: true, Description: "Database profile to use"},
			{Name: "database_name", Type: "string", Required: true, Description: "Database name to query"},
			{Name: "sql", Type: "string", Required: true, Description: "SQL statement to execute"},
			{Name: "params", Type: "array", Required: false, Description: "Positional parameters for parameterized queries"},
		},
		Summary:         "Execute SQL on a selected profile and database.",
		MinimalExample:  map[string]any{"profile_name": "analytics_db", "database_name": "analytics_db", "sql": "SELECT 1"},
		AdvancedExample: map[string]any{"profile_name": "analytics_db", "database_name": "analytics_db", "sql": "SELECT * FROM orders WHERE customer_id = ?", "params": []any{123}},
		CommonErrors: []ToolHelpError{
			{Error: "Missing required parameters", Cause: "profile_name/database_name/sql omitted", Fix: "Provide all required fields"},
			{Error: "Profile not found", Cause: "profile_name does not match a configured profile", Fix: "Use list-profiles to see available profiles"},
			{Error: "Read-only violation", Cause: "Attempting INSERT/UPDATE/DELETE on a read-only profile", Fix: "Use a non-read-only profile or SELECT queries only"},
		},
		ResponseFormat: "JSON object with columns, rows, and affected count",
	},
	"list-tables": {
		Description: "List all tables (and views) in a database. Returns table names with their schema context. Works across MySQL, MariaDB, PostgreSQL, and SQLite.",
		Parameters: []ToolParamInfo{
			{Name: "profile_name", Type: "string", Required: true, Description: "Database profile to use"},
			{Name: "database_name", Type: "string", Required: true, Description: "Database name to list tables from"},
		},
		Summary:         "List tables in a database.",
		MinimalExample:  map[string]any{"profile_name": "analytics_db", "database_name": "analytics_db"},
		AdvancedExample: map[string]any{"profile_name": "analytics_db", "database_name": "analytics_db"},
		CommonErrors: []ToolHelpError{
			{Error: "Profile not found", Cause: "profile_name does not match a configured profile", Fix: "Use list-profiles to see available profiles"},
			{Error: "Database not found", Cause: "database_name does not exist on the server", Fix: "Use list-databases to see available databases"},
		},
		ResponseFormat: "JSON object with tables array containing {schema, table} objects",
	},
	"describe-table": {
		Description: "Describe columns, types, keys, and metadata for a specific table. Returns column names, data types, nullability, keys, defaults, and other attributes.",
		Parameters: []ToolParamInfo{
			{Name: "profile_name", Type: "string", Required: true, Description: "Database profile to use"},
			{Name: "database_name", Type: "string", Required: true, Description: "Database name containing the table"},
			{Name: "table_name", Type: "string", Required: true, Description: "Name of the table to describe"},
			{Name: "schema", Type: "string", Required: false, Description: "Schema name (PostgreSQL only, auto-detected if empty)"},
		},
		Summary:         "Describe columns and metadata for one table.",
		MinimalExample:  map[string]any{"profile_name": "analytics_db", "database_name": "analytics_db", "table_name": "orders"},
		AdvancedExample: map[string]any{"profile_name": "analytics_db", "database_name": "analytics_db", "table_name": "orders", "schema": "public"},
		CommonErrors: []ToolHelpError{
			{Error: "Table not found", Cause: "table_name does not exist in the specified database", Fix: "Use list-tables to see available tables"},
			{Error: "Schema not found", Cause: "schema does not exist (PostgreSQL)", Fix: "Omit schema to auto-detect, or use a valid schema name"},
		},
		ResponseFormat: "JSON object with columns array containing column metadata",
	},
	"list-databases": {
		Description: "List all databases or schemas visible to a database profile. For PostgreSQL, lists schemas; for MySQL/MariaDB, lists databases; for SQLite, lists attached databases.",
		Parameters: []ToolParamInfo{
			{Name: "profile_name", Type: "string", Required: true, Description: "Database profile to use"},
		},
		Summary:         "List databases/schemas visible to a profile.",
		MinimalExample:  map[string]any{"profile_name": "analytics_db"},
		AdvancedExample: map[string]any{"profile_name": "analytics_db"},
		CommonErrors: []ToolHelpError{
			{Error: "Profile not found", Cause: "profile_name does not match a configured profile", Fix: "Use list-profiles to see available profiles"},
		},
		ResponseFormat: "JSON object with databases array of database name strings",
	},
	"analyze-schema": {
		Description: "Analyze database schema metadata at selectable depth. Returns table catalogs, column details, relationships, data quality metrics, and AI query suggestions based on analysis level.",
		Parameters: []ToolParamInfo{
			{Name: "profile_name", Type: "string", Required: true, Description: "Database profile to use"},
			{Name: "database_name", Type: "string", Required: false, Description: "Database name (uses profile default if empty)"},
			{Name: "analysis_level", Type: "string", Required: true, Description: "Depth of analysis to perform", Enum: []string{"basic", "detailed", "comprehensive"}},
			{Name: "include_tables", Type: "array", Required: false, Description: "Whitelist of table names to include in analysis"},
			{Name: "exclude_tables", Type: "array", Required: false, Description: "Blacklist of table names to exclude from analysis"},
			{Name: "include_queries", Type: "boolean", Required: false, Description: "Generate AI query suggestions (default: true)", Default: "true"},
		},
		Summary:         "Analyze schema metadata with selectable analysis depth.",
		MinimalExample:  map[string]any{"profile_name": "analytics_db", "analysis_level": "basic"},
		AdvancedExample: map[string]any{"profile_name": "analytics_db", "database_name": "analytics_db", "analysis_level": "comprehensive", "include_tables": []any{"orders", "customers"}, "include_queries": true},
		CommonErrors: []ToolHelpError{
			{Error: "Invalid analysis_level", Cause: "analysis_level is not one of basic, detailed, comprehensive", Fix: "Use one of: basic, detailed, comprehensive"},
			{Error: "Profile not found", Cause: "profile_name does not match a configured profile", Fix: "Use list-profiles to see available profiles"},
		},
		ResponseFormat: "JSON object with analysis_metadata, database_overview, table_catalog, and varying detail by analysis_level",
	},
	"smart-query-builder": {
		Description: "Generate SQL queries from natural language intent. Analyzes available tables and columns to build contextually appropriate queries based on your description.",
		Parameters: []ToolParamInfo{
			{Name: "profile_name", Type: "string", Required: true, Description: "Database profile to use"},
			{Name: "intent", Type: "string", Required: true, Description: "Natural language description of what you want to query"},
			{Name: "database_name", Type: "string", Required: false, Description: "Database name (uses profile default if empty)"},
			{Name: "table_names", Type: "array", Required: false, Description: "Specific tables to consider for query generation"},
		},
		Summary:         "Generate SQL from natural-language intent.",
		MinimalExample:  map[string]any{"profile_name": "analytics_db", "intent": "monthly sales"},
		AdvancedExample: map[string]any{"profile_name": "analytics_db", "database_name": "analytics_db", "intent": "top 10 customers by revenue last quarter", "table_names": []any{"orders", "customers"}},
		CommonErrors: []ToolHelpError{
			{Error: "No tables found", Cause: "Database has no accessible tables", Fix: "Verify database_name and profile connectivity"},
			{Error: "No matching tables", Cause: "No tables match the specified intent", Fix: "Try a different intent or specify table_names explicitly"},
		},
		ResponseFormat: "JSON object with sql and explanation fields",
	},
	"optimize-query": {
		Description: "Run EXPLAIN-based optimization analysis on a SQL query. Returns execution plan, performance findings, cost estimation, and optimization suggestions.",
		Parameters: []ToolParamInfo{
			{Name: "profile_name", Type: "string", Required: true, Description: "Database profile to use"},
			{Name: "database_name", Type: "string", Required: true, Description: "Database name to analyze the query on"},
			{Name: "sql", Type: "string", Required: true, Description: "SQL query to analyze"},
			{Name: "params", Type: "array", Required: false, Description: "Positional parameters for parameterized queries"},
		},
		Summary:         "Run EXPLAIN-based optimization analysis.",
		MinimalExample:  map[string]any{"profile_name": "analytics_db", "database_name": "analytics_db", "sql": "SELECT * FROM orders WHERE customer_id = 123"},
		AdvancedExample: map[string]any{"profile_name": "analytics_db", "database_name": "analytics_db", "sql": "SELECT * FROM orders WHERE customer_id = ?", "params": []any{123}},
		CommonErrors: []ToolHelpError{
			{Error: "Missing required parameters", Cause: "profile_name/database_name/sql omitted", Fix: "Provide all required fields"},
			{Error: "SQL syntax error", Cause: "The provided SQL is invalid", Fix: "Check SQL syntax and try again"},
		},
		ResponseFormat: "JSON object with plan, findings, estimation, and summary",
	},
	"validate-query": {
		Description: "Validate SQL syntax and detect risky patterns without executing the query. Checks for SQL injection risks, destructive operations, and common syntax errors.",
		Parameters: []ToolParamInfo{
			{Name: "profile_name", Type: "string", Required: true, Description: "Database profile to use for dialect validation"},
			{Name: "sql", Type: "string", Required: true, Description: "SQL query to validate"},
			{Name: "database_name", Type: "string", Required: false, Description: "Database name for context-aware validation"},
			{Name: "params", Type: "array", Required: false, Description: "Positional parameters for parameterized queries"},
		},
		Summary:         "Validate SQL syntax and risky patterns without execution.",
		MinimalExample:  map[string]any{"profile_name": "analytics_db", "sql": "SELECT * FROM users"},
		AdvancedExample: map[string]any{"profile_name": "analytics_db", "database_name": "analytics_db", "sql": "SELECT * FROM users WHERE id = ?", "params": []any{42}},
		CommonErrors: []ToolHelpError{
			{Error: "Missing required parameters", Cause: "profile_name or sql omitted", Fix: "Provide both required fields"},
			{Error: "SQL syntax error", Cause: "The SQL contains syntax errors", Fix: "Review the validation issues in the response for details"},
		},
		ResponseFormat: "JSON object with is_valid, issues, and summary",
	},
	"analyze-data-lineage": {
		Description: "Trace upstream and downstream table relationships using foreign key analysis. Shows how data flows between tables through foreign key relationships.",
		Parameters: []ToolParamInfo{
			{Name: "profile_name", Type: "string", Required: true, Description: "Database profile to use"},
			{Name: "table_name", Type: "string", Required: true, Description: "Table to trace lineage for"},
			{Name: "scope", Type: "string", Required: false, Description: "Direction of lineage analysis", Enum: []string{"upstream", "downstream", "both"}, Default: "both"},
			{Name: "database_name", Type: "string", Required: false, Description: "Database name (uses profile default if empty)"},
			{Name: "tables", Type: "array", Required: false, Description: "Specific tables to scope lineage analysis"},
		},
		Summary:         "Trace upstream/downstream relationships for a table.",
		MinimalExample:  map[string]any{"profile_name": "analytics_db", "table_name": "orders", "scope": "both"},
		AdvancedExample: map[string]any{"profile_name": "analytics_db", "database_name": "analytics_db", "table_name": "orders", "scope": "upstream", "tables": []any{"orders", "customers", "products"}},
		CommonErrors: []ToolHelpError{
			{Error: "Missing required parameters", Cause: "profile_name or table_name omitted", Fix: "Provide both required fields"},
			{Error: "Table not found", Cause: "table_name does not exist in the database", Fix: "Use list-tables to see available tables"},
		},
		ResponseFormat: "JSON object with upstream, downstream arrays, edges, summary, and scope",
	},
	"discover-insights": {
		Description: "Automatically discover KPIs, trends, anomalies, and distribution patterns in database tables. Analyzes column statistics to generate actionable insights.",
		Parameters: []ToolParamInfo{
			{Name: "profile_name", Type: "string", Required: true, Description: "Database profile to use"},
			{Name: "table_name", Type: "string", Required: true, Description: "Table to analyze for insights"},
			{Name: "columns", Type: "array", Required: false, Description: "Specific columns to analyze (analyzes all columns if empty)"},
			{Name: "insight_types", Type: "array", Required: false, Description: "Types of insights to discover", Enum: []string{"kpi", "trend", "anomaly", "distribution"}},
			{Name: "max_results", Type: "integer", Required: false, Description: "Maximum number of insights to return per type"},
		},
		Summary:         "Discover KPI, trend, anomaly, and distribution insights.",
		MinimalExample:  map[string]any{"profile_name": "analytics_db", "table_name": "orders"},
		AdvancedExample: map[string]any{"profile_name": "analytics_db", "table_name": "orders", "columns": []any{"revenue", "order_date"}, "insight_types": []any{"kpi", "trend"}, "max_results": 10},
		CommonErrors: []ToolHelpError{
			{Error: "Missing required parameters", Cause: "profile_name or table_name omitted", Fix: "Provide both required fields"},
			{Error: "Table not found", Cause: "table_name does not exist", Fix: "Use list-tables to see available tables"},
			{Error: "No numeric columns", Cause: "Table has no numeric columns for insight analysis", Fix: "Choose a table with numeric data columns"},
		},
		ResponseFormat: "JSON object with list of insights containing type, column, description, and metrics",
	},
	"track-schema-changes": {
		Description: "Track schema evolution with snapshots, history queries, drift detection, and migration generation. Supports multiple operations for comprehensive schema change management.",
		Parameters: []ToolParamInfo{
			{Name: "profile_name", Type: "string", Required: true, Description: "Database profile to use"},
			{Name: "operation", Type: "string", Required: false, Description: "Operation to perform", Enum: []string{"track", "snapshot", "diff", "history", "detect_drift"}, Default: "track"},
			{Name: "database_name", Type: "string", Required: false, Description: "Database name (uses profile default if empty)"},
			{Name: "from_snapshot_id", Type: "string", Required: false, Description: "Starting snapshot ID for diff/migration"},
			{Name: "to_snapshot_id", Type: "string", Required: false, Description: "Ending snapshot ID for diff/migration"},
			{Name: "change_types", Type: "array", Required: false, Description: "Filter changes by type", Enum: []string{"create", "alter", "drop"}},
			{Name: "limit", Type: "integer", Required: false, Description: "Maximum number of results to return"},
		},
		Summary:         "Track schema snapshots and detect drift/migrations.",
		MinimalExample:  map[string]any{"profile_name": "analytics_db", "operation": "track"},
		AdvancedExample: map[string]any{"profile_name": "analytics_db", "operation": "diff", "from_snapshot_id": "2026-04-01T00:00:00Z", "to_snapshot_id": "2026-04-15T00:00:00Z"},
		CommonErrors: []ToolHelpError{
			{Error: "Snapshot not found", Cause: "from_snapshot_id or to_snapshot_id does not exist", Fix: "Use operation=history to list available snapshots"},
			{Error: "Invalid operation", Cause: "operation is not a valid value", Fix: "Use one of: track, snapshot, diff, history, detect_drift"},
		},
		ResponseFormat: "JSON object varying by operation: snapshot details, changes array, history, or drift report",
	},
	"federated-query": {
		Description: "Execute read-only cross-profile queries with optional joins and aggregations. Query multiple database profiles and combine results through join conditions and aggregations.",
		Parameters: []ToolParamInfo{
			{Name: "sql", Type: "string", Required: false, Description: "SQL query to execute across profiles"},
			{Name: "sub_queries", Type: "array", Required: false, Description: "Array of sub-query objects with profile, sql, and alias fields"},
			{Name: "joins", Type: "array", Required: false, Description: "Join conditions between sub-query results"},
			{Name: "aggregations", Type: "array", Required: false, Description: "Post-processing aggregations (COUNT, SUM, AVG, etc.)"},
			{Name: "limit", Type: "integer", Required: false, Description: "Maximum number of result rows"},
			{Name: "offset", Type: "integer", Required: false, Description: "Number of rows to skip for pagination"},
		},
		Summary:         "Execute read-only cross-profile queries with optional joins.",
		MinimalExample:  map[string]any{"sub_queries": []any{map[string]any{"profile": "analytics_db", "sql": "SELECT 1", "alias": "q1"}}},
		AdvancedExample: map[string]any{"sub_queries": []any{map[string]any{"profile": "analytics_db", "sql": "SELECT customer_id, SUM(amount) AS total FROM orders GROUP BY customer_id", "alias": "q1"}}, "joins": []any{map[string]any{"left": "q1.customer_id", "right": "q2.id", "type": "INNER"}}, "limit": 100},
		CommonErrors: []ToolHelpError{
			{Error: "No sub-queries provided", Cause: "sub_queries and sql are both empty", Fix: "Provide at least one sub_query or a sql string"},
			{Error: "Profile not found in sub_query", Cause: "A sub-query references a non-existent profile", Fix: "Use list-profiles to see available profiles"},
			{Error: "Join alias mismatch", Cause: "Join references an alias not defined in sub_queries", Fix: "Ensure join aliases match sub_query alias fields"},
		},
		ResponseFormat: "JSON object with columns, rows, and metadata",
	},
	"discover-joins": {
		Description: "Suggest join relationships between tables based on foreign key constraints and column name matching. Helps understand how tables relate to each other.",
		Parameters: []ToolParamInfo{
			{Name: "profile_name", Type: "string", Required: true, Description: "Database profile to use"},
			{Name: "tables", Type: "array", Required: false, Description: "Specific tables to analyze for join relationships"},
		},
		Summary:         "Suggest joins based on foreign key relationships.",
		MinimalExample:  map[string]any{"profile_name": "analytics_db"},
		AdvancedExample: map[string]any{"profile_name": "analytics_db", "tables": []any{"orders", "customers", "products"}},
		CommonErrors: []ToolHelpError{
			{Error: "Profile not found", Cause: "profile_name does not match a configured profile", Fix: "Use list-profiles to see available profiles"},
			{Error: "No joins found", Cause: "Tables have no foreign key relationships", Fix: "Try including more tables or check if FK constraints exist"},
		},
		ResponseFormat: "JSON object with joins array and summary",
	},
	"sample-data": {
		Description: "Fetch sample rows from a table to understand data shape, column types, and content. Useful for exploring unfamiliar datasets and validating data quality.",
		Parameters: []ToolParamInfo{
			{Name: "profile_name", Type: "string", Required: true, Description: "Database profile to use"},
			{Name: "database_name", Type: "string", Required: true, Description: "Database name containing the table"},
			{Name: "table_name", Type: "string", Required: true, Description: "Table to sample data from"},
			{Name: "schema", Type: "string", Required: false, Description: "Schema name (PostgreSQL only)"},
			{Name: "sample_size", Type: "integer", Required: false, Description: "Number of rows to return", Default: "5"},
		},
		Summary:         "Fetch sample rows from a table.",
		MinimalExample:  map[string]any{"profile_name": "analytics_db", "database_name": "analytics_db", "table_name": "users"},
		AdvancedExample: map[string]any{"profile_name": "analytics_db", "database_name": "analytics_db", "table_name": "users", "schema": "public", "sample_size": 10},
		CommonErrors: []ToolHelpError{
			{Error: "Missing required parameters", Cause: "profile_name/database_name/table_name omitted", Fix: "Provide all required fields"},
			{Error: "Table not found", Cause: "table_name does not exist in the specified database", Fix: "Use list-tables to see available tables"},
		},
		ResponseFormat: "JSON object with table_name, sample_size, columns, sample_rows, and summary",
	},
	"mcp-info": {
		Description:     "Return MCP provider metadata including server version, author, and capabilities. No parameters required.",
		Parameters:      []ToolParamInfo{},
		Summary:         "Return MCP provider metadata.",
		MinimalExample:  map[string]any{"_note": "No parameters required"},
		AdvancedExample: map[string]any{"_note": "No additional parameters available"},
		CommonErrors: []ToolHelpError{
			{Error: "No errors possible", Cause: "This tool requires no parameters and always succeeds", Fix: "No action needed"},
		},
		ResponseFormat: "JSON object with server version, author, and capabilities",
	},
	"list-tools": {
		Description:     "Return the catalog of all available tools with their names and descriptions. No parameters required.",
		Parameters:      []ToolParamInfo{},
		Summary:         "Return tool catalog and descriptions.",
		MinimalExample:  map[string]any{"_note": "No parameters required"},
		AdvancedExample: map[string]any{"_note": "No additional parameters available"},
		CommonErrors: []ToolHelpError{
			{Error: "No errors possible", Cause: "This tool requires no parameters and always succeeds", Fix: "No action needed"},
		},
		ResponseFormat: "JSON object with tools array of {name, description} objects",
	},
	"get-tool-help": {
		Description: "Return documentation, examples, parameter details, and troubleshooting for any registered tool. Use topic to filter specific help sections.",
		Parameters: []ToolParamInfo{
			{Name: "tool_name", Type: "string", Required: true, Description: "Name of the tool to get help for"},
			{Name: "topic", Type: "string", Required: false, Description: "Specific help topic to retrieve", Enum: []string{"summary", "minimal_example", "advanced_example", "errors", "all"}, Default: "all"},
		},
		Summary:         "Return examples and troubleshooting for a tool.",
		MinimalExample:  map[string]any{"tool_name": "execute-sql", "topic": "all"},
		AdvancedExample: map[string]any{"tool_name": "analyze-schema", "topic": "advanced_example"},
		CommonErrors: []ToolHelpError{
			{Error: "Invalid topic", Cause: "topic is not one of the supported values", Fix: "Use one of: summary, minimal_example, advanced_example, errors, all"},
			{Error: "Tool not found", Cause: "tool_name does not match a registered tool", Fix: "Use list-tools to see available tool names"},
		},
		ResponseFormat: "JSON object with tool_name, found, description, parameters, examples, and errors",
	},
	"get-search-path": {
		Description: "Get the current search_path and effective schema for PostgreSQL databases. Shows which schemas are searched and which schema is active.",
		Parameters: []ToolParamInfo{
			{Name: "profile_name", Type: "string", Required: true, Description: "PostgreSQL profile to use"},
			{Name: "database_name", Type: "string", Required: true, Description: "Database name to check search path on"},
		},
		Summary:         "Get the current search_path and effective schema for PostgreSQL.",
		MinimalExample:  map[string]any{"profile_name": "analytics_db", "database_name": "analytics_db"},
		AdvancedExample: map[string]any{"profile_name": "analytics_db", "database_name": "analytics_db"},
		CommonErrors: []ToolHelpError{
			{Error: "Profile not found", Cause: "profile_name does not match a configured profile", Fix: "Use list-profiles to see available profiles"},
			{Error: "Not a PostgreSQL profile", Cause: "get-search-path only works with PostgreSQL profiles", Fix: "Use a PostgreSQL profile for this operation"},
		},
		ResponseFormat: "JSON object with search_path, current_schema, and optional connection_pooling_warning",
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
	result := GetToolHelpResult{
		ToolName:    toolName,
		Found:       true,
		Description: entry.Description,
		Parameters:  entry.Parameters,
		Topics:      supportedToolHelpTopics,
	}
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
		result.ResponseFormat = entry.ResponseFormat
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
