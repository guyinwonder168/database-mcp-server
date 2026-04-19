// server.go
// Author: guyinwonder
// Version: v1.3.0
// Project created using OpenAI GPT-4.1 via VSCode Kilocode AI code assistant extension.
// MCP server implementation for the database-mcp-provider project.
// Provides MCP actions for profile management, SQL execution, table/DB listing, and uses structured JSON logging.

package mcp

import (
	"context"
	"crypto/rand"
	"database-mcp-provider/internal/config"
	"database-mcp-provider/internal/db"
	"database-mcp-provider/internal/log"
	"database-mcp-provider/internal/mcp/analyze"
	ctxmgr "database-mcp-provider/internal/mcp/context"
	"database-mcp-provider/internal/mcp/lineage"
	"database-mcp-provider/internal/mcp/nlp"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCPServer represents the MCP server and its registered tools.
type MCPServer struct {
	server        *mcp.Server
	ConfigPath    string // <--- Add this field for testability
	errorAnalyzer *ErrorAnalyzer
	toolsRegistry []ToolInfo
	toolDecls     []ToolDeclarationInfo
	contextMgr    *ctxmgr.Manager
}

const MCPVersion = "v1.5.1"
const MCPAuthor = "guyinwonder"

const sqliteListTablesQuery = "SELECT name FROM sqlite_master WHERE type='table'"
const (
	joinSQLTemplate                     = "SELECT * FROM %s JOIN %s ON %s.%s = %s.%s"
	toolConfigureProfile                = "configure-profile"
	toolDiscoverJoins                   = "discover-joins"
	toolGetToolHelp                     = "get-tool-help"
	toolListProfiles                    = "list-profiles"
	toolListSchemas                     = "list-schemas"
	mimeTypeApplicationJSON             = "application/json"
	messageMissingRequiredParameters    = "Missing required parameters"
	actionProvideAllRequiredParameters  = "Provide all required parameters"
	messageCreateProfileConnectionInfo  = "Create the profile with database connection info"
	messageProfileNotFound              = "Profile not found"
	errorProfileNotFound                = "profile not found"
	messageFailedToConnectToDatabase    = "Failed to connect to database"
	queryShowFullTables                 = "SHOW FULL TABLES"
	queryMysqlTableInfoList             = "SELECT TABLE_SCHEMA, TABLE_NAME, TABLE_TYPE FROM information_schema.TABLES WHERE TABLE_SCHEMA = DATABASE() ORDER BY TABLE_NAME"
	queryPostgresTableInfoList          = "SELECT table_schema, table_name, table_type FROM information_schema.tables WHERE table_schema NOT IN ('pg_catalog', 'information_schema') AND table_schema NOT LIKE 'pg_%' ORDER BY table_schema, table_name"
	querySqliteTableInfoList            = "SELECT '' AS schema, name, type FROM sqlite_master WHERE type IN ('table', 'view') ORDER BY name"
	queryPostgresPublicInformationTable = "SELECT table_schema, table_name FROM information_schema.tables WHERE table_schema NOT IN ('pg_catalog', 'information_schema') AND table_schema NOT LIKE 'pg_%' ORDER BY table_schema, table_name"
	errorUnsupportedDBType              = "unsupported db_type"
	messageFailedToListTables           = "Failed to list tables"
	messageFailedToLoadConfiguration    = "Failed to load configuration"
	actionInitializeConfiguration       = "Initialize configuration"
	descriptionRunServerCreateConfig    = "Run the server to create initial configuration"
	messageProfileNotFoundFormat        = "Profile '%s' not found"
	descriptionSpecifiedProfileMissing  = "The specified database profile does not exist"
)

func paramsValueSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		OneOf: []*jsonschema.Schema{
			{Type: "string"},
			{Type: "number"},
			{Type: "boolean"},
			{Type: "null"},
		},
	}
}

func paramsArraySchema(description string) *jsonschema.Schema {
	return &jsonschema.Schema{
		Type:        "array",
		Description: description,
		Items:       paramsValueSchema(),
	}
}

func inputSchemaWithParams[T any](paramsDescription string) *jsonschema.Schema {
	schema, err := jsonschema.ForType(reflect.TypeFor[T](), &jsonschema.ForOptions{})
	if err != nil {
		panic(fmt.Sprintf("failed to infer schema for tool input: %v", err))
	}
	if schema.Properties == nil {
		schema.Properties = map[string]*jsonschema.Schema{}
	}
	schema.Properties["params"] = paramsArraySchema(paramsDescription)
	return sanitizeSchemaForGemini(schema)
}

func inputSchemaFor[T any]() *jsonschema.Schema {
	schema, err := jsonschema.ForType(reflect.TypeFor[T](), &jsonschema.ForOptions{})
	if err != nil {
		panic(fmt.Sprintf("failed to infer schema for tool input: %v", err))
	}
	return sanitizeSchemaForGemini(schema)
}

func analyzeSchemaInputSchema() *jsonschema.Schema {
	return sanitizeSchemaForGemini(&jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"profile_name":    {Type: "string"},
			"analysis_level":  {Type: "string"},
			"database_name":   {Type: "string"},
			"sample_size":     {Type: "integer"},
			"include_queries": {Type: "boolean"},
			"profiling":       {Type: "boolean"},
		},
		Required: []string{"profile_name", "analysis_level"},
	})
}

func sanitizeSchemaForGemini(schema *jsonschema.Schema) *jsonschema.Schema {
	if schema == nil {
		return nil
	}

	raw, err := json.Marshal(schema)
	if err != nil {
		return schema
	}

	var node any
	err = json.Unmarshal(raw, &node)
	if err != nil {
		return schema
	}

	sanitizeSchemaNodeForGemini(node)

	sanitizedRaw, err := json.Marshal(node)
	if err != nil {
		return schema
	}

	var sanitized jsonschema.Schema
	err = json.Unmarshal(sanitizedRaw, &sanitized)
	if err != nil {
		return schema
	}

	return &sanitized
}

// sanitizeTypeArray converts ["null","array"] to "array" and removes nullable.
func sanitizeTypeArray(n map[string]any) {
	typeValue, ok := n["type"]
	if !ok {
		return
	}
	typeList, ok := typeValue.([]any)
	if !ok {
		return
	}
	nonNullType := ""
	for _, item := range typeList {
		typeName, ok := item.(string)
		if !ok || typeName == "null" {
			continue
		}
		if nonNullType == "" {
			nonNullType = typeName
		}
	}
	if nonNullType != "" {
		n["type"] = nonNullType
		delete(n, "nullable")
	}
}

// sanitizeAdditionalProperties removes additionalProperties:false.
func sanitizeAdditionalProperties(n map[string]any) {
	if val, ok := n["additionalProperties"]; ok {
		if b, ok := val.(bool); ok && !b {
			delete(n, "additionalProperties")
		}
	}
}

// sanitizeItems fixes boolean items:true to object schema.
func sanitizeItems(n map[string]any) {
	if val, ok := n["items"]; ok {
		if b, ok := val.(bool); ok {
			if b {
				n["items"] = map[string]any{"type": "string"}
			} else {
				delete(n, "items")
			}
		}
	}
}

func sanitizeSchemaNodeForGemini(node any) {
	switch n := node.(type) {
	case map[string]any:
		sanitizeTypeArray(n)
		sanitizeAdditionalProperties(n)
		sanitizeItems(n)
		for _, child := range n {
			sanitizeSchemaNodeForGemini(child)
		}
	case []any:
		for _, child := range n {
			sanitizeSchemaNodeForGemini(child)
		}
	}
}

// buildRelationshipGraph creates a graph representation for AI consumption.
func (s *MCPServer) buildRelationshipGraph(relGraph RelationshipGraph) map[string]interface{} {
	graph := map[string]interface{}{}
	nodes := map[string]map[string]interface{}{}
	edges := []map[string]interface{}{}

	// Add nodes from FK and semantic relationships
	for _, fk := range relGraph.ForeignKeys {
		if _, ok := nodes[fk.FromTable]; !ok {
			nodes[fk.FromTable] = map[string]interface{}{"name": fk.FromTable}
		}
		if _, ok := nodes[fk.ToTable]; !ok {
			nodes[fk.ToTable] = map[string]interface{}{"name": fk.ToTable}
		}
		edges = append(edges, map[string]interface{}{
			"from":           fk.FromTable,
			"to":             fk.ToTable,
			"source_column":  fk.FromColumn,
			"target_column":  fk.ToColumn,
			"type":           fk.RelationshipType,
			"suggested_join": fk.SuggestedJoin,
		})
	}
	for _, rel := range relGraph.SemanticRelationships {
		for _, t := range rel.Tables {
			if _, ok := nodes[t]; !ok {
				nodes[t] = map[string]interface{}{"name": t}
			}
		}
		edges = append(edges, map[string]interface{}{
			"tables":           rel.Tables,
			"type":             rel.RelationshipType,
			"basis":            rel.ConnectionBasis,
			"confidence_score": rel.ConfidenceScore,
			"suggested_join":   rel.SuggestedJoin,
		})
	}
	graph["nodes"] = nodes
	graph["edges"] = edges
	graph["suggested_joins"] = relGraph.SuggestedJoins
	return graph
}

// NewMCPServer creates a new MCPServer instance using the MCP SDK.
func NewMCPServer() *MCPServer {
	return NewMCPServerWithConfig("config.yaml")
}

// NewMCPServerWithConfig allows specifying a config path (for testing).
func NewMCPServerWithConfig(configPath string) *MCPServer {
	srv := mcp.NewServer(&mcp.Implementation{Name: "database-mcp-provider", Version: MCPVersion}, nil)
	mcpServer := &MCPServer{
		server:        srv,
		ConfigPath:    configPath,
		errorAnalyzer: NewErrorAnalyzer(configPath),
		contextMgr:    ctxmgr.NewManager(30*time.Minute, 20),
	}
	mcpServer.registerAllTools()
	mcpServer.registerResources()
	return mcpServer
}

func (s *MCPServer) toolDescriptionFormatter() func(string) string {
	if s.schemaMode() == config.SchemaModeStandard {
		return strings.TrimSpace
	}
	return compactToolDescription
}

func (s *MCPServer) schemaMode() config.SchemaMode {
	cfg, err := config.LoadConfig(s.ConfigPath)
	if err != nil {
		return config.SchemaModeCompact
	}
	return cfg.SchemaMode
}

func compactToolDescription(description string) string {
	compact := strings.TrimSpace(description)
	if idx := strings.Index(compact, "Example:"); idx >= 0 {
		compact = compact[:idx]
	}
	compact = strings.Join(strings.Fields(compact), " ")
	const maxCompactDescriptionLength = 160
	if len(compact) <= maxCompactDescriptionLength {
		return compact
	}
	trimmed := compact[:maxCompactDescriptionLength]
	if cut := strings.LastIndex(trimmed, " "); cut > 100 {
		trimmed = trimmed[:cut]
	}
	return strings.TrimSpace(trimmed)
}

// registerAllTools registers all MCP tools and populates toolsRegistry.
// This is called in Start() and in tests.
func (s *MCPServer) registerAllTools() {
	// Prevent duplicate registration
	if len(s.toolsRegistry) > 0 {
		return
	}
	descriptionFormatter := s.toolDescriptionFormatter()

	// configure-profile
	{
		tool := &mcp.Tool{
			Name: toolConfigureProfile,
			Description: descriptionFormatter(`Create, update, delete, or clone a database connection profile. Required for all database actions.
		Fields:
		  profile_name (required)
		  db_type (mysql|mariadb|postgres|sqlite)
		  host / port / username / password (required except sqlite)
		  database_name
		  readonly (boolean)
		  sslmode (Postgres only, optional: disable|require|verify-ca|verify-full; defaults to require)
		  action (optional: "delete" or "clone"; omit for create/update)
		  source_profile (required for clone: name of profile to copy from)
		Examples:
		  Create/update: {"profile_name":"mydb","db_type":"postgres","host":"localhost","port":5432,"username":"app","password":"secret","database_name":"appdb","readonly":false}
		  Delete: {"action":"delete","profile_name":"mydb"}
		  Clone: {"action":"clone","profile_name":"mydb-readonly","source_profile":"mydb","readonly":true}`),
			InputSchema: inputSchemaFor[ConfigureProfileParams](),
		}
		addTool(s, tool, s.handleConfigureProfile)
	}

	// list-profiles
	{
		tool := &mcp.Tool{
			Name: toolListProfiles,
			Description: descriptionFormatter(`List all configured database profiles.
  Example:
  {}`),
			InputSchema: inputSchemaFor[ListProfilesParams](),
		}
		addTool(s, tool, s.handleListProfiles)
	}

	// execute-sql
	{
		tool := &mcp.Tool{
			Name: "execute-sql",
			Description: descriptionFormatter(`Execute an arbitrary SQL query or statement. Both 'profile_name' and 'database_name' are required.
		Note: For cross-database queries or describing tables in another database, use fully qualified table names (e.g., db.table).
		Example:
		{"profile_name":"some-profile-name","database_name":"some-database-name","sql":"SELECT * FROM some-table-name WHERE some-field-name=34;"}
		{"profile_name":"some-profile-name","sql":"DESCRIBE some-database-name.some-table-name"}`),
			InputSchema: inputSchemaWithParams[ExecuteSQLParams]("positional parameters for prepared statements; BLOB/BINARY values must be base64-encoded strings"),
		}
		addTool(s, tool, s.handleExecuteSQL)
	}

	// list-tables
	{
		tool := &mcp.Tool{
			Name: "list-tables",
			Description: descriptionFormatter(`List all tables in the selected database. Both 'profile_name' and 'database_name' are required.
		Example:
		{"profile_name":"some-profile-name","database_name":"some-database-name"}`),
			InputSchema: inputSchemaFor[ListTablesParams](),
		}
		addTool(s, tool, s.handleListTables)
	}

	// describe-table
	{
		tool := &mcp.Tool{
			Name: "describe-table",
			Description: descriptionFormatter(`Describe the comprehensive schema of a table including columns, types, constraints, comments, and metadata.
  Returns: column names, data types, nullable status, key constraints, default values, column comments, character sets, collation, auto-increment status, max length, precision, and scale.
  Example:
  {"profile_name":"some-profile-name","database_name":"some-database-name","table_name":"some-table-name"}`),
			InputSchema: inputSchemaFor[DescribeTableParams](),
		}
		addTool(s, tool, s.handleDescribeTable)
	}

	// list-databases
	{
		tool := &mcp.Tool{
			Name: "list-databases",
			Description: descriptionFormatter(`List all databases/schemas available to the profile.
  Example:
  {"profile_name":"some-profile-name"}`),
			InputSchema: inputSchemaFor[ListDatabasesParams](),
		}
		addTool(s, tool, s.handleListDatabases)
	}

	// list-schemas
	{
		tool := &mcp.Tool{
			Name: toolListSchemas,
			Description: descriptionFormatter(`List all accessible database schemas with default schema information.
  Input: profile_name (required), database_name (required).
  Returns: list of schema names and the default schema for the database.
  Example:
  {"profile_name":"some-profile-name","database_name":"some-database-name"}`),
			InputSchema: inputSchemaFor[ListSchemasParams](),
		}
		addTool(s, tool, s.handleListSchemas)
	}

	// get-search-path
	{
		tool := &mcp.Tool{
			Name: "get-search-path",
			Description: descriptionFormatter(`Get the current search_path and effective schema (read-only diagnostic).
   
   This tool queries the database to show the current search_path setting and the active schema.
   Note: This server uses connection pooling. Schema should be explicitly qualified in queries.
   
   Input: profile_name (required), database_name (required)
   Returns: search_path, current_schema, and optional connection pooling warning
   Example:
   {"profile_name":"some-profile-name","database_name":"some-database-name"}`),
			InputSchema: inputSchemaFor[GetSearchPathParams](),
		}
		addTool(s, tool, s.handleGetSearchPath)
	}

	// analyze-schema
	{
		tool := &mcp.Tool{
			Name: "analyze-schema",
			Description: descriptionFormatter(`Perform schema analysis for a database, including table/column metadata, relationships, and sample data integration.
  
  Required parameters:
   - profile_name: Database profile to analyze
   - analysis_level: REQUIRED. Must be one of "basic", "detailed", "comprehensive".
     - BASIC: Quick overview for initial exploration
     - DETAILED: Comprehensive schema for query construction
     - COMPREHENSIVE: Deep business context with AI insights
  
  Optional parameters:
   - database_name: Specific database (uses profile default if empty)
   - include_tables: Specific tables to analyze (all if empty)
   - exclude_tables: Tables to exclude from analysis
   - sample_size: Rows to sample per table (default: 10)
   - include_queries: Generate query suggestions (default: true)
   - profiling: Enable advanced statistical and pattern profiling (default: false)
  
  IMPORTANT: This tool provides raw analysis signals, not authoritative classifications.
  The domain_indicators field contains naming prefix frequencies (e.g., {"order": 3, "product": 2}),
  not definitive domain labels. Use your judgment to interpret domain indicators, table categories,
  and performance signals based on the raw data provided.
  
  AI agents MUST specify analysis_level. Example:
  {"profile_name":"analytics_db","analysis_level":"detailed","database_name":"analytics_db"}`),
			InputSchema: analyzeSchemaInputSchema(),
		}
		addTool(s, tool, s.handleAnalyzeSchema)
	}

	// smart-query-builder
	{
		tool := &mcp.Tool{
			Name: "smart-query-builder",
			Description: descriptionFormatter(`Generate optimized SQL from high-level intent and schema analysis.
  Input: profile_name, intent (natural language), optional database_name/table_name(s).
  Returns: generated SQL, explanation, and any errors.
  Example:
  {"profile_name":"some-profile-name","intent":"attendance dashboard"}`),
			InputSchema: inputSchemaFor[SmartQueryBuilderParams](),
		}
		addTool(s, tool, s.handleSmartQueryBuilder)
	}

	// optimize-query
	{
		tool := &mcp.Tool{
			Name: "optimize-query",
			Description: descriptionFormatter(`Run EXPLAIN and return optimization findings for a SQL statement.
		Input: profile_name (required), database_name (required), sql (required), params (optional).
		Returns: execution plan, detected issues (missing indexes, inefficient joins), and estimated improvement range.
		Example:
		{"profile_name":"analytics_db","database_name":"analytics_db","sql":"SELECT * FROM orders WHERE customer_id = ?","params":[123]}`),
			InputSchema: inputSchemaWithParams[OptimizeQueryParams]("positional parameters for prepared statements; BLOB/BINARY values must be base64-encoded strings"),
		}
		addTool(s, tool, s.handleOptimizeQuery)
	}

	// validate-query
	{
		tool := &mcp.Tool{
			Name: "validate-query",
			Description: descriptionFormatter(`Validate SQL syntax and detect risky patterns without executing the statement.
		Input: profile_name (required), sql (required), database_name (optional), params (optional).
		Returns: validation issues (syntax, logic, security) and pass/fail summary.
		Example:
		{"profile_name":"analytics_db","sql":"SELECT * FROM users WHERE id = ?","params":[123]}`),
			InputSchema: inputSchemaWithParams[ValidateQueryParams]("positional parameters for prepared statements (metadata only); BLOB/BINARY values must be base64-encoded strings"),
		}
		addTool(s, tool, s.handleValidateQuery)
	}

	// analyze-data-lineage
	{
		tool := &mcp.Tool{
			Name: "analyze-data-lineage",
			Description: descriptionFormatter(`Trace data dependencies for a table using foreign key relationships.
  Input: profile_name (required), table_name (required), database_name (optional), scope (optional: upstream|downstream|both).
  Returns: upstream/downstream tables and dependency edges.
  Example:
  {"profile_name":"analytics_db","table_name":"orders","scope":"both"}`),
			InputSchema: inputSchemaFor[AnalyzeDataLineageParams](),
		}
		addTool(s, tool, s.handleAnalyzeDataLineage)
	}

	// discover-insights
	{
		tool := &mcp.Tool{
			Name: "discover-insights",
			Description: descriptionFormatter(`Automatically discovers KPIs, trends, anomalies, and distribution patterns in database tables.
  Input: profile_name (required), table_name (required), columns (optional), insight_types (optional: kpi, trend, anomaly, distribution), max_results (optional).
  Returns: list of insights with type, column, description, and detailed metrics.
  Example:
  {"profile_name":"analytics_db","table_name":"sales","insight_types":["kpi","trend"],"max_results":10}`),
			InputSchema: inputSchemaWithParams[DiscoverInsightsParams]("Optional query parameters for filtering insights"),
		}
		addTool(s, tool, s.handleDiscoverInsights)
	}

	// track-schema-changes
	{
		tool := &mcp.Tool{
			Name: "track-schema-changes",
			Description: descriptionFormatter(`Track schema evolution with snapshots, history, migration generation, and drift detection.
  Input: profile_name (required), operation (optional: track|history|generate_migration|detect_drift), database_name (optional), dialect (optional),
         from_snapshot_id/to_snapshot_id (optional for migration), snapshot_id (optional for drift), limit (optional), retention_days (optional).
  Returns: schema snapshots, detected changes, migration script/validation/impact, or drift report depending on operation.
  Example:
  {"profile_name":"analytics_db","operation":"track","retention_days":30}`),
			InputSchema: inputSchemaFor[TrackSchemaChangesParams](),
		}
		addTool(s, tool, s.handleTrackSchemaChanges)
	}

	// federated-query
	{
		tool := &mcp.Tool{
			Name: "federated-query",
			Description: descriptionFormatter(`Execute read-only SQL across multiple profiles with optional cross-profile JOINs and aggregations.
  Input: either sql (profile.table syntax) or explicit sub_queries, joins (optional), aggregations (optional), limit/offset (optional), max_concurrency (optional).
  Returns: combined rows plus execution metadata (execution_time_ms, rows_from_each, partial errors).
  Example:
  {"sub_queries":[{"profile":"crm_db","sql":"SELECT id,name FROM users","alias":"u"},{"profile":"analytics_db","sql":"SELECT user_id,total FROM orders","alias":"o"}],"joins":[{"left":"u.id","right":"o.user_id","type":"INNER"}],"limit":100}`),
			InputSchema: inputSchemaFor[FederatedQueryRequest](),
		}
		addTool(s, tool, s.handleFederatedQuery)
	}

	// discover-joins
	{
		tool := &mcp.Tool{
			Name: toolDiscoverJoins,
			Description: descriptionFormatter(`Discover joinable relationships (foreign keys) between tables and suggest JOIN SQL.
  Input: profile_name (required), tables (optional).
  Returns: list of join suggestions and summary.
  Example:
  {"profile_name":"analytics_db","tables":["orders","customers"]}`),
			InputSchema: inputSchemaFor[DiscoverJoinsParams](),
		}
		addTool(s, tool, s.handleDiscoverJoins)
	}

	// sample-data
	{
		tool := &mcp.Tool{
			Name: "sample-data",
			Description: descriptionFormatter(`Fetch sample rows from a table to help AI/agents infer data types, formats, and value ranges.
  Input: profile_name (required), database_name (required), table_name (required), sample_size (optional, default: 3).
  Returns: sample rows with column names and values.
  Example:
  {"profile_name":"analytics_db","database_name":"analytics_db","table_name":"users","sample_size":5}`),
			InputSchema: inputSchemaFor[SampleDataParams](),
		}
		addTool(s, tool, s.handleSampleData)
	}

	// mcp-info
	{
		tool := &mcp.Tool{
			Name: "mcp-info",
			Description: descriptionFormatter(`Show MCP provider version and author.
  Example:
  {}`),
			InputSchema: inputSchemaFor[MCPInfoParams](),
		}
		addTool(s, tool, s.handleMCPInfo)
	}

	// list-tools
	{
		tool := &mcp.Tool{
			Name: "list-tools",
			Description: descriptionFormatter(`List all available MCP tools and their descriptions.
  Example:
  {}`),
			InputSchema: inputSchemaFor[ListToolsParams](),
		}
		addTool(s, tool, s.handleListTools)
	}

	// get-tool-help
	{
		tool := &mcp.Tool{
			Name: toolGetToolHelp,
			Description: descriptionFormatter(`Get usage help, examples, and common errors for a specific tool.
  Example:
  {"tool_name":"execute-sql","topic":"all"}`),
			InputSchema: inputSchemaFor[GetToolHelpParams](),
		}
		addTool(s, tool, s.handleGetToolHelp)
	}
}

// registerResources registers non-tool MCP resources and templates for discovery.
func (s *MCPServer) registerResources() {
	// Static tools snapshot as a resource
	s.server.AddResource(&mcp.Resource{
		URI:         "tools://list",
		Name:        "Registered MCP Tools",
		Description: "All registered MCP tools with descriptions",
		MIMEType:    mimeTypeApplicationJSON,
	}, s.resourceToolsHandler)

	// Profile metadata (secrets redacted) via URI template
	s.server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "profile://{profile}",
		Name:        "Database profile metadata",
		Description: "Profile connection metadata without secrets",
		MIMEType:    mimeTypeApplicationJSON,
	}, s.resourceProfileHandler)
}

// Start launches the MCP server, registers all tools, and starts listening for MCP requests.
func (s *MCPServer) Start() error {
	// Ensure tools are registered (constructor normally does this)
	if len(s.toolsRegistry) == 0 {
		s.registerAllTools()
	}
	log.JSONLog("info", "MCP server (SDK) running on stdio", nil)
	return s.server.Run(context.Background(), &mcp.StdioTransport{})
}

// Server returns the underlying MCP server instance (for alternate transports such as SSE).
func (s *MCPServer) Server() *mcp.Server {
	return s.server
}

/*
handleDiscoverJoins implements the MCP handler for join discovery.

It extracts foreign key relationships and suggests JOIN SQL for supported databases.
Supports MySQL, MariaDB, PostgreSQL, and SQLite.

Input: DiscoverJoinsParams (profile_name required, tables optional)
Output: DiscoverJoinsResult (list of join suggestions and summary)
*/
func (s *MCPServer) handleDiscoverJoins(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input DiscoverJoinsParams,
) (*mcp.CallToolResult, any, error) {
	p := input

	conn, prof, err := s.openConnection(ctx, p.ProfileName, "")
	if err != nil {
		if prof == nil {
			log.JSONLog("error", messageProfileNotFound, map[string]interface{}{"profile_name": p.ProfileName})
			return nil, nil, errors.New(errorProfileNotFound)
		}
		log.JSONLog("error", messageFailedToConnectToDatabase, map[string]interface{}{"error": err})
		structErr := s.errorAnalyzer.AnalyzeError(err, map[string]interface{}{
			"profile_name": p.ProfileName,
			"operation":    "discover_joins",
			"db_type":      prof.DBType,
		})
		return errorResult(structErr), nil, nil
	}
	defer conn.Close() //nolint:errcheck // Standard pattern: error in deferred close is not critical

	tableSet := buildTableFilterSet(p.Tables)
	joins, errResult, err := s.collectJoinSuggestions(ctx, conn, *prof, p.ProfileName, p.Tables, tableSet)
	if errResult != nil || err != nil {
		return errResult, nil, err
	}

	summary := fmt.Sprintf("Discovered %d join(s) based on foreign key relationships.", len(joins))
	result := DiscoverJoinsResult{
		Joins:   joins,
		Summary: summary,
	}
	b, _ := json.Marshal(result)
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(b),
			},
		},
	}, nil, nil
}

func buildTableFilterSet(tables []string) map[string]bool {
	tableSet := map[string]bool{}
	for _, table := range tables {
		tableSet[strings.ToLower(table)] = true
	}
	return tableSet
}

func shouldIncludeJoin(tableSet map[string]bool, fromTable, toTable string) bool {
	return len(tableSet) == 0 || tableSet[strings.ToLower(fromTable)] || tableSet[strings.ToLower(toTable)]
}

func (s *MCPServer) collectJoinSuggestions(
	ctx context.Context,
	conn *sql.DB,
	prof config.Profile,
	profileName string,
	requestedTables []string,
	tableSet map[string]bool,
) ([]JoinSuggestion, *mcp.CallToolResult, error) {
	if prof.DBType == "sqlite" {
		joins, err := collectSQLiteJoinSuggestions(ctx, conn, requestedTables, tableSet)
		return joins, nil, err
	}

	fkQuery, err := foreignKeyQuery(prof.DBType)
	if err != nil {
		return nil, nil, err
	}
	joins, err := collectStandardJoinSuggestions(ctx, conn, prof, fkQuery, tableSet)
	if err != nil {
		log.JSONLog("error", "Failed to query foreign keys", map[string]interface{}{"error": err})
		structErr := s.errorAnalyzer.AnalyzeError(err, map[string]interface{}{
			"profile_name": profileName,
			"operation":    "discover_joins",
			"db_type":      prof.DBType,
		})
		return nil, &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: structErr.ToJSON(),
				},
			},
		}, nil
	}
	return joins, nil, nil
}

func foreignKeyQuery(dbType string) (string, error) {
	switch dbType {
	case "mysql", "mariadb":
		return `
			SELECT TABLE_NAME, COLUMN_NAME, REFERENCED_TABLE_NAME, REFERENCED_COLUMN_NAME
			FROM INFORMATION_SCHEMA.KEY_COLUMN_USAGE
			WHERE TABLE_SCHEMA = ? AND REFERENCED_TABLE_NAME IS NOT NULL`, nil
	case "postgres":
		return `
			SELECT
				tc.table_name, kcu.column_name,
				ccu.table_name AS foreign_table_name,
				ccu.column_name AS foreign_column_name
			FROM
				information_schema.table_constraints AS tc
				JOIN information_schema.key_column_usage AS kcu
				  ON tc.constraint_name = kcu.constraint_name
				  AND tc.table_schema = kcu.table_schema
				JOIN information_schema.constraint_column_usage AS ccu
				  ON ccu.constraint_name = tc.constraint_name
				  AND ccu.table_schema = tc.table_schema
			WHERE tc.constraint_type = 'FOREIGN KEY' AND tc.table_schema = 'public'`, nil
	default:
		return "", fmt.Errorf("unsupported db_type for join discovery")
	}
}

func collectSQLiteJoinSuggestions(
	ctx context.Context,
	conn *sql.DB,
	requestedTables []string,
	tableSet map[string]bool,
) ([]JoinSuggestion, error) {
	tables := requestedTables
	if len(tables) == 0 {
		listedTables, err := listSQLiteTables(ctx, conn)
		if err != nil {
			return nil, err
		}
		tables = listedTables
	}

	joins := make([]JoinSuggestion, 0)
	for _, table := range tables {
		joins = append(joins, collectSQLiteTableJoins(ctx, conn, table, tableSet)...)
	}
	return joins, nil
}

func listSQLiteTables(ctx context.Context, conn *sql.DB) ([]string, error) {
	rows, err := conn.QueryContext(ctx, sqliteListTablesQuery)
	if err != nil {
		return nil, err
	}
	tables := make([]string, 0)
	for rows.Next() {
		var name string
		if rows.Scan(&name) == nil {
			tables = append(tables, name)
		}
	}
	if err := rows.Close(); err != nil {
		log.JSONLog("warn", "Failed to close SQLite table list rows", map[string]interface{}{"error": err.Error()})
	}
	return tables, nil
}

func collectSQLiteTableJoins(ctx context.Context, conn *sql.DB, table string, tableSet map[string]bool) []JoinSuggestion {
	rows, err := conn.QueryContext(ctx, fmt.Sprintf("PRAGMA foreign_key_list('%s')", table)) // NOSONAR
	if err != nil {
		log.JSONLog("warn", "Failed to query SQLite foreign keys", map[string]interface{}{"table": table, "error": err})
		return nil
	}

	joins := make([]JoinSuggestion, 0)
	for rows.Next() {
		var id, seq int
		var foreignTable, from, to, onUpdate, onDelete, match string
		if err := rows.Scan(&id, &seq, &foreignTable, &from, &to, &onUpdate, &onDelete, &match); err != nil {
			log.JSONLog("warn", "Failed to scan SQLite foreign key row", map[string]interface{}{"table": table, "error": err.Error()})
			continue
		}
		if !shouldIncludeJoin(tableSet, table, foreignTable) {
			continue
		}
		joins = append(joins, JoinSuggestion{
			FromTable:        table,
			FromColumn:       from,
			ToTable:          foreignTable,
			ToColumn:         to,
			Relationship:     "foreign_key",
			SuggestedJoinSQL: fmt.Sprintf(joinSQLTemplate, table, foreignTable, table, from, foreignTable, to),
		})
	}
	if err := rows.Close(); err != nil {
		log.JSONLog("warn", "Failed to close SQLite foreign key rows", map[string]interface{}{"table": table, "error": err.Error()})
	}
	return joins
}

func collectStandardJoinSuggestions(
	ctx context.Context,
	conn *sql.DB,
	prof config.Profile,
	fkQuery string,
	tableSet map[string]bool,
) ([]JoinSuggestion, error) {
	rows, err := queryForeignKeys(ctx, conn, prof, fkQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck // Standard pattern: error in deferred close is not critical

	joins := make([]JoinSuggestion, 0)
	for rows.Next() {
		var fromTable, fromCol, toTable, toCol string
		if rows.Scan(&fromTable, &fromCol, &toTable, &toCol) != nil {
			continue
		}
		if !shouldIncludeJoin(tableSet, fromTable, toTable) {
			continue
		}
		joins = append(joins, JoinSuggestion{
			FromTable:        fromTable,
			FromColumn:       fromCol,
			ToTable:          toTable,
			ToColumn:         toCol,
			Relationship:     "foreign_key",
			SuggestedJoinSQL: fmt.Sprintf(joinSQLTemplate, fromTable, toTable, fromTable, fromCol, toTable, toCol),
		})
	}
	return joins, nil
}

func queryForeignKeys(ctx context.Context, conn *sql.DB, prof config.Profile, fkQuery string) (*sql.Rows, error) {
	if prof.DBType == "mysql" || prof.DBType == "mariadb" {
		return conn.QueryContext(ctx, fkQuery, prof.DatabaseName)
	}
	return conn.QueryContext(ctx, fkQuery)
}

// handleMCPInfo returns author and version information.
func (s *MCPServer) handleMCPInfo(ctx context.Context, _ *mcp.CallToolRequest, input any) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: "Database MCP Provider\nAuthor: " + MCPAuthor + "\nVersion: " + MCPVersion + "\nBuilt via OpenAgent framework with multiple AI models (Claude, GLM, GPT, etc.).\nFeatures: 21 tools (including profile delete/clone), optimized for strict declaration budgets.",
			},
		},
	}, nil, nil
}

// --- MCP Handler Parameter Structs ---

type ConfigureProfileParams struct {
	Action        string `json:"action,omitempty"`
	ProfileName   string `json:"profile_name"`
	DBType        string `json:"db_type,omitempty"`
	Host          string `json:"host,omitempty"`
	Port          int    `json:"port,omitempty"`
	Username      string `json:"username,omitempty"`
	Password      string `json:"password,omitempty"`
	DatabaseName  string `json:"database_name,omitempty"`
	Readonly      bool   `json:"readonly"`
	SSLMode       string `json:"sslmode,omitempty"`
	SourceProfile string `json:"source_profile,omitempty"`
}

type ListProfilesParams struct{}

type ListProfilesResult struct {
	Profiles []struct {
		ProfileName string `json:"profile_name"`
		DBType      string `json:"db_type"`
	} `json:"profiles"`
}

type ExecuteSQLParams struct {
	ProfileName  string        `json:"profile_name"`
	SQL          string        `json:"sql"`
	DatabaseName string        `json:"database_name"`
	Params       []interface{} `json:"params,omitempty"`
}

type ExecuteSQLResult struct {
	Columns  []string        `json:"columns,omitempty"`
	Rows     [][]interface{} `json:"rows,omitempty"`
	Affected int             `json:"affected,omitempty"`
}

type ListTablesParams struct {
	ProfileName  string `json:"profile_name"`
	DatabaseName string `json:"database_name"`
}

type TableRef struct {
	Schema string `json:"schema"`
	Name   string `json:"table"`
}

type ListTablesResult struct {
	Tables []TableRef `json:"tables"`
}

type DescribeTableParams struct {
	ProfileName  string `json:"profile_name"`
	DatabaseName string `json:"database_name"`
	TableName    string `json:"table_name"`
	Schema       string `json:"schema,omitempty"`
}

type DescribeTableResult struct {
	Columns []ColumnInfo `json:"columns"`
}

type ColumnInfo struct {
	Name          string  `json:"name"`
	Type          string  `json:"type"`
	Nullable      bool    `json:"nullable"`
	Key           string  `json:"key,omitempty"`
	Default       *string `json:"default"`
	Comment       string  `json:"comment,omitempty"`
	Extra         string  `json:"extra,omitempty"`
	CharacterSet  string  `json:"character_set,omitempty"`
	Collation     string  `json:"collation,omitempty"`
	AutoIncrement bool    `json:"auto_increment"`
	MaxLength     *int64  `json:"max_length,omitempty"`
	Precision     *int64  `json:"precision,omitempty"`
	Scale         *int64  `json:"scale,omitempty"`
}

// --- Smart Query Builder Types ---
type SmartQueryBuilderParams struct {
	ProfileName  string   `json:"profile_name"`
	Intent       string   `json:"intent"`
	DatabaseName string   `json:"database_name,omitempty"`
	TableNames   []string `json:"table_names,omitempty"`
}

type SmartQueryBuilderResult struct {
	SQL         string `json:"sql"`
	Explanation string `json:"explanation"`
}

// --- Validate Query Types ---
type ValidateQueryParams struct {
	ProfileName  string        `json:"profile_name"`
	DatabaseName string        `json:"database_name,omitempty"`
	SQL          string        `json:"sql"`
	Params       []interface{} `json:"params,omitempty"`
}

type ValidateQueryResult struct {
	IsValid  bool              `json:"is_valid"`
	Issues   []ValidationIssue `json:"issues,omitempty"`
	Summary  string            `json:"summary"`
	SQL      string            `json:"sql"`
	Profile  string            `json:"profile_name,omitempty"`
	Database string            `json:"database_name,omitempty"`
}

// --- Optimize Query Types ---
type OptimizeQueryParams struct {
	ProfileName  string        `json:"profile_name"`
	DatabaseName string        `json:"database_name"`
	SQL          string        `json:"sql"`
	Params       []interface{} `json:"params,omitempty"`
}

type OptimizeQueryResult struct {
	Plan       *ExplainPlan          `json:"plan"`
	Findings   []OptimizationFinding `json:"findings,omitempty"`
	Estimation PerformanceEstimation `json:"estimation"`
	Summary    string                `json:"summary"`
}

// --- End Smart Query Builder Types ---
type DiscoverJoinsParams struct {
	ProfileName string   `json:"profile_name"`
	Tables      []string `json:"tables,omitempty"`
}

type JoinSuggestion struct {
	FromTable        string `json:"from_table"`
	FromColumn       string `json:"from_column"`
	ToTable          string `json:"to_table"`
	ToColumn         string `json:"to_column"`
	Relationship     string `json:"relationship"` // always "foreign_key"
	SuggestedJoinSQL string `json:"suggested_join_sql"`
}

type DiscoverJoinsResult struct {
	Joins   []JoinSuggestion `json:"joins"`
	Summary string           `json:"summary"`
}

type ListDatabasesParams struct {
	ProfileName string `json:"profile_name"`
}

type MCPInfoParams struct{}

type ListDatabasesResult struct {
	Databases []string `json:"databases"`
}

// --- Sample Data Types ---
type SampleDataParams struct {
	ProfileName  string `json:"profile_name"`
	TableName    string `json:"table_name"`
	DatabaseName string `json:"database_name"`
	Schema       string `json:"schema,omitempty"`
	SampleSize   int    `json:"sample_size,omitempty"`
}

type SampleDataResult struct {
	TableName  string          `json:"table_name"`
	SampleSize int             `json:"sample_size"`
	Columns    []string        `json:"columns"`
	SampleRows [][]interface{} `json:"sample_rows"`
	Summary    string          `json:"summary"`
}

// --- List Tools Types ---

// ListToolsParams represents the parameters for the list-tools action
type ListToolsParams struct {
	// No parameters required
}

// ToolInfo represents information about a single tool
type ToolInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ToolDeclarationInfo captures MCP declaration payload fields used in tools/list.
type ToolDeclarationInfo struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	InputSchema any    `json:"inputSchema,omitempty"`
}

// --- Lineage Types ---
type AnalyzeDataLineageParams struct {
	ProfileName  string   `json:"profile_name"`
	DatabaseName string   `json:"database_name,omitempty"`
	TableName    string   `json:"table_name"`
	Scope        string   `json:"scope,omitempty"`
	Tables       []string `json:"tables,omitempty"`
}

type AnalyzeDataLineageResult struct {
	Upstream   []string            `json:"upstream,omitempty"`
	Downstream []string            `json:"downstream,omitempty"`
	Edges      []lineage.Edge      `json:"edges,omitempty"`
	Summary    string              `json:"summary"`
	Scope      string              `json:"scope"`
	Target     string              `json:"target"`
	Graph      map[string][]string `json:"graph,omitempty"`
}

// ListToolsResult represents the response from the list-tools action
type ListToolsResult struct {
	Tools []ToolInfo `json:"tools"`
}

// ListSchemasParams represents parameters for the list-schemas tool
type ListSchemasParams struct {
	ProfileName  string `json:"profile_name"`
	DatabaseName string `json:"database_name"`
}

// ListSchemasResult represents the response from the list-schemas tool
type ListSchemasResult struct {
	Schemas       []string `json:"schemas"`
	DefaultSchema string   `json:"default_schema"`
}

// GetSearchPathParams represents parameters for the get-search-path tool
type GetSearchPathParams struct {
	ProfileName  string `json:"profile_name"`
	DatabaseName string `json:"database_name"`
}

// GetSearchPathResult represents the response from the get-search-path tool
type GetSearchPathResult struct {
	SearchPath               string `json:"search_path"`
	CurrentSchema            string `json:"current_schema"`
	ConnectionPoolingWarning string `json:"connection_pooling_warning,omitempty"`
}

func (s *MCPServer) handleListTools(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input ListToolsParams,
) (*mcp.CallToolResult, any, error) {
	result := ListToolsResult{
		Tools: s.toolsRegistry,
	}
	b, err := json.Marshal(result)
	if err != nil {
		return nil, nil, err
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(b),
			},
		},
	}, nil, nil
}

func (s *MCPServer) registerToolInfo(tool *mcp.Tool) {
	s.toolsRegistry = append(s.toolsRegistry, ToolInfo{Name: tool.Name, Description: tool.Description})
	s.toolDecls = append(s.toolDecls, ToolDeclarationInfo{Name: tool.Name, Description: tool.Description, InputSchema: tool.InputSchema})
}

func addTool[In any, Out any](
	s *MCPServer,
	tool *mcp.Tool,
	handler func(context.Context, *mcp.CallToolRequest, In) (*mcp.CallToolResult, Out, error),
) {
	mcp.AddTool(s.server, tool, wrapToolHandler(tool.Name, handler))
	s.registerToolInfo(tool)
}

func wrapToolHandler[In any, Out any](
	toolName string,
	handler func(context.Context, *mcp.CallToolRequest, In) (*mcp.CallToolResult, Out, error),
) func(context.Context, *mcp.CallToolRequest, In) (*mcp.CallToolResult, Out, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input In) (*mcp.CallToolResult, Out, error) {
		result, output, err := handler(ctx, req, input)
		if err == nil {
			return result, output, nil
		}
		structErr := NewStructuredError(
			ErrorCodeInternalError,
			"Tool execution failed",
			err.Error(),
		).WithContext("tool_name", toolName)
		var zeroOut Out
		return errorResult(structErr), zeroOut, nil
	}
}

// resourceToolsHandler returns the current tools registry as a resource.
func (s *MCPServer) resourceToolsHandler(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	b, err := json.Marshal(s.toolsRegistry)
	if err != nil {
		return nil, err
	}
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{
				URI:      req.Params.URI,
				MIMEType: mimeTypeApplicationJSON,
				Text:     string(b),
			},
		},
	}, nil
}

// resourceProfileHandler returns profile metadata (with secrets removed) for the requested profile.
func (s *MCPServer) resourceProfileHandler(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	const prefix = "profile://"
	if !strings.HasPrefix(req.Params.URI, prefix) {
		return nil, fmt.Errorf("invalid profile resource uri: %s", req.Params.URI)
	}
	profileName := strings.TrimPrefix(req.Params.URI, prefix)
	if profileName == "" {
		return nil, fmt.Errorf("profile name missing in uri: %s", req.Params.URI)
	}

	_, prof, err := s.findProfile(profileName)
	if err != nil {
		if err.Error() == errorProfileNotFound {
			return nil, fmt.Errorf("profile not found: %s", profileName)
		}
		return nil, err
	}
	// Redact password
	safe := *prof
	safe.Password = ""

	b, err := json.Marshal(safe)
	if err != nil {
		return nil, err
	}
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{
				URI:      req.Params.URI,
				MIMEType: mimeTypeApplicationJSON,
				Text:     string(b),
			},
		},
	}, nil
}

// --- MCP Handler Implementations ---

func appendContextNote(explanation string, history []ctxmgr.Message) string {
	contextNote := buildContextNote(history)
	if contextNote == "" {
		return explanation
	}
	return fmt.Sprintf("%s %s", explanation, contextNote)
}

func buildContextNote(history []ctxmgr.Message) string {
	lastUser := lastMessageContent(history, "user")
	lastAssistant := lastMessageContent(history, "assistant")
	if lastUser == "" && lastAssistant == "" {
		return ""
	}
	parts := make([]string, 0, 2)
	if lastUser != "" {
		parts = append(parts, fmt.Sprintf("last user intent: %s", lastUser))
	}
	if lastAssistant != "" {
		parts = append(parts, fmt.Sprintf("last generated SQL: %s", lastAssistant))
	}
	return fmt.Sprintf("Context (%s).", strings.Join(parts, "; "))
}

func isContextEnabled(cfg config.NLPConfig) bool {
	if cfg.Enabled == nil {
		return true
	}
	return *cfg.Enabled
}

func applyContextSettings(manager *ctxmgr.Manager, cfg config.NLPConfig) {
	if manager == nil {
		return
	}
	if cfg.ContextTimeout != "" {
		if ttl, err := time.ParseDuration(cfg.ContextTimeout); err == nil {
			manager.SetTTL(ttl)
		}
	}
	if cfg.MaxConversationLength > 0 {
		manager.SetMaxRecent(cfg.MaxConversationLength)
	}
}

func contextAwareIntent(intent string, history []ctxmgr.Message) string {
	if len(history) == 0 {
		return intent
	}
	lastUser := lastMessageContent(history, "user")
	if lastUser == "" {
		return intent
	}
	return fmt.Sprintf("%s %s", lastUser, intent)
}

func combineEntityHints(primary, history nlp.EntityExtractionResult) nlp.EntityExtractionResult {
	result := nlp.EntityExtractionResult{}
	result.Tables = appendUnique(result.Tables, primary.Tables)
	result.Columns = appendUnique(result.Columns, primary.Columns)
	result.Tables = appendUnique(result.Tables, history.Tables)
	result.Columns = appendUnique(result.Columns, history.Columns)
	return result
}

func historyIntent(history []ctxmgr.Message) string {
	parts := make([]string, 0, len(history))
	for _, msg := range history {
		if msg.Role == "user" {
			parts = append(parts, msg.Content)
		}
	}
	return strings.Join(parts, " ")
}

func selectTableForIntent(p SmartQueryBuilderParams, tables []string, contextEnabled bool, history []ctxmgr.Message, domains []string) string {
	if len(p.TableNames) > 0 {
		return p.TableNames[0]
	}
	if len(tables) == 0 {
		return ""
	}
	intent := p.Intent
	if contextEnabled {
		intent = contextAwareIntent(p.Intent, history)
	}
	keywords := extractKeywords(intent)
	if len(domains) > 0 {
		keywords = append(keywords, domainKeywords(domains)...)
	}
	return matchTableByKeywords(tables, keywords)
}

func domainKeywords(domains []string) []string {
	keywords := make([]string, 0, len(domains))
	for _, domain := range domains {
		keyword := strings.ToLower(strings.TrimSpace(domain))
		if keyword == "" {
			continue
		}
		keywords = append(keywords, keyword)
	}
	return keywords
}

func extractKeywords(intent string) []string {
	words := strings.FieldsFunc(strings.ToLower(intent), func(r rune) bool {
		return r == ' ' || r == ',' || r == '.' || r == '-' || r == '_'
	})
	stopwords := map[string]bool{
		"the": true, "a": true, "an": true, "for": true, "to": true, "of": true, "and": true, "in": true, "on": true, "with": true, "by": true, "at": true, "from": true, "as": true, "is": true, "are": true, "was": true, "were": true, "be": true, "this": true, "that": true, "it": true, "dashboard": true,
	}
	var keywords []string
	for _, w := range words {
		if !stopwords[w] && len(w) > 2 {
			keywords = append(keywords, w)
		}
	}
	return keywords
}

func matchTableByKeywords(tables, keywords []string) string {
	bestScore := 0
	selected := ""
	for _, t := range tables {
		score := 0
		tLower := strings.ToLower(t)
		for _, k := range keywords {
			if strings.Contains(tLower, k) {
				score++
			}
		}
		if score > bestScore {
			bestScore = score
			selected = t
		}
	}
	if selected == "" {
		return tables[0]
	}
	return selected
}

func appendUnique(target, values []string) []string {
	for _, val := range values {
		if !stringInSlice(target, val) {
			target = append(target, val)
		}
	}
	return target
}

func stringInSlice(values []string, val string) bool {
	for _, v := range values {
		if v == val {
			return true
		}
	}
	return false
}

func lastMessageContent(history []ctxmgr.Message, role string) string {
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role == role {
			return history[i].Content
		}
	}
	return ""
}

// handleSmartQueryBuilder generates SQL from high-level intent and schema.
func (s *MCPServer) handleSmartQueryBuilder(ctx context.Context, _ *mcp.CallToolRequest, input SmartQueryBuilderParams) (*mcp.CallToolResult, any, error) {
	p := input

	cfg, prof, profileErr, err := s.loadSmartBuilderProfile(p)
	if profileErr != nil || err != nil {
		return profileErr, nil, err
	}

	contextEnabled := isContextEnabled(cfg.NLP)
	applyContextSettings(s.contextMgr, cfg.NLP)

	var history []ctxmgr.Message
	if contextEnabled {
		history = s.contextMgr.History(p.ProfileName)
	}
	intentForMatch := p.Intent
	if contextEnabled {
		intentForMatch = contextAwareIntent(p.Intent, history)
	}
	intentResult := nlp.ClassifyIntent(intentForMatch)
	entityResult := nlp.ExtractEntities(p.Intent)
	if contextEnabled {
		historyEntities := nlp.ExtractEntities(historyIntent(history))
		entityResult = combineEntityHints(entityResult, historyEntities)
	}

	conn, connErrResult := s.openSmartBuilderConnection(ctx, p, cfg, prof)
	if connErrResult != nil {
		return connErrResult, nil, nil
	}
	defer conn.Close() //nolint:errcheck // Standard pattern: error in deferred close is not critical

	tables, tableErrResult, err := s.listSmartBuilderTables(ctx, conn, p, prof)
	if tableErrResult != nil || err != nil {
		return tableErrResult, nil, err
	}

	table := selectTableForIntent(p, tables, contextEnabled, history, cfg.NLP.BusinessDomains)
	if table == "" {
		return smartBuilderNoTableMatchResult(tables), nil, nil
	}
	columns, err := querySmartBuilderColumns(ctx, conn, prof.DBType, table)
	if err != nil {
		return nil, nil, err
	}

	colList := "*"
	if len(columns) > 0 {
		colList = strings.Join(columns, ", ")
	}
	sql := fmt.Sprintf("SELECT %s FROM %s;", colList, table)
	explanation := buildSmartBuilderExplanation(table, colList, p.Intent, cfg.NLP.BusinessDomains)

	result := SmartQueryBuilderResult{
		SQL:         sql,
		Explanation: fmt.Sprintf("%s Intent: %s (%.0f%%). Entities: tables=%v, cols=%v.", explanation, intentResult.Intent, intentResult.Confidence*100, entityResult.Tables, entityResult.Columns),
	}
	if contextEnabled {
		result.Explanation = appendContextNote(result.Explanation, history)
	}

	if contextEnabled {
		// Persist minimal context keyed by profile (session IDs not exposed in current SDK)
		s.contextMgr.Append(p.ProfileName, "user", p.Intent)
		s.contextMgr.Append(p.ProfileName, "assistant", result.SQL)
	}
	b, _ := json.Marshal(result)
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(b),
			},
		},
	}, nil, nil
}

func (s *MCPServer) loadSmartBuilderProfile(p SmartQueryBuilderParams) (*config.Config, *config.Profile, *mcp.CallToolResult, error) {
	cfg, prof, err := s.findProfile(p.ProfileName)
	if err == nil {
		return cfg, prof, nil, nil
	}
	if err.Error() != errorProfileNotFound {
		return nil, nil, nil, err
	}
	structErr := NewStructuredError(
		ErrorCodeProfileNotFound,
		messageProfileNotFound,
		"profile_name does not exist; configure a profile before using smart-query-builder",
	).WithSuggestions(
		ErrorSuggestion{
			Tool:        toolConfigureProfile,
			Description: messageCreateProfileConnectionInfo,
			Example:     `{"profile_name":"mydb","db_type":"postgres","host":"localhost","port":5432,"username":"user","password":"pass","database_name":"mydb"}`,
		},
	).WithContext("profile_name", p.ProfileName)
	return nil, nil, errorResult(structErr), errors.New(errorProfileNotFound)
}

func (s *MCPServer) openSmartBuilderConnection(
	ctx context.Context,
	p SmartQueryBuilderParams,
	cfg *config.Config,
	prof *config.Profile,
) (*sql.DB, *mcp.CallToolResult) {
	dsn := db.DSN(prof.DBType, prof.Host, prof.Port, prof.Username, prof.Password, prof.DatabaseName, prof.SSLMode)
	conn, err := db.OpenConnectionWithPool(ctx, prof.DBType, dsn, cfg.MaxPoolSize)
	if err == nil {
		return conn, nil
	}
	log.JSONLog("error", "Failed to open database connection", map[string]interface{}{"profile": p.ProfileName, "error": err})
	structErr := s.errorAnalyzer.AnalyzeError(err, map[string]interface{}{
		"profile_name":  p.ProfileName,
		"database_name": p.DatabaseName,
		"operation":     "smart_query_builder",
		"db_type":       prof.DBType,
	})
	return nil, &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: structErr.ToJSON(),
			},
		},
	}
}

func (s *MCPServer) listSmartBuilderTables(
	ctx context.Context,
	conn *sql.DB,
	p SmartQueryBuilderParams,
	prof *config.Profile,
) ([]string, *mcp.CallToolResult, error) {
	query, err := tableListQuery(prof.DBType)
	if err != nil {
		return nil, nil, err
	}
	rows, err := conn.QueryContext(ctx, query)
	if err != nil {
		log.JSONLog("error", messageFailedToListTables, map[string]interface{}{"error": err})
		dbName := prof.DatabaseName
		if p.DatabaseName != "" {
			dbName = p.DatabaseName
		}
		structErr := s.errorAnalyzer.AnalyzeError(err, map[string]interface{}{
			"profile_name":  p.ProfileName,
			"database_name": dbName,
			"operation":     "list_tables",
			"db_type":       prof.DBType,
		})
		return nil, &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: structErr.ToJSON(),
				},
			},
		}, nil
	}
	defer rows.Close() //nolint:errcheck // Standard pattern: error in deferred close is not critical

	tables := make([]string, 0)
	for rows.Next() {
		name, scanErr := scanTableName(rows, prof.DBType)
		if scanErr != nil {
			log.JSONLog("warn", "Failed to scan table name", map[string]interface{}{"db_type": prof.DBType, "error": scanErr.Error(), "operation": "smart_query_builder_list_tables"})
			continue
		}
		tables = append(tables, name)
	}
	return tables, nil, nil
}

func tableListQuery(dbType string) (string, error) {
	switch dbType {
	case "mysql", "mariadb":
		return queryShowFullTables, nil
	case "postgres":
		return queryPostgresPublicInformationTable, nil
	case "sqlite":
		return sqliteListTablesQuery, nil
	default:
		return "", errors.New(errorUnsupportedDBType)
	}
}

// tableInfoListQuery returns a SQL query that yields exactly 3 columns
// (schema, name, type) suitable for scanTableInfo. This eliminates the
// column-count mismatch between query and scanner that caused BUG-001/002
// when SHOW FULL TABLES (2 cols) was paired with scanTableInfo (3-col scan).
func tableInfoListQuery(dbType string) (string, error) {
	switch dbType {
	case "mysql", "mariadb":
		return queryMysqlTableInfoList, nil
	case "postgres":
		return queryPostgresTableInfoList, nil
	case "sqlite":
		return querySqliteTableInfoList, nil
	default:
		return "", errors.New(errorUnsupportedDBType)
	}
}

func scanTableName(rows *sql.Rows, dbType string) (string, error) {
	if dbType == "mysql" || dbType == "mariadb" {
		var name, tableType string
		if err := rows.Scan(&name, &tableType); err != nil {
			return "", err
		}
		return name, nil
	}
	var name string
	if err := rows.Scan(&name); err != nil {
		return "", err
	}
	return name, nil
}

// scanTableInfo scans a 3-column row (schema, name, type) from a table info
// query. All dbTypes now use 3-column queries from tableInfoListQuery, so
// no dbType branching is needed. The type column is scanned but discarded
// since TableRef only stores Schema and Name.
func scanTableInfo(rows *sql.Rows) (TableRef, error) {
	var schema, name, tableType string
	if err := rows.Scan(&schema, &name, &tableType); err != nil {
		return TableRef{}, err
	}
	return TableRef{Schema: schema, Name: name}, nil
}

func smartBuilderNoTableMatchResult(tables []string) *mcp.CallToolResult {
	message := "No table found matching the intent for query generation."
	suggestion := "No tables found in the database."
	if len(tables) > 0 {
		suggestion = fmt.Sprintf("Available tables: %s.", strings.Join(tables, ", "))
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf(`{"status":"error","error_code":"NO_TABLE_MATCH","message":"%s %s"}`, message, suggestion),
			},
		},
	}
}

func querySmartBuilderColumns(ctx context.Context, conn *sql.DB, dbType, table string) ([]string, error) {
	query, err := smartBuilderColumnQuery(dbType, table)
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck // Standard pattern: error in deferred close is not critical

	columns := make([]string, 0)
	for rows.Next() {
		columnName, scanErr := scanSmartBuilderColumn(rows, dbType)
		if scanErr != nil {
			log.JSONLog("warn", "Failed to scan column metadata", map[string]interface{}{"table": table, "error": scanErr.Error(), "db_type": dbType, "operation": "smart_query_builder_columns"})
			continue
		}
		if columnName != "" {
			columns = append(columns, columnName)
		}
	}
	return columns, nil
}

func smartBuilderColumnQuery(dbType, table string) (string, error) {
	switch dbType {
	case "mysql", "mariadb":
		return fmt.Sprintf("SHOW COLUMNS FROM `%s`", table), nil
	case "postgres":
		return fmt.Sprintf("SELECT column_name FROM information_schema.columns WHERE table_name = '%s' AND table_schema = 'public'", table), nil
	case "sqlite":
		return fmt.Sprintf("PRAGMA table_info('%s')", table), nil
	default:
		return "", errors.New(errorUnsupportedDBType)
	}
}

func scanSmartBuilderColumn(rows *sql.Rows, dbType string) (string, error) {
	var columnName string
	switch dbType {
	case "mysql", "mariadb":
		var field, typ, null, key, def, extra string
		if err := rows.Scan(&field, &typ, &null, &key, &def, &extra); err != nil {
			return "", err
		}
		columnName = field
	case "postgres":
		if err := rows.Scan(&columnName); err != nil {
			return "", err
		}
	case "sqlite":
		var cid int
		var typ, notnull, dfltValue, pk interface{}
		if err := rows.Scan(&cid, &columnName, &typ, &notnull, &dfltValue, &pk); err != nil {
			return "", err
		}
	default:
		return "", errors.New(errorUnsupportedDBType)
	}
	return columnName, nil
}

func buildSmartBuilderExplanation(table, colList, intent string, domains []string) string {
	explanation := fmt.Sprintf("Selected table '%s' and columns [%s] based on keywords from intent '%s'.", table, colList, intent)
	if len(domains) == 0 {
		return explanation
	}
	return fmt.Sprintf("%s Business domains: %s.", explanation, strings.Join(domains, ", "))
}

// Helper: Invoke smart-query-builder handler for query suggestion generation
func (s *MCPServer) generateQuerySuggestionViaSmartBuilder(
	ctx context.Context,
	session *mcp.ServerSession,
	profileName, intent, dbName string,
	tableNames []string,
) (*SmartQueryBuilderResult, error) {
	params := SmartQueryBuilderParams{
		ProfileName:  profileName,
		Intent:       intent,
		DatabaseName: dbName,
		TableNames:   tableNames,
	}
	result, _, err := s.handleSmartQueryBuilder(ctx, &mcp.CallToolRequest{Session: session}, params)
	if err != nil || result == nil || len(result.Content) == 0 {
		return nil, err
	}
	var sqbr SmartQueryBuilderResult
	if err := json.Unmarshal([]byte(result.Content[0].(*mcp.TextContent).Text), &sqbr); err != nil {
		return nil, err
	}
	return &sqbr, nil
}

// handleOptimizeQuery runs EXPLAIN, applies optimization rules, and returns estimation.
func (s *MCPServer) handleOptimizeQuery(ctx context.Context, _ *mcp.CallToolRequest, input OptimizeQueryParams) (*mcp.CallToolResult, any, error) {
	p := input
	if strings.TrimSpace(p.ProfileName) == "" || strings.TrimSpace(p.DatabaseName) == "" || strings.TrimSpace(p.SQL) == "" {
		structErr := NewStructuredError(
			ErrorCodeMissingParameter,
			messageMissingRequiredParameters,
			"profile_name, database_name, and sql are required for optimize-query",
		).WithSuggestions(
			ErrorSuggestion{
				Action:      "Provide all required fields",
				Description: "Include profile_name, database_name, and sql in the request",
				Example:     `{"profile_name":"mydb","database_name":"mydb","sql":"SELECT 1"}`,
			},
		)
		return errorResult(structErr), nil, nil
	}

	cfg, prof, err := s.findProfile(p.ProfileName)
	if err != nil {
		if err.Error() == errorProfileNotFound {
			structErr := NewStructuredError(
				ErrorCodeProfileNotFound,
				messageProfileNotFound,
				"profile_name does not exist; configure a profile before using smart-query-builder",
			).WithSuggestions(
				ErrorSuggestion{
					Tool:        toolConfigureProfile,
					Description: messageCreateProfileConnectionInfo,
					Example:     `{"profile_name":"mydb","db_type":"postgres","host":"localhost","port":5432,"username":"user","password":"pass","database_name":"mydb"}`,
				},
			).WithContext("profile_name", p.ProfileName)
			return errorResult(structErr), nil, errors.New(errorProfileNotFound)
		}
		return nil, nil, err
	}

	// use a copy to allow database override without mutating config
	profCopy := *prof
	if p.DatabaseName != "" {
		profCopy.DatabaseName = p.DatabaseName
	}

	expl, err := s.explainWithFindings(ctx, profCopy, cfg.MaxPoolSize, p.SQL, p.Params)
	if err != nil {
		return nil, nil, err
	}

	result := OptimizeQueryResult{
		Plan:       expl.Plan,
		Findings:   expl.Findings,
		Estimation: expl.Estimate,
		Summary:    summarizeOptimization(expl),
	}
	b, _ := json.Marshal(result)
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
	}, nil, nil
}

func summarizeOptimization(expl *ExplainWithFindings) string {
	if expl == nil {
		return "No results."
	}
	imp := expl.Estimate.Improvement
	if imp.UpperPercent == 0 {
		return "No significant optimization opportunities detected."
	}
	return fmt.Sprintf(
		"Potential improvement: %.0f-%.0f%% (confidence %.0f%%). Findings: %d.",
		imp.LowerPercent, imp.UpperPercent, imp.Confidence*100, len(expl.Findings),
	)
}

// handleValidateQuery validates SQL syntax and basic safety without executing it.
func (s *MCPServer) handleValidateQuery(ctx context.Context, _ *mcp.CallToolRequest, input ValidateQueryParams) (*mcp.CallToolResult, any, error) {
	p := input
	if strings.TrimSpace(p.ProfileName) == "" || strings.TrimSpace(p.SQL) == "" {
		structErr := NewStructuredError(
			ErrorCodeMissingParameter,
			messageMissingRequiredParameters,
			"profile_name and sql are required for validate-query",
		).WithSuggestions(
			ErrorSuggestion{
				Action:      "Provide profile_name and sql",
				Description: "Include both profile_name and sql in the request payload",
				Example:     `{"profile_name":"mydb","sql":"SELECT 1"}`,
			},
		)
		return errorResult(structErr), nil, nil
	}

	_, prof, err := s.findProfile(p.ProfileName)
	if err != nil {
		if err.Error() == errorProfileNotFound {
			structErr := NewStructuredError(
				ErrorCodeProfileNotFound,
				messageProfileNotFound,
				"profile_name does not exist; configure a profile before using validate-query",
			).WithSuggestions(
				ErrorSuggestion{
					Tool:        toolConfigureProfile,
					Description: messageCreateProfileConnectionInfo,
					Example:     `{"profile_name":"mydb","db_type":"postgres","host":"localhost","port":5432,"username":"user","password":"pass","database_name":"mydb"}`,
				},
			).WithContext("profile_name", p.ProfileName)
			return errorResult(structErr), nil, errors.New(errorProfileNotFound)
		}
		return nil, nil, err
	}

	result := validateSQL(p.SQL)
	result.Profile = p.ProfileName
	if p.DatabaseName != "" {
		result.Database = p.DatabaseName
	} else {
		result.Database = prof.DatabaseName
	}

	b, _ := json.Marshal(result)
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
	}, nil, nil
}

// handleAnalyzeDataLineage computes upstream/downstream dependencies using FK relationships.
func (s *MCPServer) handleAnalyzeDataLineage(ctx context.Context, _ *mcp.CallToolRequest, input AnalyzeDataLineageParams) (*mcp.CallToolResult, any, error) {
	p := input
	if strings.TrimSpace(p.ProfileName) == "" || strings.TrimSpace(p.TableName) == "" {
		structErr := NewStructuredError(
			ErrorCodeMissingParameter,
			messageMissingRequiredParameters,
			"profile_name and table_name are required for analyze-data-lineage",
		)
		return errorResult(structErr), nil, nil
	}

	// build FK edges via INFORMATION_SCHEMA / PRAGMA similar to discover-joins
	conn, prof, err := s.openConnection(ctx, p.ProfileName, "")
	if err != nil {
		if prof == nil {
			return nil, nil, errors.New(errorProfileNotFound)
		}
		return nil, nil, err
	}
	defer conn.Close() //nolint:errcheck // Standard pattern: error in deferred close is not critical

	edges, err := collectLineageEdges(ctx, conn, *prof)
	if err != nil {
		return nil, nil, err
	}

	scope := p.Scope
	if scope == "" {
		scope = "both"
	}
	res := lineage.Analyze(p.TableName, edges, scope)
	graph := lineage.BuildGraph(edges)
	res.Graph = graph.Out

	b, _ := json.Marshal(res)
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
	}, nil, nil
}

func collectLineageEdges(ctx context.Context, conn *sql.DB, prof config.Profile) ([]lineage.Edge, error) {
	switch prof.DBType {
	case "mysql", "mariadb":
		return loadMySQLLineageEdges(ctx, conn, prof.DatabaseName)
	case "postgres":
		return loadPostgresLineageEdges(ctx, conn)
	case "sqlite":
		return loadSQLiteLineageEdges(ctx, conn)
	default:
		return nil, fmt.Errorf("unsupported db_type for lineage: %s", prof.DBType)
	}
}

func loadMySQLLineageEdges(ctx context.Context, conn *sql.DB, databaseName string) ([]lineage.Edge, error) {
	rows, err := conn.QueryContext(ctx, `
		SELECT TABLE_NAME, REFERENCED_TABLE_NAME
		FROM INFORMATION_SCHEMA.KEY_COLUMN_USAGE
		WHERE TABLE_SCHEMA = ? AND REFERENCED_TABLE_NAME IS NOT NULL
	`, databaseName)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck // Standard pattern: error in deferred close is not critical
	return scanLineageEdges(rows), nil
}

func loadPostgresLineageEdges(ctx context.Context, conn *sql.DB) ([]lineage.Edge, error) {
	rows, err := conn.QueryContext(ctx, `
		SELECT
			tc.table_name AS from_table,
			ccu.table_name AS to_table
		FROM information_schema.table_constraints AS tc
		JOIN information_schema.key_column_usage AS kcu
			ON tc.constraint_name = kcu.constraint_name AND tc.table_schema = kcu.table_schema
		JOIN information_schema.constraint_column_usage AS ccu
			ON ccu.constraint_name = tc.constraint_name AND ccu.table_schema = tc.table_schema
		WHERE tc.constraint_type = 'FOREIGN KEY' AND tc.table_schema = 'public'
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck // Standard pattern: error in deferred close is not critical
	return scanLineageEdges(rows), nil
}

func scanLineageEdges(rows *sql.Rows) []lineage.Edge {
	edges := make([]lineage.Edge, 0)
	for rows.Next() {
		var from, to string
		if rows.Scan(&from, &to) == nil {
			edges = append(edges, lineage.Edge{From: from, To: to})
		}
	}
	return edges
}

func loadSQLiteLineageEdges(ctx context.Context, conn *sql.DB) ([]lineage.Edge, error) {
	tables, err := listSQLiteTables(ctx, conn)
	if err != nil {
		return nil, err
	}
	edges := make([]lineage.Edge, 0)
	for _, table := range tables {
		rows, queryErr := conn.QueryContext(ctx, fmt.Sprintf("PRAGMA foreign_key_list('%s')", table)) // NOSONAR
		if queryErr != nil {
			continue
		}
		for rows.Next() {
			var id, seq int
			var refTable, fromCol, toCol, onUpd, onDel, match string
			if rows.Scan(&id, &seq, &refTable, &fromCol, &toCol, &onUpd, &onDel, &match) == nil {
				edges = append(edges, lineage.Edge{From: table, To: refTable})
			}
		}
		if err := rows.Close(); err != nil {
			log.JSONLog("warn", "Failed to close SQLite foreign key rows", map[string]interface{}{"table": table, "error": err.Error()})
		}
	}
	return edges, nil
}

func (s *MCPServer) handleConfigureProfile(ctx context.Context, _ *mcp.CallToolRequest, input ConfigureProfileParams) (*mcp.CallToolResult, any, error) {
	cfg, err := config.LoadConfig(s.ConfigPath)
	log.JSONLog("debug", "Loaded config for configure-profile", map[string]interface{}{"configPath": s.ConfigPath, "error": err})
	if err != nil {
		log.JSONLog("warn", "Failed to load config, creating new config", map[string]interface{}{"error": err.Error()})
		cfg = &config.Config{}
	}
	p := normalizeConfigureProfileParams(input)
	if errResult := validateConfigureProfileParams(p); errResult != nil {
		return errResult, nil, nil
	}

	switch p.Action {
	case "delete":
		return s.handleDeleteProfile(cfg, p)
	case "clone":
		return s.handleCloneProfile(cfg, p)
	default:
		return s.handleUpsertProfile(cfg, p)
	}
}

func (s *MCPServer) saveConfigResult(cfg *config.Config, profileName string) *mcp.CallToolResult {
	if err := config.SaveConfig(s.ConfigPath, cfg); err != nil {
		structErr := s.errorAnalyzer.AnalyzeError(err, map[string]interface{}{
			"profile_name": profileName,
			"operation":    "save_config",
		})
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: structErr.ToJSON()}},
		}
	}
	return nil
}

func (s *MCPServer) handleUpsertProfile(cfg *config.Config, p ConfigureProfileParams) (*mcp.CallToolResult, any, error) {
	ensureConfigAESKey(cfg, s.ConfigPath)
	upsertConfigProfile(cfg, p)
	if errResult := s.saveConfigResult(cfg, p.ProfileName); errResult != nil {
		return errResult, nil, nil
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: "Profile configured successfully."}},
	}, nil, nil
}

func (s *MCPServer) handleDeleteProfile(cfg *config.Config, p ConfigureProfileParams) (*mcp.CallToolResult, any, error) {
	found := false
	for i := range cfg.Profiles {
		if cfg.Profiles[i].ProfileName == p.ProfileName {
			cfg.Profiles = append(cfg.Profiles[:i], cfg.Profiles[i+1:]...)
			found = true
			break
		}
	}
	if !found {
		structErr := NewStructuredError(
			ErrorCodeProfileNotFound,
			messageProfileNotFound,
			fmt.Sprintf(messageProfileNotFoundFormat, p.ProfileName),
		).WithContext("profile_name", p.ProfileName)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: structErr.ToJSON()}},
		}, nil, nil
	}
	if errResult := s.saveConfigResult(cfg, p.ProfileName); errResult != nil {
		return errResult, nil, nil
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{
			Text: fmt.Sprintf("Profile '%s' deleted successfully.", p.ProfileName),
		}},
	}, nil, nil
}

func applyCloneOverrides(cloned *config.Profile, p ConfigureProfileParams) {
	if p.DBType != "" {
		cloned.DBType = p.DBType
	}
	if p.Host != "" {
		cloned.Host = p.Host
	}
	if p.Port != 0 {
		cloned.Port = p.Port
	}
	if p.Username != "" {
		cloned.Username = p.Username
	}
	if p.Password != "" {
		cloned.Password = p.Password
	}
	if p.DatabaseName != "" {
		cloned.DatabaseName = p.DatabaseName
	}
	if p.Readonly {
		cloned.Readonly = p.Readonly
	}
	if p.SSLMode != "" {
		cloned.SSLMode = p.SSLMode
	}
}

func (s *MCPServer) handleCloneProfile(cfg *config.Config, p ConfigureProfileParams) (*mcp.CallToolResult, any, error) {
	// Find source profile
	var source *config.Profile
	for i := range cfg.Profiles {
		if cfg.Profiles[i].ProfileName == p.SourceProfile {
			source = &cfg.Profiles[i]
			break
		}
	}
	if source == nil {
		structErr := NewStructuredError(
			ErrorCodeProfileNotFound,
			messageProfileNotFound,
			fmt.Sprintf("Source profile '%s' not found", p.SourceProfile),
		).WithContext("source_profile", p.SourceProfile)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: structErr.ToJSON()}},
		}, nil, nil
	}

	// Check target doesn't already exist
	for i := range cfg.Profiles {
		if cfg.Profiles[i].ProfileName == p.ProfileName {
			structErr := NewStructuredError(
				ErrorCodeInvalidInput,
				"Profile already exists",
				fmt.Sprintf("Profile '%s' already exists; use a different name or delete it first", p.ProfileName),
			).WithContext("profile_name", p.ProfileName)
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: structErr.ToJSON()}},
			}, nil, nil
		}
	}

	// Copy source and apply overrides
	cloned := *source
	cloned.ProfileName = p.ProfileName
	applyCloneOverrides(&cloned, p)

	ensureConfigAESKey(cfg, s.ConfigPath)
	cfg.Profiles = append(cfg.Profiles, cloned)

	if errResult := s.saveConfigResult(cfg, p.ProfileName); errResult != nil {
		return errResult, nil, nil
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{
			Text: fmt.Sprintf("Profile '%s' cloned from '%s' successfully.", p.ProfileName, p.SourceProfile),
		}},
	}, nil, nil
}

func normalizeConfigureProfileParams(input ConfigureProfileParams) ConfigureProfileParams {
	p := input
	if (p.DBType == "mysql" || p.DBType == "mariadb") && p.DatabaseName == "" {
		p.DatabaseName = "mysql"
	}
	if p.DBType == "postgres" && p.SSLMode == "" {
		p.SSLMode = "require"
	}
	return p
}

func validateConfigureProfileParams(p ConfigureProfileParams) *mcp.CallToolResult {
	switch p.Action {
	case "":
		if p.ProfileName != "" && p.DBType != "" && p.DatabaseName != "" {
			return nil
		}
		structErr := NewStructuredError(
			ErrorCodeMissingParameter,
			messageMissingRequiredParameters,
			"All of profile_name, db_type, and database_name are required",
		).WithSuggestions(
			ErrorSuggestion{
				Action:      actionProvideAllRequiredParameters,
				Description: "Ensure profile_name, db_type, and database_name are included",
				Example:     `{"profile_name": "mydb", "db_type": "mysql", "database_name": "mydb"}`,
			},
		)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: structErr.ToJSON()}},
		}

	case "delete":
		if p.ProfileName == "" {
			structErr := NewStructuredError(
				ErrorCodeMissingParameter,
				messageMissingRequiredParameters,
				"profile_name is required for delete",
			).WithSuggestions(ErrorSuggestion{
				Action:  actionProvideAllRequiredParameters,
				Example: `{"action": "delete", "profile_name": "mydb"}`,
			})
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: structErr.ToJSON()}},
			}
		}
		return nil

	case "clone":
		if p.ProfileName == "" || p.SourceProfile == "" {
			structErr := NewStructuredError(
				ErrorCodeMissingParameter,
				messageMissingRequiredParameters,
				"profile_name and source_profile are required for clone",
			).WithSuggestions(ErrorSuggestion{
				Action:  actionProvideAllRequiredParameters,
				Example: `{"action": "clone", "profile_name": "new-profile", "source_profile": "existing-profile"}`,
			})
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: structErr.ToJSON()}},
			}
		}
		return nil

	default:
		structErr := NewStructuredError(
			ErrorCodeInvalidInput,
			"Unknown action",
			fmt.Sprintf("Unknown action '%s'; valid actions are: delete, clone (or omit for create/update)", p.Action),
		)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: structErr.ToJSON()}},
		}
	}
}

func ensureConfigAESKey(cfg *config.Config, configPath string) {
	if cfg.AESKey != "" && len(cfg.AESKey) == 32 {
		return
	}
	keyBytes := make([]byte, 24) // Base64(24 bytes) == 32 chars, no padding
	if _, err := rand.Read(keyBytes); err != nil {
		log.JSONLog("error", "Failed to generate AES key", map[string]interface{}{"error": err.Error()})
		return
	}
	cfg.AESKey = base64.StdEncoding.EncodeToString(keyBytes)
	log.JSONLog("info", "Generated new AES key for configuration", map[string]interface{}{
		"config_path": configPath,
		"length":      len(cfg.AESKey),
	})
}

func upsertConfigProfile(cfg *config.Config, p ConfigureProfileParams) {
	profile := config.Profile{
		ProfileName:  p.ProfileName,
		DBType:       p.DBType,
		Host:         p.Host,
		Port:         p.Port,
		Username:     p.Username,
		Password:     p.Password,
		DatabaseName: p.DatabaseName,
		Readonly:     p.Readonly,
		SSLMode:      p.SSLMode,
	}
	for idx := range cfg.Profiles {
		if cfg.Profiles[idx].ProfileName == p.ProfileName {
			cfg.Profiles[idx] = profile
			return
		}
	}
	cfg.Profiles = append(cfg.Profiles, profile)
}

func (s *MCPServer) handleListProfiles(ctx context.Context, _ *mcp.CallToolRequest, input any) (*mcp.CallToolResult, any, error) {
	cfg, err := config.LoadConfig(s.ConfigPath)
	if err != nil || len(cfg.Profiles) == 0 {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: "No database profiles configured. Please use the 'configure-profile' tool to add a profile.",
				},
			},
		}, nil, nil
	}
	result := ListProfilesResult{}
	for _, p := range cfg.Profiles {
		result.Profiles = append(result.Profiles, struct {
			ProfileName string `json:"profile_name"`
			DBType      string `json:"db_type"`
		}{
			ProfileName: p.ProfileName,
			DBType:      p.DBType,
		})
	}
	b, err := json.Marshal(result)
	if err != nil {
		return nil, nil, err
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(b),
			},
		},
	}, nil, nil
}

func (s *MCPServer) handleExecuteSQL(ctx context.Context, _ *mcp.CallToolRequest, input ExecuteSQLParams) (*mcp.CallToolResult, any, error) {
	p := input
	if errResult := validateExecuteSQLParams(p); errResult != nil {
		return errResult, nil, nil
	}

	cfg, prof, errResult, err := s.resolveExecuteSQLProfile(p)
	if errResult != nil || err != nil {
		return errResult, nil, err
	}
	if errResult := enforceReadOnlyPolicy(p.ProfileName, p.SQL, prof.Readonly); errResult != nil {
		return errResult, nil, fmt.Errorf("blocked by readonly profile")
	}

	dbName := selectedDatabaseName(prof.DatabaseName, p.DatabaseName)
	conn, errResult := s.openExecuteSQLConnection(ctx, p.ProfileName, dbName, cfg.MaxPoolSize, prof)
	if errResult != nil {
		return errResult, nil, nil
	}
	defer conn.Close() //nolint:errcheck // Standard pattern: error in deferred close is not critical

	if result := s.switchMySQLDatabase(ctx, conn, prof, p.ProfileName, p.DatabaseName); result != nil {
		return result, nil, nil
	}

	queryResult, handled, errResult, err := s.tryExecuteSQLQuery(ctx, conn, p, prof)
	if errResult != nil || err != nil {
		return errResult, nil, err
	}
	if handled {
		return callToolResultForExecuteSQL(queryResult), nil, nil
	}

	execResult, errResult := s.executeSQLStatement(ctx, conn, p, prof, dbName)
	if errResult != nil {
		return errResult, nil, nil
	}
	affected, _ := execResult.RowsAffected()
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(mustJSONMarshal(ExecuteSQLResult{
					Affected: int(affected),
				})),
			},
		},
	}, nil, nil
}

func validateExecuteSQLParams(p ExecuteSQLParams) *mcp.CallToolResult {
	if p.ProfileName != "" && p.DatabaseName != "" {
		return nil
	}
	structErr := NewStructuredError(
		ErrorCodeMissingParameter,
		messageMissingRequiredParameters,
		"Both profile_name and database_name are required",
	).WithSuggestions(
		ErrorSuggestion{
			Action:      actionProvideAllRequiredParameters,
			Description: "Ensure profile_name and database_name are included",
			Example:     `{"profile_name": "mydb", "database_name": "mydb", "sql": "SELECT 1"}`,
		},
	)
	return errorResult(structErr)
}

func (s *MCPServer) resolveExecuteSQLProfile(p ExecuteSQLParams) (*config.Config, *config.Profile, *mcp.CallToolResult, error) {
	cfg, prof, err := s.findProfile(p.ProfileName)
	if err == nil {
		return cfg, prof, nil, nil
	}
	if cfg == nil || len(cfg.Profiles) == 0 {
		log.JSONLog("error", "No database profiles configured", map[string]interface{}{"error": err})
		structErr := NewStructuredError(
			ErrorCodeConfigNotFound,
			"No database profiles configured",
			"Configuration file is missing or contains no profiles",
		).WithSuggestions(
			ErrorSuggestion{
				Tool:        toolConfigureProfile,
				Description: "Create a new database profile",
				Example:     `{"profile_name": "mydb", "db_type": "mysql", "host": "localhost", "port": 3306, "username": "user", "password": "pass", "database_name": "mydb"}`,
			},
		)
		return nil, nil, errorResult(structErr), nil
	}
	log.JSONLog("error", messageProfileNotFound, map[string]interface{}{"profile_name": p.ProfileName})
	return nil, nil, nil, errors.New(errorProfileNotFound)
}

func enforceReadOnlyPolicy(profileName, sqlText string, readonly bool) *mcp.CallToolResult {
	if !readonly {
		return nil
	}
	sanitized := sanitizeReadOnlySQL(stripLeadingSQLComments(sqlText))
	statements := splitSQLStatements(sanitized)
	if len(statements) > 1 {
		structErr := NewStructuredError(
			ErrorCodeSQLExecutionError,
			"Read-only profile restriction",
			"Multiple statements are not allowed on readonly profiles",
		).WithSuggestions(
			ErrorSuggestion{
				Action:      "Send a single read-only statement",
				Description: "Combine logic into one SELECT/SHOW/DESCRIBE/EXPLAIN/PRAGMA",
				Example:     "SELECT * FROM table_name",
			},
		).WithContext("profile_name", profileName).
			WithContext("query", sqlText)
		return errorResult(structErr)
	}

	if disallowed := disallowedReadOnlyVerb(sanitized); !isAllowedReadOnlyStart(sanitized) || disallowed != "" {
		reason := "Write or unsafe operations are not allowed on readonly profiles"
		if disallowed != "" {
			reason = fmt.Sprintf("Detected disallowed verb '%s' in query for readonly profile", disallowed)
		}
		structErr := NewStructuredError(
			ErrorCodeSQLExecutionError,
			"Read-only profile restriction",
			reason,
		).WithSuggestions(
			ErrorSuggestion{
				Action:      "Use read-only queries",
				Description: "Only single SELECT (or WITH ... SELECT), SHOW, DESCRIBE, EXPLAIN, PRAGMA queries are allowed",
				Example:     "WITH recent AS (SELECT * FROM orders LIMIT 10) SELECT * FROM recent;",
			},
			ErrorSuggestion{
				Action:      "Use a different profile",
				Description: "Switch to a profile that allows write operations",
			},
		).WithContextMap(map[string]interface{}{
			"profile_name": profileName,
			"query":        sqlText,
		})
		return errorResult(structErr)
	}
	return nil
}

func stripLeadingSQLComments(sqlText string) string {
	sqlNorm := strings.TrimSpace(sqlText)
	for {
		trimmed := strings.TrimSpace(sqlNorm)
		switch {
		case strings.HasPrefix(trimmed, "--"):
			idx := strings.Index(trimmed, "\n")
			if idx == -1 {
				return ""
			}
			sqlNorm = trimmed[idx+1:]
		case strings.HasPrefix(trimmed, "/*"):
			idx := strings.Index(trimmed, "*/")
			if idx == -1 {
				return ""
			}
			sqlNorm = trimmed[idx+2:]
		default:
			return trimmed
		}
	}
}

func sanitizeReadOnlySQL(sqlText string) string {
	var builder strings.Builder
	inSingle := false
	for idx := 0; idx < len(sqlText); idx++ {
		ch := sqlText[idx]
		if ch == '\'' {
			inSingle = !inSingle
			builder.WriteByte(' ')
			continue
		}
		if inSingle {
			builder.WriteByte(' ')
			continue
		}
		builder.WriteByte(byte(unicode.ToLower(rune(ch))))
	}
	return builder.String()
}

func splitSQLStatements(sqlText string) []string {
	parts := strings.Split(sqlText, ";")
	statements := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			statements = append(statements, trimmed)
		}
	}
	return statements
}

func isAllowedReadOnlyStart(sqlText string) bool {
	allowedStarters := []string{"select", "show", "describe", "explain", "pragma", "with"}
	trimmed := strings.TrimLeft(sqlText, "(")
	for _, allowed := range allowedStarters {
		if strings.HasPrefix(trimmed, allowed) {
			return true
		}
	}
	return false
}

func disallowedReadOnlyVerb(sqlText string) string {
	disallowed := []string{
		"insert", "update", "delete", "alter", "create", "drop", "truncate",
		"grant", "revoke", "replace", "merge", "call", "do", "attach", "detach", "vacuum",
	}
	for _, verb := range disallowed {
		re := regexp.MustCompile(`\b` + verb + `\b`)
		if re.MatchString(sqlText) {
			return verb
		}
	}
	return ""
}

func selectedDatabaseName(defaultName, override string) string {
	if override != "" {
		return override
	}
	return defaultName
}

func (s *MCPServer) openExecuteSQLConnection(
	ctx context.Context,
	profileName, databaseName string,
	maxPoolSize int,
	prof *config.Profile,
) (*sql.DB, *mcp.CallToolResult) {
	dsn := db.DSN(prof.DBType, prof.Host, prof.Port, prof.Username, prof.Password, databaseName, prof.SSLMode)
	conn, err := db.OpenConnectionWithPool(ctx, prof.DBType, dsn, maxPoolSize)
	if err == nil {
		return conn, nil
	}
	log.JSONLog("error", messageFailedToConnectToDatabase, map[string]interface{}{"error": err})
	structErr := s.errorAnalyzer.AnalyzeError(err, map[string]interface{}{
		"profile_name": profileName,
		"operation":    "connect",
		"db_type":      prof.DBType,
		"database":     databaseName,
	})
	return nil, errorResult(structErr)
}

func (s *MCPServer) tryExecuteSQLQuery(
	ctx context.Context,
	conn *sql.DB,
	p ExecuteSQLParams,
	prof *config.Profile,
) (ExecuteSQLResult, bool, *mcp.CallToolResult, error) {
	rows, queryErrResult, queryErr := s.queryRowsForSQL(ctx, conn, p, prof)
	if queryErrResult != nil || queryErr != nil {
		return ExecuteSQLResult{}, false, queryErrResult, queryErr
	}
	if rows == nil {
		return ExecuteSQLResult{}, false, nil, nil
	}
	defer rows.Close() //nolint:errcheck // Standard pattern: error in deferred close is not critical

	result, err := scanExecuteSQLRows(ctx, conn, p.SQL, rows)
	if err != nil {
		return ExecuteSQLResult{}, false, nil, err
	}
	return result, true, nil, nil
}

func (s *MCPServer) queryRowsForSQL(
	ctx context.Context,
	conn *sql.DB,
	p ExecuteSQLParams,
	prof *config.Profile,
) (*sql.Rows, *mcp.CallToolResult, error) {
	if len(p.Params) == 0 {
		rows, err := conn.QueryContext(ctx, p.SQL)
		if err != nil {
			log.JSONLog("error", "Query failed", map[string]interface{}{"sql": p.SQL, "params": p.Params, "error": err})
			return nil, nil, nil
		}
		return rows, nil, nil
	}

	stmt, err := conn.PrepareContext(ctx, p.SQL)
	if err != nil {
		log.JSONLog("error", "Failed to prepare statement", map[string]interface{}{"sql": p.SQL, "error": err})
		structErr := s.errorAnalyzer.AnalyzeError(err, map[string]interface{}{
			"profile_name": p.ProfileName,
			"sql":          p.SQL,
			"operation":    "prepare_statement",
			"db_type":      prof.DBType,
		})
		return nil, errorResult(structErr), nil
	}
	defer stmt.Close() //nolint:errcheck // Standard pattern: error in deferred close is not critical

	rows, err := stmt.Query(p.Params...) //nolint:noctx // Prepared with PrepareContext, context already bound
	if err != nil {
		log.JSONLog("error", "Failed to execute prepared query", map[string]interface{}{"sql": p.SQL, "params": p.Params, "error": err})
		structErr := s.errorAnalyzer.AnalyzeError(err, map[string]interface{}{
			"profile_name": p.ProfileName,
			"sql":          p.SQL,
			"operation":    "prepared_query",
			"db_type":      prof.DBType,
		})
		return nil, errorResult(structErr), nil
	}
	return rows, nil, nil
}

func scanExecuteSQLRows(ctx context.Context, conn *sql.DB, sqlText string, rows *sql.Rows) (ExecuteSQLResult, error) {
	cols, _ := rows.Columns()
	typeMap := loadMySQLTypeMapForQuery(ctx, conn, sqlText)
	results := make([][]interface{}, 0)
	for rows.Next() {
		row := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for idx := range row {
			ptrs[idx] = &row[idx]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return ExecuteSQLResult{}, err
		}
		normalizeQueryRowValues(row, cols, typeMap)
		results = append(results, row)
	}
	return ExecuteSQLResult{Columns: cols, Rows: results}, nil
}

func loadMySQLTypeMapForQuery(ctx context.Context, conn *sql.DB, sqlText string) map[string]string {
	typeMap := make(map[string]string)
	tableName := tableNameFromSimpleSelect(sqlText)
	if tableName == "" {
		return typeMap
	}
	typeRows, err := conn.QueryContext(ctx, "DESCRIBE "+tableName) // NOSONAR
	if err != nil {
		return typeMap
	}
	for typeRows.Next() {
		var field, typ, null, key, def, extra string
		if typeRows.Scan(&field, &typ, &null, &key, &def, &extra) == nil {
			typeMap[field] = typ
		}
	}
	if err := typeRows.Close(); err != nil {
		log.JSONLog("warn", "Failed to close describe rows", map[string]interface{}{"table": tableName, "error": err.Error()})
	}
	return typeMap
}

func tableNameFromSimpleSelect(sqlText string) string {
	sqlLower := strings.ToLower(sqlText)
	if !strings.HasPrefix(strings.TrimSpace(sqlLower), "select") {
		return ""
	}
	fromIdx := strings.Index(sqlLower, "from ")
	if fromIdx == -1 {
		return ""
	}
	parts := strings.Fields(sqlLower[fromIdx+5:])
	if len(parts) == 0 {
		return ""
	}
	return strings.Trim(parts[0], "`")
}

func normalizeQueryRowValues(row []interface{}, cols []string, typeMap map[string]string) {
	for idx, value := range row {
		bytes, ok := value.([]byte)
		if !ok {
			continue
		}
		row[idx] = decodeMySQLByteValue(bytes, typeMap[cols[idx]])
	}
}

func decodeMySQLByteValue(raw []byte, colType string) interface{} {
	value := string(raw)
	switch {
	case strings.HasPrefix(colType, "int"), strings.HasPrefix(colType, "tinyint"), strings.HasPrefix(colType, "bigint"), strings.HasPrefix(colType, "smallint"), strings.HasPrefix(colType, "mediumint"):
		parsed, _ := strconv.ParseInt(value, 10, 64)
		return parsed
	case strings.HasPrefix(colType, "float"), strings.HasPrefix(colType, "double"), strings.HasPrefix(colType, "decimal"):
		parsed, _ := strconv.ParseFloat(value, 64)
		return parsed
	default:
		return value
	}
}

func (s *MCPServer) executeSQLStatement(
	ctx context.Context,
	conn *sql.DB,
	p ExecuteSQLParams,
	prof *config.Profile,
	databaseName string,
) (sql.Result, *mcp.CallToolResult) {
	if len(p.Params) == 0 {
		res, err := conn.ExecContext(ctx, p.SQL)
		if err != nil {
			return nil, executeSQLErrorResult(s.errorAnalyzer, err, p, prof, databaseName)
		}
		return res, nil
	}

	stmt, err := conn.PrepareContext(ctx, p.SQL)
	if err != nil {
		return nil, executeSQLErrorResult(s.errorAnalyzer, err, p, prof, databaseName)
	}
	defer stmt.Close() //nolint:errcheck // Standard pattern: error in deferred close is not critical

	res, err := stmt.Exec(p.Params...) //nolint:noctx // Prepared with PrepareContext, context already bound
	if err != nil {
		log.JSONLog("error", "Failed to execute prepared statement", map[string]interface{}{"sql": p.SQL, "params": p.Params, "error": err})
		structErr := s.errorAnalyzer.AnalyzeError(err, map[string]interface{}{
			"profile_name": p.ProfileName,
			"sql":          p.SQL,
			"operation":    "prepared_exec",
			"db_type":      prof.DBType,
		})
		return nil, errorResult(structErr)
	}
	return res, nil
}

func executeSQLErrorResult(analyzer *ErrorAnalyzer, err error, p ExecuteSQLParams, prof *config.Profile, dbName string) *mcp.CallToolResult {
	log.JSONLog("error", "SQL execution failed", map[string]interface{}{"sql": p.SQL, "error": err})
	structErr := analyzer.AnalyzeError(err, map[string]interface{}{
		"profile_name":  p.ProfileName,
		"sql":           p.SQL,
		"query":         p.SQL,
		"operation":     "execute_sql",
		"db_type":       prof.DBType,
		"database_name": dbName,
	})
	return errorResult(structErr)
}

func callToolResultForExecuteSQL(result ExecuteSQLResult) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(mustJSONMarshal(result)),
			},
		},
	}
}

// mustJSONMarshal is a helper for panic-free JSON marshaling.
func mustJSONMarshal(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}

func (s *MCPServer) handleListTables(ctx context.Context, _ *mcp.CallToolRequest, input ListTablesParams) (*mcp.CallToolResult, any, error) {
	p := input
	if errResult := validateListTablesParams(p); errResult != nil {
		return errResult, nil, nil
	}
	cfg, prof, errResult := s.resolveListTablesProfile(p)
	if errResult != nil {
		return errResult, nil, nil
	}

	dbName := selectedDatabaseName(prof.DatabaseName, p.DatabaseName)
	conn, errResult := s.openExecuteSQLConnection(ctx, p.ProfileName, dbName, cfg.MaxPoolSize, prof)
	if errResult != nil {
		return errResult, nil, nil
	}
	defer conn.Close() //nolint:errcheck // Standard pattern: error in deferred close is not critical

	if result := s.switchMySQLDatabase(ctx, conn, prof, p.ProfileName, p.DatabaseName); result != nil {
		return result, nil, nil
	}

	tables, errResult, err := s.queryTableNames(ctx, conn, p.ProfileName, dbName, prof.DBType)
	if errResult != nil || err != nil {
		return errResult, nil, err
	}

	result := ListTablesResult{Tables: tables}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(mustJSONMarshal(result)),
			},
		},
	}, nil, nil
}

func validateListTablesParams(p ListTablesParams) *mcp.CallToolResult {
	if p.ProfileName != "" && p.DatabaseName != "" {
		return nil
	}
	structErr := NewStructuredError(
		ErrorCodeMissingParameter,
		messageMissingRequiredParameters,
		"Both profile_name and database_name are required",
	).WithSuggestions(
		ErrorSuggestion{
			Action:      actionProvideAllRequiredParameters,
			Description: "Ensure profile_name and database_name are included",
			Example:     `{"profile_name": "mydb", "database_name": "mydb"}`,
		},
	)
	return errorResult(structErr)
}

func (s *MCPServer) resolveListTablesProfile(p ListTablesParams) (*config.Config, *config.Profile, *mcp.CallToolResult) {
	cfg, prof, err := s.findProfile(p.ProfileName)
	if err == nil {
		return cfg, prof, nil
	}
	if cfg == nil {
		structErr := NewStructuredError(
			ErrorCodeConfigNotFound,
			messageFailedToLoadConfiguration,
			err.Error(),
		).WithSuggestions(
			ErrorSuggestion{
				Action:      actionInitializeConfiguration,
				Description: descriptionRunServerCreateConfig,
			},
		)
		return nil, nil, errorResult(structErr)
	}
	availableProfiles := make([]string, 0, len(cfg.Profiles))
	for _, profile := range cfg.Profiles {
		availableProfiles = append(availableProfiles, profile.ProfileName)
	}
	structErr := NewStructuredError(
		ErrorCodeProfileNotFound,
		fmt.Sprintf(messageProfileNotFoundFormat, p.ProfileName),
		descriptionSpecifiedProfileMissing,
	).WithSuggestions(
		ErrorSuggestion{
			Tool:        toolListProfiles,
			Description: "List all available database profiles",
		},
	).WithContext("available_profiles", availableProfiles)
	return nil, nil, errorResult(structErr)
}

func (s *MCPServer) queryTableNames(
	ctx context.Context,
	conn *sql.DB,
	profileName, databaseName, dbType string,
) ([]TableRef, *mcp.CallToolResult, error) {
	query, err := tableInfoListQuery(dbType)
	if err != nil {
		return nil, nil, err
	}
	rows, err := conn.QueryContext(ctx, query)
	if err != nil {
		log.JSONLog("error", messageFailedToListTables, map[string]interface{}{"profile": profileName, "error": err})
		structErr := s.errorAnalyzer.AnalyzeError(err, map[string]interface{}{
			"profile_name":  profileName,
			"database_name": databaseName,
			"operation":     "list_tables",
			"db_type":       dbType,
		})
		return nil, errorResult(structErr), nil
	}
	defer rows.Close() //nolint:errcheck // Standard pattern: error in deferred close is not critical

	tables := make([]TableRef, 0)
	for rows.Next() {
		info, scanErr := scanTableInfo(rows)
		if scanErr != nil {
			return nil, nil, scanErr
		}
		tables = append(tables, info)
	}
	return tables, nil, nil
}

func (s *MCPServer) handleDescribeTable(ctx context.Context, _ *mcp.CallToolRequest, input DescribeTableParams) (*mcp.CallToolResult, any, error) {
	p := input
	if errResult := validateDescribeTableParams(p); errResult != nil {
		return errResult, nil, nil
	}
	cfg, prof, errResult, err := s.resolveDescribeTableProfile(p.ProfileName)
	if errResult != nil || err != nil {
		return errResult, nil, err
	}

	conn, errResult := s.openExecuteSQLConnection(ctx, p.ProfileName, p.DatabaseName, cfg.MaxPoolSize, prof)
	if errResult != nil {
		return errResult, nil, nil
	}
	defer conn.Close() //nolint:errcheck // Standard pattern: error in deferred close is not critical

	columns, errResult, err := s.queryDescribeTableColumns(ctx, conn, p, prof.DBType)
	if errResult != nil || err != nil {
		return errResult, nil, err
	}

	result := DescribeTableResult{Columns: columns}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(mustJSONMarshal(result)),
			},
		},
	}, nil, nil
}

func validateDescribeTableParams(p DescribeTableParams) *mcp.CallToolResult {
	if p.DatabaseName != "" && p.TableName != "" {
		return nil
	}
	structErr := NewStructuredError(
		ErrorCodeMissingParameter,
		messageMissingRequiredParameters,
		"Both database_name and table_name are required",
	).WithSuggestions(
		ErrorSuggestion{
			Action:      actionProvideAllRequiredParameters,
			Description: "Specify both database_name and table_name",
			Example:     `{"profile_name": "mydb", "database_name": "mydb", "table_name": "users"}`,
		},
	)
	return errorResult(structErr)
}

// fetchSchemasFromDB queries information_schema.schemata and returns the list of schema names.
func fetchSchemasFromDB(ctx context.Context, conn *sql.DB) ([]string, error) {
	rows, err := conn.QueryContext(ctx,
		`SELECT schema_name FROM information_schema.schemata 
		 WHERE schema_name NOT IN ('pg_catalog', 'information_schema') 
		 AND schema_name NOT LIKE 'pg_%' 
		 ORDER BY schema_name`)
	if err != nil {
		return nil, fmt.Errorf("failed to query schemas: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var schemas []string
	for rows.Next() {
		var schema string
		if err := rows.Scan(&schema); err != nil {
			return nil, fmt.Errorf("failed to scan schema: %w", err)
		}
		schemas = append(schemas, schema)
	}
	return schemas, nil
}

// resolveDefaultSchema determines the default schema based on DB type.
func resolveDefaultSchema(ctx context.Context, conn *sql.DB, dbType, databaseName string) string {
	if dbType == "postgres" {
		var schema string
		if err := conn.QueryRowContext(ctx, "SELECT current_schema()").Scan(&schema); err != nil {
			return "public"
		}
		return schema
	}
	// MySQL/MariaDB
	return databaseName
}

func (s *MCPServer) handleListSchemas(ctx context.Context, _ *mcp.CallToolRequest, input ListSchemasParams) (*mcp.CallToolResult, any, error) {
	if input.ProfileName == "" || input.DatabaseName == "" {
		return nil, nil, fmt.Errorf("profile_name and database_name are required")
	}

	cfg, prof, err := s.findProfile(input.ProfileName)
	if err != nil {
		return nil, nil, err
	}

	conn, errResult := s.openExecuteSQLConnection(ctx, input.ProfileName, input.DatabaseName, cfg.MaxPoolSize, prof)
	if errResult != nil {
		return errResult, nil, nil
	}
	defer conn.Close() //nolint:errcheck

	var schemas []string
	var defaultSchema string

	if prof.DBType == "sqlite" {
		schemas = []string{"main"}
		defaultSchema = "main"
	} else {
		schemas, err = fetchSchemasFromDB(ctx, conn)
		if err != nil {
			return nil, nil, err
		}
		defaultSchema = resolveDefaultSchema(ctx, conn, prof.DBType, input.DatabaseName)
	}

	result := ListSchemasResult{
		Schemas:       schemas,
		DefaultSchema: defaultSchema,
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(mustJSONMarshal(result)),
			},
		},
	}, nil, nil
}

func (s *MCPServer) handleGetSearchPath(ctx context.Context, _ *mcp.CallToolRequest, input GetSearchPathParams) (*mcp.CallToolResult, any, error) {
	if input.ProfileName == "" || input.DatabaseName == "" {
		return nil, nil, fmt.Errorf("profile_name and database_name are required")
	}

	cfg, prof, err := s.findProfile(input.ProfileName)
	if err != nil {
		return nil, nil, err
	}

	conn, errResult := s.openExecuteSQLConnection(ctx, input.ProfileName, input.DatabaseName, cfg.MaxPoolSize, prof)
	if errResult != nil {
		return errResult, nil, nil
	}
	defer conn.Close() //nolint:errcheck

	var searchPath, currentSchema string
	if prof.DBType == "postgres" {
		if err := conn.QueryRowContext(ctx, "SHOW search_path").Scan(&searchPath); err != nil {
			searchPath = "unknown"
		}
		if err := conn.QueryRowContext(ctx, "SELECT current_schema()").Scan(&currentSchema); err != nil {
			currentSchema = "unknown"
		}
	} else {
		searchPath = input.DatabaseName
		currentSchema = input.DatabaseName
	}

	result := GetSearchPathResult{
		SearchPath:               searchPath,
		CurrentSchema:            currentSchema,
		ConnectionPoolingWarning: "This server uses connection pooling. Schema should be explicitly qualified in queries.",
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(mustJSONMarshal(result)),
			},
		},
	}, nil, nil
}

func (s *MCPServer) resolveDescribeTableProfile(profileName string) (*config.Config, *config.Profile, *mcp.CallToolResult, error) {
	cfg, prof, err := s.findProfile(profileName)
	if err == nil {
		return cfg, prof, nil, nil
	}
	if cfg == nil {
		structErr := NewStructuredError(
			ErrorCodeConfigNotFound,
			messageFailedToLoadConfiguration,
			err.Error(),
		).WithSuggestions(
			ErrorSuggestion{
				Action:      actionInitializeConfiguration,
				Description: descriptionRunServerCreateConfig,
			},
		)
		return nil, nil, errorResult(structErr), nil
	}
	return nil, nil, nil, errors.New(errorProfileNotFound)
}

func (s *MCPServer) queryDescribeTableColumns(
	ctx context.Context,
	conn *sql.DB,
	p DescribeTableParams,
	dbType string,
) ([]ColumnInfo, *mcp.CallToolResult, error) {
	switch dbType {
	case "mysql", "mariadb":
		columns, err := describeMySQLTableColumns(ctx, conn, p.DatabaseName, p.TableName)
		return columns, describeTableErrorResult(s.errorAnalyzer, err, p, dbType), nil
	case "postgres":
		columns, err := describePostgresTableColumns(ctx, conn, p.TableName, p.Schema)
		return columns, describeTableErrorResult(s.errorAnalyzer, err, p, dbType), nil
	case "sqlite":
		columns, err := describeSQLiteTableColumns(ctx, conn, p.TableName)
		return columns, describeTableErrorResult(s.errorAnalyzer, err, p, dbType), nil
	default:
		return nil, nil, errors.New(errorUnsupportedDBType)
	}
}

func describeTableErrorResult(analyzer *ErrorAnalyzer, err error, p DescribeTableParams, dbType string) *mcp.CallToolResult {
	if err == nil {
		return nil
	}
	log.JSONLog("error", "Failed to describe table", map[string]interface{}{"table": p.TableName, "error": err})
	structErr := analyzer.AnalyzeError(err, map[string]interface{}{
		"profile_name":  p.ProfileName,
		"database_name": p.DatabaseName,
		"table_name":    p.TableName,
		"schema":        p.Schema,
		"operation":     "describe_table",
		"db_type":       dbType,
	})
	return errorResult(structErr)
}

func describeMySQLTableColumns(ctx context.Context, conn *sql.DB, databaseName, tableName string) ([]ColumnInfo, error) {
	query := `SELECT
		COLUMN_NAME as name,
		COLUMN_TYPE as type,
		IS_NULLABLE as nullable,
		COLUMN_KEY as key_type,
		COLUMN_DEFAULT as default_value,
		COLUMN_COMMENT as comment,
		EXTRA as extra,
		CHARACTER_SET_NAME as character_set,
		COLLATION_NAME as collation,
		CHARACTER_MAXIMUM_LENGTH as max_length,
		NUMERIC_PRECISION as numeric_precision,
		NUMERIC_SCALE as numeric_scale
	FROM INFORMATION_SCHEMA.COLUMNS
	WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
	ORDER BY ORDINAL_POSITION`

	rows, err := conn.QueryContext(ctx, query, databaseName, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck // Standard pattern: error in deferred close is not critical

	columns := make([]ColumnInfo, 0)
	for rows.Next() {
		row, err := scanMySQLDescribeColumnRow(rows)
		if err != nil {
			return nil, err
		}
		columns = append(columns, buildMySQLDescribeColumnInfo(row))
	}
	return columns, nil
}

type mysqlDescribeColumnRow struct {
	name, typ, nullable, keyType, extra string
	defaultVal, comment, characterSet   sql.NullString
	collation                           sql.NullString
	maxLength, precision, scale         sql.NullInt64
}

func scanMySQLDescribeColumnRow(rows *sql.Rows) (mysqlDescribeColumnRow, error) {
	row := mysqlDescribeColumnRow{}
	err := rows.Scan(
		&row.name, &row.typ, &row.nullable, &row.keyType, &row.defaultVal, &row.comment,
		&row.extra, &row.characterSet, &row.collation, &row.maxLength, &row.precision, &row.scale,
	)
	if err != nil {
		return mysqlDescribeColumnRow{}, err
	}
	return row, nil
}

func buildMySQLDescribeColumnInfo(row mysqlDescribeColumnRow) ColumnInfo {
	col := ColumnInfo{
		Name:          row.name,
		Type:          row.typ,
		Nullable:      row.nullable == "YES",
		Key:           row.keyType,
		Extra:         row.extra,
		AutoIncrement: strings.Contains(row.extra, "auto_increment"),
	}
	setMySQLDescribeColumnOptionalFields(&col, row)
	return col
}

func setMySQLDescribeColumnOptionalFields(col *ColumnInfo, row mysqlDescribeColumnRow) {
	if row.defaultVal.Valid {
		col.Default = &row.defaultVal.String
	}
	if row.comment.Valid {
		col.Comment = row.comment.String
	}
	if row.characterSet.Valid {
		col.CharacterSet = row.characterSet.String
	}
	if row.collation.Valid {
		col.Collation = row.collation.String
	}
	if row.maxLength.Valid {
		col.MaxLength = &row.maxLength.Int64
	}
	if row.precision.Valid {
		col.Precision = &row.precision.Int64
	}
	if row.scale.Valid {
		col.Scale = &row.scale.Int64
	}
}

// postgresColumnRow holds the scanned fields for a PostgreSQL column from information_schema.
type postgresColumnRow struct {
	name, typ, nullable, keyType string
	defaultVal, comment          sql.NullString
	maxLength, precision, scale  sql.NullInt64
}

// mapPostgresColumn maps scanned PostgreSQL column fields into a ColumnInfo struct.
func mapPostgresColumn(row postgresColumnRow) ColumnInfo {
	col := ColumnInfo{
		Name:          row.name,
		Type:          row.typ,
		Nullable:      row.nullable == "YES",
		Key:           row.keyType,
		AutoIncrement: strings.Contains(row.defaultVal.String, "nextval"),
	}
	if row.defaultVal.Valid {
		col.Default = &row.defaultVal.String
	}
	if row.comment.Valid {
		col.Comment = row.comment.String
	}
	if row.maxLength.Valid {
		col.MaxLength = &row.maxLength.Int64
	}
	if row.precision.Valid {
		col.Precision = &row.precision.Int64
	}
	if row.scale.Valid {
		col.Scale = &row.scale.Int64
	}
	return col
}

func describePostgresTableColumns(ctx context.Context, conn *sql.DB, tableName, schema string) ([]ColumnInfo, error) {
	// Resolve schema using ResolveSchema with auto-detection fallback
	sqlConn, err := conn.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}
	defer sqlConn.Close() //nolint:errcheck // Standard pattern: error in deferred close is not critical

	resolvedSchema, err := ResolveSchema(ctx, sqlConn, schema)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve schema: %w", err)
	}

	// Remove quotes for use as a parameter value (ResolveSchema returns quoted identifier)
	resolvedSchema = strings.Trim(resolvedSchema, "\"")

	query := `SELECT
		c.column_name as name,
		c.data_type as type,
		c.is_nullable as nullable,
		c.column_default as default_value,
		COALESCE(pgd.description, '') as comment,
		c.character_maximum_length,
		c.numeric_precision,
		c.numeric_scale,
		CASE
			WHEN tc.constraint_type = 'PRIMARY KEY' THEN 'PRI'
			WHEN tc.constraint_type = 'UNIQUE' THEN 'UNI'
			WHEN tc.constraint_type = 'FOREIGN KEY' THEN 'MUL'
			ELSE ''
		END as key_type
	FROM information_schema.columns c
	LEFT JOIN pg_class pgc ON pgc.relname = c.table_name
	LEFT JOIN pg_namespace pgn ON pgn.oid = pgc.relnamespace AND pgn.nspname = c.table_schema
	LEFT JOIN pg_attribute pga ON pga.attrelid = pgc.oid AND pga.attname = c.column_name
	LEFT JOIN pg_description pgd ON pgd.objoid = pgc.oid AND pgd.objsubid = pga.attnum
	LEFT JOIN information_schema.key_column_usage kcu ON kcu.column_name = c.column_name
		AND kcu.table_name = c.table_name AND kcu.table_schema = c.table_schema
	LEFT JOIN information_schema.table_constraints tc ON tc.constraint_name = kcu.constraint_name
		AND tc.table_name = c.table_name AND tc.table_schema = c.table_schema
	WHERE c.table_schema = $2 AND c.table_name = $1
	ORDER BY c.ordinal_position`

	rows, err := conn.QueryContext(ctx, query, tableName, resolvedSchema)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck // Standard pattern: error in deferred close is not critical

	columns := make([]ColumnInfo, 0)
	for rows.Next() {
		var row postgresColumnRow
		if err := rows.Scan(&row.name, &row.typ, &row.nullable, &row.defaultVal, &row.comment, &row.maxLength, &row.precision, &row.scale, &row.keyType); err != nil {
			return nil, err
		}
		columns = append(columns, mapPostgresColumn(row))
	}
	return columns, nil
}

func describeSQLiteTableColumns(ctx context.Context, conn *sql.DB, tableName string) ([]ColumnInfo, error) {
	query := fmt.Sprintf("PRAGMA table_xinfo('%s')", tableName)
	rows, err := conn.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck // Standard pattern: error in deferred close is not critical

	columns := make([]ColumnInfo, 0)
	for rows.Next() {
		var cid, notnull, pk, hidden int
		var name, typ string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk, &hidden); err != nil {
			return nil, err
		}
		col := ColumnInfo{
			Name:          name,
			Type:          typ,
			Nullable:      notnull == 0,
			AutoIncrement: false,
			Comment:       "",
		}
		if pk > 0 {
			col.Key = "PRI"
		}
		if dflt.Valid {
			col.Default = &dflt.String
		}
		columns = append(columns, col)
	}
	return columns, nil
}

func (s *MCPServer) handleListDatabases(ctx context.Context, _ *mcp.CallToolRequest, input ListDatabasesParams) (*mcp.CallToolResult, any, error) {
	p := input
	cfg, prof, err := s.findProfile(p.ProfileName)
	if err != nil {
		log.JSONLog("error", messageFailedToLoadConfiguration, map[string]interface{}{"error": err})
		if cfg == nil {
			structErr := NewStructuredError(
				ErrorCodeConfigNotFound,
				messageFailedToLoadConfiguration,
				err.Error(),
			).WithSuggestions(
				ErrorSuggestion{
					Action:      actionInitializeConfiguration,
					Description: descriptionRunServerCreateConfig,
				},
			)
			return errorResult(structErr), nil, nil
		}
		log.JSONLog("error", messageProfileNotFound, map[string]interface{}{"profile_name": p.ProfileName})
		structErr := NewStructuredError(
			ErrorCodeProfileNotFound,
			fmt.Sprintf("Profile '%s' not found", p.ProfileName),
			descriptionSpecifiedProfileMissing,
		).WithSuggestions(
			ErrorSuggestion{
				Tool:        toolListProfiles,
				Description: "List all available database profiles",
			},
			ErrorSuggestion{
				Tool:        toolConfigureProfile,
				Description: "Create a new database profile",
				Example:     fmt.Sprintf(`{"profile_name": "%s", "db_type": "mysql", ...}`, p.ProfileName),
			},
		)
		return errorResult(structErr), nil, nil
	}
	dsn := db.DSN(prof.DBType, prof.Host, prof.Port, prof.Username, prof.Password, prof.DatabaseName, prof.SSLMode)
	conn, err := db.OpenConnectionWithPool(ctx, prof.DBType, dsn, cfg.MaxPoolSize)
	if err != nil {
		log.JSONLog("error", messageFailedToConnectToDatabase, map[string]interface{}{"error": err})
		structErr := s.errorAnalyzer.AnalyzeError(err, map[string]interface{}{
			"profile_name": p.ProfileName,
			"operation":    "smart_query_builder",
			"db_type":      prof.DBType,
		})
		return errorResult(structErr), nil, nil
	}
	defer conn.Close() //nolint:errcheck // Standard pattern: error in deferred close is not critical
	var query string
	switch prof.DBType {
	case "mysql", "mariadb":
		query = "SHOW DATABASES"
	case "postgres":
		query = "SELECT datname FROM pg_database WHERE datistemplate = false"
	case "sqlite":
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: string(mustJSONMarshal(ListDatabasesResult{
						Databases: []string{prof.DatabaseName},
					})),
				},
			},
		}, nil, nil
	default:
		return nil, nil, errors.New(errorUnsupportedDBType)
	}
	rows, err := conn.QueryContext(ctx, query)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close() //nolint:errcheck // Standard pattern: error in deferred close is not critical
	var dbs []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, nil, err
		}
		dbs = append(dbs, name)
	}
	result := ListDatabasesResult{Databases: dbs}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(mustJSONMarshal(result)),
			},
		},
	}, nil, nil
}

/*
handleSampleData implements the MCP handler for fetching sample rows from tables.

It extracts sample rows to help AI/agents infer data types, formats, and value ranges.
Supports MySQL, MariaDB, PostgreSQL, and SQLite with configurable sample size.

Input: SampleDataParams (profile_name, table_name required; database_name, sample_size optional)
Output: SampleDataResult (sample rows with column names and metadata)
*/
func (s *MCPServer) handleSampleData(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input SampleDataParams,
) (*mcp.CallToolResult, any, error) {
	p := input
	if errResult := validateSampleDataParams(p); errResult != nil {
		return errResult, nil, fmt.Errorf("missing required parameters")
	}

	cfg, prof, errResult, err := s.resolveSampleDataProfile(p)
	if errResult != nil || err != nil {
		return errResult, nil, err
	}

	sampleSize := normalizeSampleSize(p.SampleSize)
	dbName := selectedDatabaseName(prof.DatabaseName, p.DatabaseName)
	conn, errResult := s.openSampleDataConnection(ctx, p.ProfileName, dbName, cfg.MaxPoolSize, prof)
	if errResult != nil {
		return errResult, nil, nil
	}
	defer conn.Close() //nolint:errcheck // Standard pattern: error in deferred close is not critical

	if result := s.switchMySQLDatabase(ctx, conn, prof, p.ProfileName, p.DatabaseName); result != nil {
		return result, nil, nil
	}

	sampleQuery, errResult, err := buildSampleQuery(prof.DBType, p.TableName, sampleSize)
	if errResult != nil || err != nil {
		return errResult, nil, err
	}

	columns, sampleRows, errResult, err := s.fetchSampleRows(ctx, conn, p, dbName, prof.DBType, sampleQuery)
	if errResult != nil || err != nil {
		return errResult, nil, err
	}

	actualSize := len(sampleRows)
	summary := fmt.Sprintf("Retrieved %d sample row(s) from table '%s' with %d column(s).", actualSize, p.TableName, len(columns))

	result := SampleDataResult{
		TableName:  p.TableName,
		SampleSize: actualSize,
		Columns:    columns,
		SampleRows: sampleRows,
		Summary:    summary,
	}

	log.JSONLog("info", "Sample data retrieved successfully", map[string]interface{}{
		"table":   p.TableName,
		"rows":    actualSize,
		"columns": len(columns),
	})

	b, _ := json.Marshal(result)
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(b),
			},
		},
	}, nil, nil
}

func validateSampleDataParams(p SampleDataParams) *mcp.CallToolResult {
	if p.ProfileName != "" && p.DatabaseName != "" && p.TableName != "" {
		return nil
	}
	structErr := NewStructuredError(
		ErrorCodeMissingParameter,
		messageMissingRequiredParameters,
		"All of profile_name, database_name, and table_name are required",
	).WithSuggestions(
		ErrorSuggestion{
			Action:      actionProvideAllRequiredParameters,
			Description: "Ensure profile_name, database_name, and table_name are included",
			Example:     `{"profile_name": "mydb", "database_name": "mydb", "table_name": "users", "sample_size": 5}`,
		},
	)
	return errorResult(structErr)
}

func (s *MCPServer) resolveSampleDataProfile(
	p SampleDataParams,
) (*config.Config, *config.Profile, *mcp.CallToolResult, error) {
	cfg, prof, err := s.findProfile(p.ProfileName)
	if err != nil {
		if cfg == nil {
			log.JSONLog("error", "Failed to load config for sample data", map[string]interface{}{"error": err.Error()})
			structErr := NewStructuredError(
				ErrorCodeConfigNotFound,
				messageFailedToLoadConfiguration,
				err.Error(),
			).WithSuggestions(
				ErrorSuggestion{
					Action:      actionInitializeConfiguration,
					Description: descriptionRunServerCreateConfig,
				},
			)
			return nil, nil, errorResult(structErr), nil
		}
		log.JSONLog("error", "Profile not found for sample data", map[string]interface{}{"profile_name": p.ProfileName})
		return nil, nil, nil, errors.New(errorProfileNotFound)
	}
	if isSupportedSampleDataType(prof.DBType) {
		return cfg, prof, nil, nil
	}
	log.JSONLog("error", "Unsupported database type for sample data", map[string]interface{}{"db_type": prof.DBType})
	structErr := NewStructuredError(
		ErrorCodeUnsupportedDB,
		fmt.Sprintf("Unsupported database type: %s", prof.DBType),
		"Sample data is only supported for MySQL, MariaDB, PostgreSQL, and SQLite",
	).WithContext("supported_types", []string{"mysql", "mariadb", "postgres", "sqlite"})
	return nil, nil, errorResult(structErr), fmt.Errorf("unsupported db_type for sample data")
}

func isSupportedSampleDataType(dbType string) bool {
	return dbType == "mysql" || dbType == "mariadb" || dbType == "postgres" || dbType == "sqlite"
}

func normalizeSampleSize(size int) int {
	if size <= 0 {
		return 3
	}
	if size > 100 {
		return 100
	}
	return size
}

func (s *MCPServer) openSampleDataConnection(
	ctx context.Context,
	profileName, databaseName string,
	maxPoolSize int,
	prof *config.Profile,
) (*sql.DB, *mcp.CallToolResult) {
	conn, errResult := s.openExecuteSQLConnection(ctx, profileName, databaseName, maxPoolSize, prof)
	if errResult != nil {
		return nil, errResult
	}
	return conn, nil
}

func buildSampleQuery(dbType, tableName string, sampleSize int) (string, *mcp.CallToolResult, error) {
	switch dbType {
	case "mysql", "mariadb":
		return fmt.Sprintf("SELECT * FROM `%s` LIMIT %d", tableName, sampleSize), nil, nil
	case "postgres":
		return fmt.Sprintf("SELECT * FROM \"%s\" LIMIT %d", tableName, sampleSize), nil, nil
	case "sqlite":
		return fmt.Sprintf("SELECT * FROM '%s' LIMIT %d", tableName, sampleSize), nil, nil
	default:
		structErr := NewStructuredError(
			ErrorCodeUnsupportedDB,
			fmt.Sprintf("Unsupported database type: %s", dbType),
			"Sample data is only supported for MySQL, MariaDB, PostgreSQL, and SQLite",
		).WithContext("supported_types", []string{"mysql", "mariadb", "postgres", "sqlite"})
		return "", errorResult(structErr), fmt.Errorf("unsupported db_type for sample data")
	}
}

func (s *MCPServer) fetchSampleRows(
	ctx context.Context,
	conn *sql.DB,
	p SampleDataParams,
	dbName, dbType, sampleQuery string,
) ([]string, [][]interface{}, *mcp.CallToolResult, error) {
	log.JSONLog("debug", "Executing sample data query", map[string]interface{}{"query": sampleQuery, "table": p.TableName})
	rows, err := conn.QueryContext(ctx, sampleQuery)
	if err != nil {
		log.JSONLog("error", "Sample data query failed", map[string]interface{}{"query": sampleQuery, "error": err.Error()})
		structErr := s.errorAnalyzer.AnalyzeError(err, map[string]interface{}{
			"profile_name":  p.ProfileName,
			"table_name":    p.TableName,
			"database_name": dbName,
			"operation":     "sample_data",
			"db_type":       dbType,
			"query":         sampleQuery,
		})
		return nil, nil, errorResult(structErr), nil
	}
	defer rows.Close() //nolint:errcheck // Standard pattern: error in deferred close is not critical

	columns, err := rows.Columns()
	if err != nil {
		log.JSONLog("error", "Failed to get column names for sample data", map[string]interface{}{"table": p.TableName, "error": err.Error()})
		return nil, nil, nil, err
	}

	sampleRows, err := scanSampleDataRows(rows, p.TableName)
	if err != nil {
		return nil, nil, nil, err
	}
	return columns, sampleRows, nil, nil
}

func scanSampleDataRows(rows *sql.Rows, tableName string) ([][]interface{}, error) {
	sampleRows := make([][]interface{}, 0)
	columns, _ := rows.Columns()
	for rows.Next() {
		row := make([]interface{}, len(columns))
		ptrs := make([]interface{}, len(columns))
		for idx := range row {
			ptrs[idx] = &row[idx]
		}
		if err := rows.Scan(ptrs...); err != nil {
			log.JSONLog("error", "Failed to scan sample data row", map[string]interface{}{"table": tableName, "error": err.Error()})
			return nil, err
		}
		for idx, value := range row {
			if bytes, ok := value.([]byte); ok {
				row[idx] = string(bytes)
			}
		}
		sampleRows = append(sampleRows, row)
	}
	if err := rows.Err(); err != nil {
		log.JSONLog("error", "Error during sample data iteration", map[string]interface{}{"table": tableName, "error": err.Error()})
		return nil, err
	}
	return sampleRows, nil
}

// handleAnalyzeSchema implements the MCP handler for comprehensive schema analysis.
// Delegates core analysis to the analyze.Run() pure function.
func (s *MCPServer) handleAnalyzeSchema(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input AnalyzeSchemaParams,
) (*mcp.CallToolResult, any, error) {
	p := input
	if errResult, err := validateAnalyzeSchemaParams(p); errResult != nil || err != nil {
		return errResult, nil, err
	}

	cfg, prof, dbName, errResult, err := s.resolveAnalyzeSchemaProfile(p)
	if errResult != nil || err != nil {
		return errResult, nil, err
	}

	conn, errResult, err := s.openAnalyzeSchemaConnection(ctx, p, cfg, prof, dbName)
	if errResult != nil || err != nil {
		return errResult, nil, err
	}
	defer conn.Close() //nolint:errcheck // Standard pattern: error in deferred close is not critical

	tables, errResult, err := s.listAnalyzeSchemaTables(ctx, conn, p, prof, dbName)
	if errResult != nil || err != nil {
		return errResult, nil, err
	}
	filteredTables := filterAnalyzeSchemaTables(tables, p.IncludeTables, p.ExcludeTables)

	schema := resolveSchemaForAnalyze(ctx, conn, p.Schema, prof.DBType, dbName)

	// Delegate core analysis to the pure analyze.Run() function
	result, runErr := analyze.Run(ctx, conn, prof.DBType, schema, p, filteredTables)
	if runErr != nil {
		log.JSONLog("error", "analyze.Run failed", map[string]interface{}{"error": runErr.Error()})
		structErr := NewStructuredError(
			ErrorCodeInternalError,
			"Schema analysis failed",
			runErr.Error(),
		)
		return errorResult(structErr), nil, runErr
	}

	// Build server-side result (merges analyze output with server-specific fields)
	serverResult := s.buildAnalyzeSchemaResultFromAnalyze(result, prof.DBType, filteredTables)

	// Detect privilege issues: tables found but no column data returned
	if w := buildPrivilegeWarning(prof.DBType, len(filteredTables), len(result.TableSchemas), dbName, schema); w != "" {
		serverResult.Warnings = append(serverResult.Warnings, w)
	}

	// Query suggestions — calls MCP tools, can't be pure
	aiQuerySuggestions := s.buildAnalyzeSchemaQuerySuggestions(ctx, p, filteredTables, dbName)
	serverResult.AIQuerySuggestions = aiQuerySuggestions

	// Profiling — server-side concern
	if p.Profiling {
		sampleDataMap := fetchAllSampleRowsForProfiling(ctx, conn, filteredTables, prof.DBType, p.SampleSize)
		enhanced := enhanceSchemaAnalysis(ctx, serverResult.TableSchemas, sampleDataMap, defaultProfilingWorkers)
		if enhanced != nil {
			mergeWithExistingSchema(&serverResult, enhanced)
		}
	}

	resultContent, marshalErr := marshalAnalyzeSchemaResult(serverResult)
	if marshalErr != nil {
		return nil, nil, marshalErr
	}
	return resultContent, nil, nil
}

// resolveSchemaForAnalyze determines the schema name for the analyze package.
// For PostgreSQL it resolves the default schema (e.g. "public") if none was specified.
// For MySQL/MariaDB it returns the database name when no explicit schema is provided.
// SQLite has no schema concept, so it returns empty string.
func resolveSchemaForAnalyze(ctx context.Context, conn *sql.DB, schema, dbType, databaseName string) string {
	if schema != "" {
		return schema
	}
	switch dbType {
	case "mysql", "mariadb":
		return databaseName
	case "postgres", "postgresql":
		dbConn, err := conn.Conn(ctx)
		if err != nil {
			log.JSONLog("warn", "Failed to get connection for schema resolution", map[string]interface{}{"error": err.Error()})
			return "public"
		}
		resolved, err := GetDefaultSchema(ctx, dbConn)
		_ = dbConn.Close()
		if err != nil {
			log.JSONLog("warn", "Failed to resolve default schema, using public", map[string]interface{}{"error": err.Error()})
			return "public"
		}
		return resolved
	default:
		// SQLite and others: no schema concept
		return ""
	}
}

// buildPrivilegeWarning generates a warning when tables were found but no column data
// was returned, which typically indicates insufficient database privileges.
func buildPrivilegeWarning(dbType string, tableCount, resultColumnCount int, dbName, schema string) string {
	if tableCount == 0 || resultColumnCount > 0 {
		return ""
	}
	switch dbType {
	case "mysql", "mariadb":
		return fmt.Sprintf(
			"Found %d tables but no column data. The database user may lack SELECT privilege on %s. "+
				"Grant with: GRANT SELECT ON %s.* TO 'user'@'host'",
			tableCount, dbName, dbName,
		)
	case "postgres", "postgresql":
		schemaHint := schema
		if schemaHint == "" {
			schemaHint = "public"
		}
		return fmt.Sprintf(
			"Found %d tables but no column data. The database user may lack USAGE privilege on schema %s. "+
				"Grant with: GRANT USAGE ON SCHEMA %s TO \"user\"",
			tableCount, schemaHint, schemaHint,
		)
	default:
		return fmt.Sprintf("Found %d tables but no column data — this may indicate insufficient database privileges", tableCount)
	}
}

// fetchAllSampleRowsForProfiling fetches sample rows for profiling (kept in server.go
// because profiling is a server-side concern).
func fetchAllSampleRowsForProfiling(ctx context.Context, conn *sql.DB, tableNames []string, dbType string, sampleSize int) map[string][]map[string]interface{} {
	sampleDataMap := make(map[string][]map[string]interface{})
	for _, table := range tableNames {
		sampleDataMap[table] = fetchAnalyzeSchemaSampleRows(ctx, conn, table, dbType, normalizeAnalyzeSchemaSampleSize(sampleSize))
	}
	return sampleDataMap
}

// buildAnalyzeSchemaResultFromAnalyze converts the analyze.Run() result into the
// server's AnalyzeSchemaResult, adding server-specific fields.
func (s *MCPServer) buildAnalyzeSchemaResultFromAnalyze(analyzeResult *analyze.AnalyzeSchemaResult, _ string, filteredTables []string) AnalyzeSchemaResult {
	result := AnalyzeSchemaResult{
		AnalysisMetadata:        analyzeResult.AnalysisMetadata,
		DatabaseOverview:        analyzeResult.DatabaseOverview,
		TableCatalog:            analyzeResult.TableCatalog,
		TableSchemas:            analyzeResult.TableSchemas,
		RelationshipGraph:       analyzeResult.RelationshipGraph,
		RelationshipGraphVisual: s.buildRelationshipGraph(analyzeResult.RelationshipGraph),
		BusinessContext:         analyzeResult.BusinessContext,
		DataQualityMetrics:      analyzeResult.DataQualityMetrics,
		PerformanceOptimization: analyzeResult.PerformanceOptimization,
		ClassificationSignals:   analyzeResult.ClassificationSignals,
		QuickInsights:           []string{fmt.Sprintf("Schema analysis completed for %d tables.", len(filteredTables))},
		Warnings:                analyzeResult.Warnings,
	}
	return result
}

func (s *MCPServer) buildAnalyzeSchemaQuerySuggestions(
	ctx context.Context,
	p AnalyzeSchemaParams,
	filteredTables []string,
	dbName string,
) AIQuerySuggestions {
	var suggestions AIQuerySuggestions
	if p.AnalysisLevel != AnalysisLevelComprehensive || !p.IncludeQueries {
		return suggestions
	}
	for _, tableName := range filteredTables {
		question := fmt.Sprintf("Show all rows in %s", tableName)
		suggestion, err := s.generateQuerySuggestionViaSmartBuilder(ctx, nil, p.ProfileName, question, dbName, []string{tableName})
		if err != nil || suggestion == nil {
			continue
		}
		suggestions.DataExploration = append(suggestions.DataExploration, QuerySuggestion{
			Category:   "exploration",
			Question:   question,
			SQL:        suggestion.SQL,
			Complexity: "easy",
		})
	}
	return suggestions
}

func validateAnalyzeSchemaParams(p AnalyzeSchemaParams) (*mcp.CallToolResult, error) {
	if p.Validate() == nil {
		return nil, nil
	}
	structErr := NewStructuredError(
		ErrorCodeMissingParameter,
		"Missing or invalid required parameter",
		"analysis_level is required and must be one of: basic, detailed, comprehensive",
	).WithSuggestions(
		ErrorSuggestion{
			Action:      "Specify analysis_level",
			Description: "analysis_level is required and must be one of: basic, detailed, comprehensive",
			Example:     `{"profile_name": "analytics_db", "analysis_level": "detailed"}`,
		},
	)
	return errorResult(structErr), errors.New("missing or invalid analysis_level")
}

func (s *MCPServer) resolveAnalyzeSchemaProfile(
	p AnalyzeSchemaParams,
) (*config.Config, *config.Profile, string, *mcp.CallToolResult, error) {
	cfg, prof, err := s.findProfile(p.ProfileName)
	if err == nil {
		dbName := selectedDatabaseName(prof.DatabaseName, p.DatabaseName)
		return cfg, prof, dbName, nil, nil
	}
	if cfg == nil {
		structErr := NewStructuredError(
			ErrorCodeConfigNotFound,
			messageFailedToLoadConfiguration,
			err.Error(),
		)
		return nil, nil, "", errorResult(structErr), err
	}
	structErr := NewStructuredError(
		ErrorCodeProfileNotFound,
		fmt.Sprintf(messageProfileNotFoundFormat, p.ProfileName),
		descriptionSpecifiedProfileMissing,
	)
	return nil, nil, "", errorResult(structErr), errors.New(errorProfileNotFound)
}

func (s *MCPServer) openAnalyzeSchemaConnection(
	ctx context.Context,
	p AnalyzeSchemaParams,
	cfg *config.Config,
	prof *config.Profile,
	dbName string,
) (*sql.DB, *mcp.CallToolResult, error) {
	dsn := db.DSN(prof.DBType, prof.Host, prof.Port, prof.Username, prof.Password, dbName, prof.SSLMode)
	conn, err := db.OpenConnectionWithPool(ctx, prof.DBType, dsn, cfg.MaxPoolSize)
	if err == nil {
		return conn, nil, nil
	}
	log.JSONLog("error", messageFailedToConnectToDatabase, map[string]interface{}{"error": err})
	structErr := s.errorAnalyzer.AnalyzeError(err, map[string]interface{}{
		"profile_name": p.ProfileName,
		"operation":    "analyze_schema",
		"db_type":      prof.DBType,
	})
	return nil, errorResult(structErr), err
}

func (s *MCPServer) listAnalyzeSchemaTables(
	ctx context.Context,
	conn *sql.DB,
	p AnalyzeSchemaParams,
	prof *config.Profile,
	dbName string,
) ([]string, *mcp.CallToolResult, error) {
	tableRefs, errResult, err := s.queryTableNames(ctx, conn, p.ProfileName, dbName, prof.DBType)
	if errResult != nil || err != nil {
		return nil, errResult, err
	}
	// Extract just the table names for compatibility
	tableNames := make([]string, len(tableRefs))
	for i, ref := range tableRefs {
		tableNames[i] = ref.Name
	}
	return tableNames, nil, nil
}

func filterAnalyzeSchemaTables(tables, includeTables, excludeTables []string) []string {
	includeSet := buildTableFilterSet(includeTables)
	excludeSet := buildTableFilterSet(excludeTables)

	filteredTables := make([]string, 0, len(tables))
	for _, tableName := range tables {
		tableLower := strings.ToLower(tableName)
		if len(includeSet) > 0 && !includeSet[tableLower] {
			continue
		}
		if excludeSet[tableLower] {
			continue
		}
		filteredTables = append(filteredTables, tableName)
	}
	if len(filteredTables) == 0 {
		return tables
	}
	return filteredTables
}

// fetchAnalyzeSchemaSampleRows fetches sample rows from a table for profiling.
// Kept in server.go because profiling is a server-side concern.
func fetchAnalyzeSchemaSampleRows(
	ctx context.Context,
	conn *sql.DB,
	tableName,
	dbType string,
	sampleSize int,
) []map[string]interface{} {
	sampleQuery, ok := analyzeSchemaSampleQuery(dbType, tableName, sampleSize)
	if !ok {
		return []map[string]interface{}{}
	}
	rows, err := conn.QueryContext(ctx, sampleQuery)
	if err != nil {
		log.JSONLog("warn", "Failed to fetch sample rows during analysis", map[string]interface{}{
			"table": tableName,
			"error": err.Error(),
			"query": sampleQuery,
		})
		return []map[string]interface{}{}
	}
	defer rows.Close() //nolint:errcheck // Standard pattern: error in deferred close is not critical
	return scanAnalyzeSchemaSampleRows(rows, tableName)
}

func analyzeSchemaSampleQuery(dbType, tableName string, sampleSize int) (string, bool) {
	switch dbType {
	case "mysql", "mariadb":
		return fmt.Sprintf("SELECT * FROM `%s` LIMIT %d", tableName, sampleSize), true
	case "postgres":
		return fmt.Sprintf("SELECT * FROM \"%s\" LIMIT %d", tableName, sampleSize), true
	case "sqlite":
		return fmt.Sprintf("SELECT * FROM '%s' LIMIT %d", tableName, sampleSize), true
	default:
		return "", false
	}
}

func scanAnalyzeSchemaSampleRows(rows *sql.Rows, tableName string) []map[string]interface{} {
	sampleRows := make([]map[string]interface{}, 0)
	columns, err := rows.Columns()
	if err != nil {
		log.JSONLog("warn", "Failed to get sample row columns during analysis", map[string]interface{}{
			"table": tableName,
			"error": err.Error(),
		})
		return sampleRows
	}
	for rows.Next() {
		row := make([]interface{}, len(columns))
		ptrs := make([]interface{}, len(columns))
		for idx := range row {
			ptrs[idx] = &row[idx]
		}
		if err := rows.Scan(ptrs...); err != nil {
			log.JSONLog("warn", "Failed to scan sample row during analysis", map[string]interface{}{
				"table": tableName,
				"error": err.Error(),
			})
			continue
		}
		sampleRows = append(sampleRows, normalizeAnalyzeSchemaSampleRow(columns, row))
	}
	if err := rows.Err(); err != nil {
		log.JSONLog("warn", "Iteration error while reading sample rows during analysis", map[string]interface{}{
			"table": tableName,
			"error": err.Error(),
		})
	}
	return sampleRows
}

func normalizeAnalyzeSchemaSampleRow(columns []string, row []interface{}) map[string]interface{} {
	rowMap := make(map[string]interface{}, len(columns))
	for idx, value := range row {
		if bytes, ok := value.([]byte); ok {
			rowMap[columns[idx]] = string(bytes)
			continue
		}
		rowMap[columns[idx]] = value
	}
	return rowMap
}

func normalizeAnalyzeSchemaSampleSize(sampleSize int) int {
	if sampleSize <= 0 {
		return 10
	}
	return sampleSize
}

func marshalAnalyzeSchemaResult(result AnalyzeSchemaResult) (*mcp.CallToolResult, error) {
	b, err := json.Marshal(result)
	if err != nil {
		log.JSONLog("error", "Failed to serialize AnalyzeSchemaResult", map[string]interface{}{"error": err.Error()})
		return nil, err
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(b),
			},
		},
	}, nil
}

// toFloat64 converts numeric types to float64.
// Used by insights_handler.go and insights_stats.go.
func toFloat64(v interface{}) (float64, error) {
	switch val := v.(type) {
	case int:
		return float64(val), nil
	case int32:
		return float64(val), nil
	case int64:
		return float64(val), nil
	case float32:
		return float64(val), nil
	case float64:
		return val, nil
	case string:
		return strconv.ParseFloat(val, 64)
	default:
		return 0, fmt.Errorf("not a number")
	}
}
