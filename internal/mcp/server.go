// server.go
// Author: guyinwonder
// Version: v1.0.0
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
	ctxmgr "database-mcp-provider/internal/mcp/context"
	"database-mcp-provider/internal/mcp/lineage"
	"database-mcp-provider/internal/mcp/nlp"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"sort"
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
	contextMgr    *ctxmgr.Manager
}

const MCPVersion = "v1.0.4"
const MCPAuthor = "guyinwonder"

// Cap for number of data quality issues retained per column to prevent unbounded payload growth
const maxQualityIssuesPerColumn = 10

const sqliteListTablesQuery = "SELECT name FROM sqlite_master WHERE type='table'"

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
	return schema
}

// --- Semantic Relationship Mapping Helpers ---
//
// mapSemanticRelationships combines formal foreign keys, join suggestions, and naming pattern analysis.
func (s *MCPServer) mapSemanticRelationships(
	tables map[string]TableInfo,
	joinSuggestions []JoinSuggestion,
) RelationshipGraph {
	var fkRels []ForeignKeyRelationship
	var semanticRels []SemanticRelationship
	var suggestedJoins []string

	// Add formal FK relationships
	for tableName, t := range tables {
		for _, col := range t.Columns {
			if col.IsForeignKey && col.ForeignKeyRef != nil {
				fkRels = append(fkRels, ForeignKeyRelationship{
					FromTable:        tableName,
					FromColumn:       col.ColumnName,
					ToTable:          col.ForeignKeyRef.RefTable,
					ToColumn:         col.ForeignKeyRef.RefColumn,
					RelationshipType: "many_to_one",
					SuggestedJoin:    fmt.Sprintf("SELECT * FROM %s JOIN %s ON %s.%s = %s.%s", tableName, col.ForeignKeyRef.RefTable, tableName, col.ColumnName, col.ForeignKeyRef.RefTable, col.ForeignKeyRef.RefColumn),
				})
				suggestedJoins = append(suggestedJoins, fmt.Sprintf("SELECT * FROM %s JOIN %s ON %s.%s = %s.%s", tableName, col.ForeignKeyRef.RefTable, tableName, col.ColumnName, col.ForeignKeyRef.RefTable, col.ForeignKeyRef.RefColumn))
			}
		}
	}

	// Add join suggestions as semantic relationships
	for _, js := range joinSuggestions {
		semanticRels = append(semanticRels, SemanticRelationship{
			Tables:           []string{js.FromTable, js.ToTable},
			RelationshipType: "join_suggestion",
			ConnectionBasis:  "discover-joins",
			ConfidenceScore:  0.8,
			SuggestedJoin:    js.SuggestedJoinSQL,
		})
		suggestedJoins = append(suggestedJoins, js.SuggestedJoinSQL)
	}

	// Add implicit relationships from naming patterns
	implicit := s.detectImplicitRelationships(tables)
	semanticRels = append(semanticRels, implicit...)

	return RelationshipGraph{
		ForeignKeys:           fkRels,
		SemanticRelationships: semanticRels,
		SuggestedJoins:        suggestedJoins,
	}
}

// detectImplicitRelationships finds relationships via naming patterns.
func (s *MCPServer) detectImplicitRelationships(tables map[string]TableInfo) []SemanticRelationship {
	var relationships []SemanticRelationship
	for srcName, source := range tables {
		for tgtName, target := range tables {
			if srcName == tgtName {
				continue
			}
			rel := s.analyzeNamingRelationships(srcName, source, tgtName, target)
			if rel != nil && rel.ConfidenceScore > 0.5 {
				relationships = append(relationships, *rel)
			}
		}
	}
	return relationships
}

// analyzeNamingRelationships compares column names for reference patterns.
func (s *MCPServer) analyzeNamingRelationships(srcName string, source TableInfo, tgtName string, target TableInfo) *SemanticRelationship {
	for _, col := range source.Columns {
		// Pattern: targetTableName + "_id"
		expected := tgtName + "_id"
		if col.ColumnName == expected {
			return &SemanticRelationship{
				Tables:           []string{srcName, tgtName},
				RelationshipType: "implicit_naming",
				ConnectionBasis:  "column_naming",
				ConfidenceScore:  0.8,
				SuggestedJoin:    fmt.Sprintf("SELECT * FROM %s JOIN %s ON %s.%s = %s.id", srcName, tgtName, srcName, col.ColumnName, tgtName),
			}
		}
		// Pattern: shared column names
		for _, tgtCol := range target.Columns {
			if col.ColumnName == tgtCol.ColumnName && col.ColumnName != "id" {
				return &SemanticRelationship{
					Tables:           []string{srcName, tgtName},
					RelationshipType: "shared_column",
					ConnectionBasis:  "shared_column",
					ConfidenceScore:  0.6,
					SuggestedJoin:    fmt.Sprintf("SELECT * FROM %s JOIN %s ON %s.%s = %s.%s", srcName, tgtName, srcName, col.ColumnName, tgtName, tgtCol.ColumnName),
				}
			}
		}
		// Pattern: *_id columns
		if len(col.ColumnName) > 3 && col.ColumnName[len(col.ColumnName)-3:] == "_id" {
			prefix := col.ColumnName[:len(col.ColumnName)-3]
			if prefix == tgtName {
				return &SemanticRelationship{
					Tables:           []string{srcName, tgtName},
					RelationshipType: "implicit_naming",
					ConnectionBasis:  "column_naming",
					ConfidenceScore:  0.7,
					SuggestedJoin:    fmt.Sprintf("SELECT * FROM %s JOIN %s ON %s.%s = %s.id", srcName, tgtName, srcName, col.ColumnName, tgtName),
				}
			}
		}
	}
	return nil
}

// correlateDataValues checks if values in source reference columns exist in target table.
func (s *MCPServer) correlateDataValues(
	srcName string, source TableInfo,
	tgtName string,
	sampleData map[string][]map[string]interface{},
) float64 {
	// Find candidate reference columns
	var refCols []string
	for _, col := range source.Columns {
		if col.ColumnName == tgtName+"_id" || (len(col.ColumnName) > 3 && col.ColumnName[len(col.ColumnName)-3:] == "_id") {
			refCols = append(refCols, col.ColumnName)
		}
	}
	if len(refCols) == 0 {
		return 0.0
	}

	// Get sample values
	sourceRows := sampleData[srcName]
	targetRows := sampleData[tgtName]
	if len(sourceRows) == 0 || len(targetRows) == 0 {
		return 0.0
	}

	// Build set of target IDs
	targetIDs := map[interface{}]struct{}{}
	for _, row := range targetRows {
		if id, ok := row["id"]; ok {
			targetIDs[id] = struct{}{}
		}
	}

	// Check how many source values exist in target
	total := 0
	matches := 0
	for _, row := range sourceRows {
		for _, col := range refCols {
			if val, ok := row[col]; ok {
				total++
				if _, exists := targetIDs[val]; exists {
					matches++
				}
			}
		}
	}
	if total == 0 {
		return 0.0
	}
	return float64(matches) / float64(total)
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

// registerAllTools registers all MCP tools and populates toolsRegistry.
// This is called in Start() and in tests.
func (s *MCPServer) registerAllTools() {
	// Prevent duplicate registration
	if len(s.toolsRegistry) > 0 {
		return
	}

	// configure-profile
	{
		tool := &mcp.Tool{
			Name: "configure-profile",
			Description: `Create or update a database connection profile. Required for all database actions.
		Fields:
		  profile_name (required)
		  db_type (mysql|mariadb|postgres|sqlite) (required)
		  host / port / username / password (required except sqlite)
		  database_name (required)
		  readonly (boolean)
		  sslmode (Postgres only, optional: disable|require|verify-ca|verify-full; defaults to require)
		Example:
		{"profile_name":"some-profile-name","db_type":"postgres","host":"localhost","port":5432,"username":"app","password":"secret","database_name":"appdb","readonly":false,"sslmode":"require"}`,
		}
		mcp.AddTool(s.server, tool, s.handleConfigureProfile)
		s.toolsRegistry = append(s.toolsRegistry, ToolInfo{Name: tool.Name, Description: tool.Description})
	}

	// list-profiles
	{
		tool := &mcp.Tool{
			Name: "list-profiles",
			Description: `List all configured database profiles.
  Example:
  {}`,
		}
		mcp.AddTool(s.server, tool, s.handleListProfiles)
		s.toolsRegistry = append(s.toolsRegistry, ToolInfo{Name: tool.Name, Description: tool.Description})
	}

	// execute-sql
	{
		tool := &mcp.Tool{
			Name: "execute-sql",
			Description: `Execute an arbitrary SQL query or statement. Both 'profile_name' and 'database_name' are required.
		Note: For cross-database queries or describing tables in another database, use fully qualified table names (e.g., db.table).
		Example:
		{"profile_name":"some-profile-name","database_name":"some-database-name","sql":"SELECT * FROM some-table-name WHERE some-field-name=34;"}
		{"profile_name":"some-profile-name","sql":"DESCRIBE some-database-name.some-table-name"}`,
			InputSchema: inputSchemaWithParams[ExecuteSQLParams]("positional parameters for prepared statements; BLOB/BINARY values must be base64-encoded strings"),
		}
		mcp.AddTool(s.server, tool, s.handleExecuteSQL)
		s.toolsRegistry = append(s.toolsRegistry, ToolInfo{Name: tool.Name, Description: tool.Description})
	}

	// list-tables
	{
		tool := &mcp.Tool{
			Name: "list-tables",
			Description: `List all tables in the selected database. Both 'profile_name' and 'database_name' are required.
		Example:
		{"profile_name":"some-profile-name","database_name":"some-database-name"}`,
		}
		mcp.AddTool(s.server, tool, s.handleListTables)
		s.toolsRegistry = append(s.toolsRegistry, ToolInfo{Name: tool.Name, Description: tool.Description})
	}

	// describe-table
	{
		tool := &mcp.Tool{
			Name: "describe-table",
			Description: `Describe the comprehensive schema of a table including columns, types, constraints, comments, and metadata.
  Returns: column names, data types, nullable status, key constraints, default values, column comments, character sets, collation, auto-increment status, max length, precision, and scale.
  Example:
  {"profile_name":"some-profile-name","database_name":"some-database-name","table_name":"some-table-name"}`,
		}
		mcp.AddTool(s.server, tool, s.handleDescribeTable)
		s.toolsRegistry = append(s.toolsRegistry, ToolInfo{Name: tool.Name, Description: tool.Description})
	}

	// list-databases
	{
		tool := &mcp.Tool{
			Name: "list-databases",
			Description: `List all databases/schemas available to the profile.
  Example:
  {"profile_name":"some-profile-name"}`,
		}
		mcp.AddTool(s.server, tool, s.handleListDatabases)
		s.toolsRegistry = append(s.toolsRegistry, ToolInfo{Name: tool.Name, Description: tool.Description})
	}

	// analyze-schema
	{
		tool := &mcp.Tool{
			Name: "analyze-schema",
			Description: `Perform schema analysis for a database, including table/column metadata, relationships, and sample data integration.
  
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
  
  AI agents MUST specify analysis_level. Example:
  {"profile_name":"analytics_db","analysis_level":"detailed","database_name":"analytics_db"}`,
		}
		mcp.AddTool(s.server, tool, s.handleAnalyzeSchema)
		s.toolsRegistry = append(s.toolsRegistry, ToolInfo{Name: tool.Name, Description: tool.Description})
	}

	// smart-query-builder
	{
		tool := &mcp.Tool{
			Name: "smart-query-builder",
			Description: `Generate optimized SQL from high-level intent and schema analysis.
  Input: profile_name, intent (natural language), optional database_name/table_name(s).
  Returns: generated SQL, explanation, and any errors.
  Example:
  {"profile_name":"some-profile-name","intent":"attendance dashboard"}`,
		}
		mcp.AddTool(s.server, tool, s.handleSmartQueryBuilder)
		s.toolsRegistry = append(s.toolsRegistry, ToolInfo{Name: tool.Name, Description: tool.Description})
	}

	// optimize-query
	{
		tool := &mcp.Tool{
			Name: "optimize-query",
			Description: `Run EXPLAIN and return optimization findings for a SQL statement.
		Input: profile_name (required), database_name (required), sql (required), params (optional).
		Returns: execution plan, detected issues (missing indexes, inefficient joins), and estimated improvement range.
		Example:
		{"profile_name":"analytics_db","database_name":"analytics_db","sql":"SELECT * FROM orders WHERE customer_id = ?","params":[123]}`,
			InputSchema: inputSchemaWithParams[OptimizeQueryParams]("positional parameters for prepared statements; BLOB/BINARY values must be base64-encoded strings"),
		}
		mcp.AddTool(s.server, tool, s.handleOptimizeQuery)
		s.toolsRegistry = append(s.toolsRegistry, ToolInfo{Name: tool.Name, Description: tool.Description})
	}

	// validate-query
	{
		tool := &mcp.Tool{
			Name: "validate-query",
			Description: `Validate SQL syntax and detect risky patterns without executing the statement.
		Input: profile_name (required), sql (required), database_name (optional), params (optional).
		Returns: validation issues (syntax, logic, security) and pass/fail summary.
		Example:
		{"profile_name":"analytics_db","sql":"SELECT * FROM users WHERE id = ?","params":[123]}`,
			InputSchema: inputSchemaWithParams[ValidateQueryParams]("positional parameters for prepared statements (metadata only); BLOB/BINARY values must be base64-encoded strings"),
		}
		mcp.AddTool(s.server, tool, s.handleValidateQuery)
		s.toolsRegistry = append(s.toolsRegistry, ToolInfo{Name: tool.Name, Description: tool.Description})
	}

	// analyze-data-lineage
	{
		tool := &mcp.Tool{
			Name: "analyze-data-lineage",
			Description: `Trace data dependencies for a table using foreign key relationships.
  Input: profile_name (required), table_name (required), database_name (optional), scope (optional: upstream|downstream|both).
  Returns: upstream/downstream tables and dependency edges.
  Example:
  {"profile_name":"analytics_db","table_name":"orders","scope":"both"}`,
		}
		mcp.AddTool(s.server, tool, s.handleAnalyzeDataLineage)
		s.toolsRegistry = append(s.toolsRegistry, ToolInfo{Name: tool.Name, Description: tool.Description})
	}

	// discover-insights
	{
		tool := &mcp.Tool{
			Name: "discover-insights",
			Description: `Automatically discovers KPIs, trends, anomalies, and distribution patterns in database tables.
  Input: profile_name (required), table_name (required), columns (optional), insight_types (optional: kpi, trend, anomaly, distribution), max_results (optional).
  Returns: list of insights with type, column, description, and detailed metrics.
  Example:
  {"profile_name":"analytics_db","table_name":"sales","insight_types":["kpi","trend"],"max_results":10}`,
			InputSchema: inputSchemaWithParams[DiscoverInsightsParams]("Optional query parameters for filtering insights"),
		}
		mcp.AddTool(s.server, tool, s.handleDiscoverInsights)
		s.toolsRegistry = append(s.toolsRegistry, ToolInfo{Name: tool.Name, Description: tool.Description})
	}

	// track-schema-changes
	{
		tool := &mcp.Tool{
			Name: "track-schema-changes",
			Description: `Track schema evolution with snapshots, history, migration generation, and drift detection.
  Input: profile_name (required), operation (optional: track|history|generate_migration|detect_drift), database_name (optional), dialect (optional),
         from_snapshot_id/to_snapshot_id (optional for migration), snapshot_id (optional for drift), limit (optional), retention_days (optional).
  Returns: schema snapshots, detected changes, migration script/validation/impact, or drift report depending on operation.
  Example:
  {"profile_name":"analytics_db","operation":"track","retention_days":30}`,
		}
		mcp.AddTool(s.server, tool, s.handleTrackSchemaChanges)
		s.toolsRegistry = append(s.toolsRegistry, ToolInfo{Name: tool.Name, Description: tool.Description})
	}

	// discover-joins
	{
		tool := &mcp.Tool{
			Name: "discover-joins",
			Description: `Discover joinable relationships (foreign keys) between tables and suggest JOIN SQL.
  Input: profile_name (required), tables (optional).
  Returns: list of join suggestions and summary.
  Example:
  {"profile_name":"analytics_db","tables":["orders","customers"]}`,
		}
		mcp.AddTool(s.server, tool, s.handleDiscoverJoins)
		s.toolsRegistry = append(s.toolsRegistry, ToolInfo{Name: tool.Name, Description: tool.Description})
	}

	// sample-data
	{
		tool := &mcp.Tool{
			Name: "sample-data",
			Description: `Fetch sample rows from a table to help AI/agents infer data types, formats, and value ranges.
  Input: profile_name (required), database_name (required), table_name (required), sample_size (optional, default: 3).
  Returns: sample rows with column names and values.
  Example:
  {"profile_name":"analytics_db","database_name":"analytics_db","table_name":"users","sample_size":5}`,
		}
		mcp.AddTool(s.server, tool, s.handleSampleData)
		s.toolsRegistry = append(s.toolsRegistry, ToolInfo{Name: tool.Name, Description: tool.Description})
	}

	// mcp-info
	{
		tool := &mcp.Tool{
			Name: "mcp-info",
			Description: `Show MCP provider version and author.
  Example:
  {}`,
		}
		mcp.AddTool(s.server, tool, s.handleMCPInfo)
		s.toolsRegistry = append(s.toolsRegistry, ToolInfo{Name: tool.Name, Description: tool.Description})
	}

	// list-tools
	{
		tool := &mcp.Tool{
			Name: "list-tools",
			Description: `List all available MCP tools and their descriptions.
  Example:
  {}`,
		}
		mcp.AddTool(s.server, tool, s.handleListTools)
		s.toolsRegistry = append(s.toolsRegistry, ToolInfo{Name: tool.Name, Description: tool.Description})
	}
}

// registerResources registers non-tool MCP resources and templates for discovery.
func (s *MCPServer) registerResources() {
	// Static tools snapshot as a resource
	s.server.AddResource(&mcp.Resource{
		URI:         "tools://list",
		Name:        "Registered MCP Tools",
		Description: "All registered MCP tools with descriptions",
		MIMEType:    "application/json",
	}, s.resourceToolsHandler)

	// Profile metadata (secrets redacted) via URI template
	s.server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "profile://{profile}",
		Name:        "Database profile metadata",
		Description: "Profile connection metadata without secrets",
		MIMEType:    "application/json",
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

	// 1. Load config/profile and connect to DB
	conn, prof, err := s.openConnection(ctx, p.ProfileName, "")
	if err != nil {
		if prof == nil {
			log.JSONLog("error", "Profile not found", map[string]interface{}{"profile_name": p.ProfileName})
			return nil, nil, fmt.Errorf("profile not found")
		}
		log.JSONLog("error", "Failed to connect to database", map[string]interface{}{"error": err})
		structErr := s.errorAnalyzer.AnalyzeError(err, map[string]interface{}{
			"profile_name": p.ProfileName,
			"operation":    "discover_joins",
			"db_type":      prof.DBType,
		})
		return errorResult(structErr), nil, nil
	}
	defer conn.Close() //nolint:errcheck // Standard pattern: error in deferred close is not critical

	// 3. Query foreign key metadata (MySQL/MariaDB/Postgres/SQLite)
	var fkQuery string
	switch prof.DBType {
	case "mysql", "mariadb":
		fkQuery = `
			SELECT TABLE_NAME, COLUMN_NAME, REFERENCED_TABLE_NAME, REFERENCED_COLUMN_NAME
			FROM INFORMATION_SCHEMA.KEY_COLUMN_USAGE
			WHERE TABLE_SCHEMA = ? AND REFERENCED_TABLE_NAME IS NOT NULL`
	case "postgres":
		fkQuery = `
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
			WHERE tc.constraint_type = 'FOREIGN KEY' AND tc.table_schema = 'public'`
	case "sqlite":
		// SQLite: need to PRAGMA foreign_key_list for each table
		// Will handle below
	default:
		return nil, nil, fmt.Errorf("unsupported db_type for join discovery")
	}

	joins := []JoinSuggestion{}
	tableSet := map[string]bool{}
	if len(p.Tables) > 0 {
		for _, t := range p.Tables {
			tableSet[strings.ToLower(t)] = true
		}
	}

	if prof.DBType == "sqlite" {
		// Get all tables if not specified
		tables := p.Tables
		if len(tables) == 0 {
			rows, err := conn.QueryContext(ctx, sqliteListTablesQuery)
			if err != nil {
				return nil, nil, err
			}
			for rows.Next() {
				var name string
				if err := rows.Scan(&name); err == nil {
					tables = append(tables, name)
				}
			}
			if err := rows.Close(); err != nil {
				log.JSONLog("warn", "Failed to close SQLite table list rows", map[string]interface{}{"error": err.Error()})
			}
		}
		for _, tbl := range tables {
			rows, err := conn.QueryContext(ctx, fmt.Sprintf("PRAGMA foreign_key_list('%s')", tbl)) // NOSONAR
			if err != nil {
				log.JSONLog("warn", "Failed to query SQLite foreign keys", map[string]interface{}{"table": tbl, "error": err})
				continue
			}
			for rows.Next() {
				var id, seq int
				var table, from, to, onUpdate, onDelete, match string
				if err := rows.Scan(&id, &seq, &table, &from, &to, &onUpdate, &onDelete, &match); err == nil {
					if len(tableSet) == 0 || tableSet[strings.ToLower(tbl)] || tableSet[strings.ToLower(table)] {
						joins = append(joins, JoinSuggestion{
							FromTable:        tbl,
							FromColumn:       from,
							ToTable:          table,
							ToColumn:         to,
							Relationship:     "foreign_key",
							SuggestedJoinSQL: fmt.Sprintf("SELECT * FROM %s JOIN %s ON %s.%s = %s.%s", tbl, table, tbl, from, table, to),
						})
					}
				} else {
					log.JSONLog("warn", "Failed to scan SQLite foreign key row", map[string]interface{}{"table": tbl, "error": err.Error()})
				}
			}
			if err := rows.Close(); err != nil {
				log.JSONLog("warn", "Failed to close SQLite foreign key rows", map[string]interface{}{"table": tbl, "error": err.Error()})
			}
		}
	} else {
		var rows *sql.Rows
		if prof.DBType == "mysql" || prof.DBType == "mariadb" {
			rows, err = conn.QueryContext(ctx, fkQuery, prof.DatabaseName)
		} else {
			rows, err = conn.QueryContext(ctx, fkQuery)
		}
		if err != nil {
			log.JSONLog("error", "Failed to query foreign keys", map[string]interface{}{"error": err})
			structErr := s.errorAnalyzer.AnalyzeError(err, map[string]interface{}{
				"profile_name": p.ProfileName,
				"operation":    "discover_joins",
				"db_type":      prof.DBType,
			})
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{
						Text: structErr.ToJSON(),
					},
				},
			}, nil, nil
		}
		defer rows.Close() //nolint:errcheck // Standard pattern: error in deferred close is not critical
		for rows.Next() {
			var fromTable, fromCol, toTable, toCol string
			if err := rows.Scan(&fromTable, &fromCol, &toTable, &toCol); err == nil {
				if len(tableSet) == 0 || tableSet[strings.ToLower(fromTable)] || tableSet[strings.ToLower(toTable)] {
					joins = append(joins, JoinSuggestion{
						FromTable:        fromTable,
						FromColumn:       fromCol,
						ToTable:          toTable,
						ToColumn:         toCol,
						Relationship:     "foreign_key",
						SuggestedJoinSQL: fmt.Sprintf("SELECT * FROM %s JOIN %s ON %s.%s = %s.%s", fromTable, toTable, fromTable, fromCol, toTable, toCol),
					})
				}
			}
		}
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

// handleMCPInfo returns author and version information.
func (s *MCPServer) handleMCPInfo(ctx context.Context, _ *mcp.CallToolRequest, input any) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: "Database MCP Provider\nAuthor: " + MCPAuthor + "\nVersion: " + MCPVersion + "\nCreated using OpenAI GPT-4.1 via VSCode Kilocode AI code assistant extension.",
			},
		},
	}, nil, nil
}

// --- MCP Handler Parameter Structs ---

type ConfigureProfileParams struct {
	ProfileName  string `json:"profile_name" jsonschema:"unique name for this database profile"`
	DBType       string `json:"db_type" jsonschema:"database type: mysql | mariadb | postgres | sqlite"`
	Host         string `json:"host,omitempty" jsonschema:"hostname or IP (not required for sqlite)"`
	Port         int    `json:"port,omitempty" jsonschema:"TCP port number (not required for sqlite)"`
	Username     string `json:"username,omitempty" jsonschema:"database username (not required for sqlite)"`
	Password     string `json:"password,omitempty" jsonschema:"database password (not required for sqlite)"`
	DatabaseName string `json:"database_name" jsonschema:"default database/schema name or sqlite file path"`
	Readonly     bool   `json:"readonly" jsonschema:"when true, write operations are blocked"`
	SSLMode      string `json:"sslmode,omitempty" jsonschema:"postgres only: disable | require | verify-ca | verify-full"`
}

type ListProfilesResult struct {
	Profiles []struct {
		ProfileName string `json:"profile_name"`
		DBType      string `json:"db_type"`
	} `json:"profiles"`
}

type ExecuteSQLParams struct {
	ProfileName  string        `json:"profile_name" jsonschema:"profile to use for connection"`
	SQL          string        `json:"sql" jsonschema:"SQL statement to execute"`
	DatabaseName string        `json:"database_name" jsonschema:"target database/schema (sqlite uses file path)"`
	Params       []interface{} `json:"params,omitempty" jsonschema:"positional parameters for prepared statement"`
}

type ExecuteSQLResult struct {
	Columns  []string        `json:"columns,omitempty"`
	Rows     [][]interface{} `json:"rows,omitempty"`
	Affected int             `json:"affected,omitempty"`
}

type ListTablesParams struct {
	ProfileName  string `json:"profile_name" jsonschema:"profile to use for connection"`
	DatabaseName string `json:"database_name" jsonschema:"database/schema to inspect"`
}

type ListTablesResult struct {
	Tables []string `json:"tables"`
}

type DescribeTableParams struct {
	ProfileName  string `json:"profile_name" jsonschema:"profile to use for connection"`
	DatabaseName string `json:"database_name" jsonschema:"database/schema containing the table"`
	TableName    string `json:"table_name" jsonschema:"table to describe"`
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
	ProfileName  string   `json:"profile_name" jsonschema:"profile to use for connection"`
	Intent       string   `json:"intent" jsonschema:"natural-language intent to convert to SQL"`
	DatabaseName string   `json:"database_name,omitempty" jsonschema:"optional database/schema context"`
	TableNames   []string `json:"table_names,omitempty" jsonschema:"optional focus tables for SQL generation"`
}

type SmartQueryBuilderResult struct {
	SQL         string `json:"sql"`
	Explanation string `json:"explanation"`
}

// --- Validate Query Types ---
type ValidateQueryParams struct {
	ProfileName  string        `json:"profile_name" jsonschema:"profile to use for connection"`
	DatabaseName string        `json:"database_name,omitempty" jsonschema:"optional database/schema context"`
	SQL          string        `json:"sql" jsonschema:"SQL statement to validate"`
	Params       []interface{} `json:"params,omitempty" jsonschema:"positional parameters for prepared statement (metadata only)"`
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
	ProfileName  string        `json:"profile_name" jsonschema:"profile to use for connection"`
	DatabaseName string        `json:"database_name" jsonschema:"database/schema to use for EXPLAIN (sqlite uses file path)"`
	SQL          string        `json:"sql" jsonschema:"SQL statement to analyze"`
	Params       []interface{} `json:"params,omitempty" jsonschema:"positional parameters for prepared statement"`
}

type OptimizeQueryResult struct {
	Plan       *ExplainPlan          `json:"plan"`
	Findings   []OptimizationFinding `json:"findings,omitempty"`
	Estimation PerformanceEstimation `json:"estimation"`
	Summary    string                `json:"summary"`
}

// --- End Smart Query Builder Types ---
type DiscoverJoinsParams struct {
	ProfileName string   `json:"profile_name" jsonschema:"profile to use for connection"`
	Tables      []string `json:"tables,omitempty" jsonschema:"optional subset of tables to analyze for joins"`
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
	ProfileName string `json:"profile_name" jsonschema:"profile to use for connection"`
}

type ListDatabasesResult struct {
	Databases []string `json:"databases"`
}

// --- Sample Data Types ---
type SampleDataParams struct {
	ProfileName  string `json:"profile_name" jsonschema:"profile to use for connection"`
	TableName    string `json:"table_name" jsonschema:"table to sample"`
	DatabaseName string `json:"database_name" jsonschema:"database/schema containing the table"`
	SampleSize   int    `json:"sample_size,omitempty" jsonschema:"number of rows to return (default 3)"`
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

// --- Lineage Types ---
type AnalyzeDataLineageParams struct {
	ProfileName  string   `json:"profile_name" jsonschema:"profile to use for connection"`
	DatabaseName string   `json:"database_name,omitempty" jsonschema:"database/schema to use (sqlite uses file path)"`
	TableName    string   `json:"table_name" jsonschema:"target table for lineage"`
	Scope        string   `json:"scope,omitempty" jsonschema:"upstream|downstream|both (default both)"`
	Tables       []string `json:"tables,omitempty" jsonschema:"optional subset of tables to include"`
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
				MIMEType: "application/json",
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
		if err.Error() == "profile not found" {
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
				MIMEType: "application/json",
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

func matchTableByKeywords(tables []string, keywords []string) string {
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

func appendUnique(target []string, values []string) []string {
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

	// 1. Load config and profile
	cfg, prof, err := s.findProfile(p.ProfileName)
	if err != nil {
		if err.Error() == "profile not found" {
			structErr := NewStructuredError(
				ErrorCodeProfileNotFound,
				"Profile not found",
				"profile_name does not exist; configure a profile before using smart-query-builder",
			).WithSuggestions(
				ErrorSuggestion{
					Tool:        "configure-profile",
					Description: "Create the profile with database connection info",
					Example:     `{"profile_name":"mydb","db_type":"postgres","host":"localhost","port":5432,"username":"user","password":"pass","database_name":"mydb"}`,
				},
			).WithContext("profile_name", p.ProfileName)
			return errorResult(structErr), nil, fmt.Errorf("profile not found")
		}
		return nil, nil, err
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

	// 2. Fetch all table names
	dsn := db.DSN(prof.DBType, prof.Host, prof.Port, prof.Username, prof.Password, prof.DatabaseName, prof.SSLMode)
	conn, err := db.OpenConnectionWithPool(ctx, prof.DBType, dsn, cfg.MaxPoolSize)
	if err != nil {
		log.JSONLog("error", "Failed to open database connection", map[string]interface{}{"profile": p.ProfileName, "error": err})
		structErr := s.errorAnalyzer.AnalyzeError(err, map[string]interface{}{
			"profile_name":  p.ProfileName,
			"database_name": p.DatabaseName,
			"operation":     "smart_query_builder",
			"db_type":       prof.DBType,
		})
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: structErr.ToJSON(),
				},
			},
		}, nil, nil
	}
	defer conn.Close() //nolint:errcheck // Standard pattern: error in deferred close is not critical

	var tables []string
	{
		var query string
		switch prof.DBType {
		case "mysql", "mariadb":
			query = "SHOW FULL TABLES"
		case "postgres":
			query = "SELECT table_name FROM information_schema.tables WHERE table_schema='public'"
		case "sqlite":
			query = sqliteListTablesQuery
		default:
			return nil, nil, fmt.Errorf("unsupported db_type")
		}
		rows, err := conn.QueryContext(ctx, query)
		if err != nil {
			log.JSONLog("error", "Failed to list tables", map[string]interface{}{"error": err})
			// Use the database name from params or profile
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
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{
						Text: structErr.ToJSON(),
					},
				},
			}, nil, nil
		}
		defer rows.Close() //nolint:errcheck // Standard pattern: error in deferred close is not critical
		for rows.Next() {
			if prof.DBType == "mysql" || prof.DBType == "mariadb" {
				var name, tableType string
				if err := rows.Scan(&name, &tableType); err == nil {
					tables = append(tables, name)
				} else {
					log.JSONLog("warn", "Failed to scan table name", map[string]interface{}{"db_type": prof.DBType, "error": err.Error(), "operation": "smart_query_builder_list_tables"})
				}
			} else {
				var name string
				if err := rows.Scan(&name); err == nil {
					tables = append(tables, name)
				} else {
					log.JSONLog("warn", "Failed to scan table name", map[string]interface{}{"db_type": prof.DBType, "error": err.Error(), "operation": "smart_query_builder_list_tables"})
				}
			}
		}
	}

	// 3. Parse intent and match to table names
	table := selectTableForIntent(p, tables, contextEnabled, history, cfg.NLP.BusinessDomains)

	if table == "" {
		errMsg := "No table found matching the intent for query generation."
		suggestion := ""
		if len(tables) > 0 {
			suggestion = fmt.Sprintf("Available tables: %s.", strings.Join(tables, ", "))
		} else {
			suggestion = "No tables found in the database."
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf(`{"status":"error","error_code":"NO_TABLE_MATCH","message":"%s %s"}`, errMsg, suggestion),
				},
			},
		}, nil, nil
	}
	// Fetch columns for the selected table
	var columns []string
	{
		var colQuery string
		switch prof.DBType {
		case "mysql", "mariadb":
			colQuery = fmt.Sprintf("SHOW COLUMNS FROM `%s`", table)
		case "postgres":
			colQuery = fmt.Sprintf("SELECT column_name FROM information_schema.columns WHERE table_name = '%s' AND table_schema = 'public'", table)
		case "sqlite":
			colQuery = fmt.Sprintf("PRAGMA table_info('%s')", table)
		default:
			return nil, nil, fmt.Errorf("unsupported db_type")
		}
		rows, err := conn.QueryContext(ctx, colQuery)
		if err != nil {
			return nil, nil, err
		}
		defer rows.Close() //nolint:errcheck // Standard pattern: error in deferred close is not critical
		for rows.Next() {
			var colName string
			switch prof.DBType {
			case "mysql", "mariadb":
				var field, typ, null, key, def, extra string
				if err := rows.Scan(&field, &typ, &null, &key, &def, &extra); err == nil {
					colName = field
				} else {
					log.JSONLog("warn", "Failed to scan column metadata", map[string]interface{}{"table": table, "error": err.Error(), "db_type": prof.DBType, "operation": "smart_query_builder_columns"})
					continue
				}
			case "postgres":
				if err := rows.Scan(&colName); err != nil {
					log.JSONLog("warn", "Failed to scan PostgreSQL column name", map[string]interface{}{"table": table, "error": err.Error(), "operation": "smart_query_builder_columns"})
					continue
				}
			case "sqlite":
				var cid int
				var typ, notnull, dflt_value, pk interface{}
				if err := rows.Scan(&cid, &colName, &typ, &notnull, &dflt_value, &pk); err != nil {
					log.JSONLog("warn", "Failed to scan SQLite column metadata", map[string]interface{}{"table": table, "error": err.Error(), "operation": "smart_query_builder_columns"})
					continue
				}
			}
			if colName != "" {
				columns = append(columns, colName)
			}
		}
	}
	colList := "*"
	if len(columns) > 0 {
		colList = strings.Join(columns, ", ")
	}
	sql := fmt.Sprintf("SELECT %s FROM %s;", colList, table)
	explanation := fmt.Sprintf("Selected table '%s' and columns [%s] based on keywords from intent '%s'.", table, colList, p.Intent)
	if len(cfg.NLP.BusinessDomains) > 0 {
		explanation = fmt.Sprintf("%s Business domains: %s.", explanation, strings.Join(cfg.NLP.BusinessDomains, ", "))
	}

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
			"Missing required parameters",
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
		if err.Error() == "profile not found" {
			structErr := NewStructuredError(
				ErrorCodeProfileNotFound,
				"Profile not found",
				"profile_name does not exist; configure a profile before using smart-query-builder",
			).WithSuggestions(
				ErrorSuggestion{
					Tool:        "configure-profile",
					Description: "Create the profile with database connection info",
					Example:     `{"profile_name":"mydb","db_type":"postgres","host":"localhost","port":5432,"username":"user","password":"pass","database_name":"mydb"}`,
				},
			).WithContext("profile_name", p.ProfileName)
			return errorResult(structErr), nil, fmt.Errorf("profile not found")
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
			"Missing required parameters",
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
		if err.Error() == "profile not found" {
			structErr := NewStructuredError(
				ErrorCodeProfileNotFound,
				"Profile not found",
				"profile_name does not exist; configure a profile before using validate-query",
			).WithSuggestions(
				ErrorSuggestion{
					Tool:        "configure-profile",
					Description: "Create the profile with database connection info",
					Example:     `{"profile_name":"mydb","db_type":"postgres","host":"localhost","port":5432,"username":"user","password":"pass","database_name":"mydb"}`,
				},
			).WithContext("profile_name", p.ProfileName)
			return errorResult(structErr), nil, fmt.Errorf("profile not found")
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
			"Missing required parameters",
			"profile_name and table_name are required for analyze-data-lineage",
		)
		return errorResult(structErr), nil, nil
	}

	// build FK edges via INFORMATION_SCHEMA / PRAGMA similar to discover-joins
	conn, prof, err := s.openConnection(ctx, p.ProfileName, "")
	if err != nil {
		if prof == nil {
			return nil, nil, fmt.Errorf("profile not found")
		}
		return nil, nil, err
	}
	defer conn.Close() //nolint:errcheck // Standard pattern: error in deferred close is not critical

	var edges []lineage.Edge
	switch prof.DBType {
	case "mysql", "mariadb":
		rows, err := conn.QueryContext(ctx, `
			SELECT TABLE_NAME, REFERENCED_TABLE_NAME
			FROM INFORMATION_SCHEMA.KEY_COLUMN_USAGE
			WHERE TABLE_SCHEMA = ? AND REFERENCED_TABLE_NAME IS NOT NULL
		`, prof.DatabaseName)
		if err != nil {
			return nil, nil, err
		}
		defer rows.Close() //nolint:errcheck // Standard pattern: error in deferred close is not critical
		for rows.Next() {
			var tbl, ref string
			if err := rows.Scan(&tbl, &ref); err == nil {
				edges = append(edges, lineage.Edge{From: tbl, To: ref})
			}
		}
	case "postgres":
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
			return nil, nil, err
		}
		defer rows.Close() //nolint:errcheck // Standard pattern: error in deferred close is not critical
		for rows.Next() {
			var from, to string
			if err := rows.Scan(&from, &to); err == nil {
				edges = append(edges, lineage.Edge{From: from, To: to})
			}
		}
	case "sqlite":
		// fetch tables
		var tables []string
		tRows, err := conn.QueryContext(ctx, sqliteListTablesQuery)
		if err != nil {
			return nil, nil, err
		}
		for tRows.Next() {
			var name string
			if err := tRows.Scan(&name); err == nil {
				tables = append(tables, name)
			}
		}
		if err := tRows.Close(); err != nil {
			log.JSONLog("warn", "Failed to close SQLite table list rows", map[string]interface{}{"error": err.Error()})
		}
		for _, tbl := range tables {
			rows, err := conn.QueryContext(ctx, fmt.Sprintf("PRAGMA foreign_key_list('%s')", tbl)) // NOSONAR
			if err != nil {
				continue
			}
			for rows.Next() {
				var id, seq int
				var refTable, fromCol, toCol, onUpd, onDel, match string
				if err := rows.Scan(&id, &seq, &refTable, &fromCol, &toCol, &onUpd, &onDel, &match); err == nil {
					edges = append(edges, lineage.Edge{From: tbl, To: refTable})
				}
			}
			if err := rows.Close(); err != nil {
				log.JSONLog("warn", "Failed to close SQLite foreign key rows", map[string]interface{}{"table": tbl, "error": err.Error()})
			}
		}
	default:
		return nil, nil, fmt.Errorf("unsupported db_type for lineage: %s", prof.DBType)
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

func (s *MCPServer) handleConfigureProfile(ctx context.Context, _ *mcp.CallToolRequest, input ConfigureProfileParams) (*mcp.CallToolResult, any, error) {
	// Load config, or create new if missing
	cfg, err := config.LoadConfig(s.ConfigPath)
	log.JSONLog("debug", "Loaded config for configure-profile", map[string]interface{}{"configPath": s.ConfigPath, "error": err})
	if err != nil {
		log.JSONLog("warn", "Failed to load config, creating new config", map[string]interface{}{"error": err.Error()})
		cfg = &config.Config{}
	}
	p := input
	// Default database_name to "mysql" for MySQL/MariaDB if empty
	if (p.DBType == "mysql" || p.DBType == "mariadb") && p.DatabaseName == "" {
		p.DatabaseName = "mysql"
	}
	// Default sslmode to "require" for Postgres if empty (secure default)
	if p.DBType == "postgres" && p.SSLMode == "" {
		p.SSLMode = "require"
	}
	// Validate required fields
	if p.ProfileName == "" || p.DBType == "" || p.DatabaseName == "" {
		structErr := NewStructuredError(
			ErrorCodeMissingParameter,
			"Missing required parameters",
			"All of profile_name, db_type, and database_name are required",
		).WithSuggestions(
			ErrorSuggestion{
				Action:      "Provide all required parameters",
				Description: "Ensure profile_name, db_type, and database_name are included",
				Example:     `{"profile_name": "mydb", "db_type": "mysql", "database_name": "mydb"}`,
			},
		)
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: structErr.ToJSON(),
				},
			},
		}, nil, nil
	}

	// Ensure AES key (auto-generate secure key if missing or invalid length)
	if cfg.AESKey == "" || len(cfg.AESKey) != 32 {
		keyBytes := make([]byte, 24) // Base64(24 bytes) == 32 chars, no padding
		if _, genErr := rand.Read(keyBytes); genErr == nil {
			cfg.AESKey = base64.StdEncoding.EncodeToString(keyBytes)
			log.JSONLog("info", "Generated new AES key for configuration", map[string]interface{}{
				"config_path": s.ConfigPath,
				"length":      len(cfg.AESKey),
			})
		} else {
			log.JSONLog("error", "Failed to generate AES key", map[string]interface{}{"error": genErr.Error()})
		}
	}

	// Update or add profile
	found := false
	for i := range cfg.Profiles {
		if cfg.Profiles[i].ProfileName == p.ProfileName {
			cfg.Profiles[i] = config.Profile{
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
			found = true
			break
		}
	}
	if !found {
		cfg.Profiles = append(cfg.Profiles, config.Profile{
			ProfileName:  p.ProfileName,
			DBType:       p.DBType,
			Host:         p.Host,
			Port:         p.Port,
			Username:     p.Username,
			Password:     p.Password,
			DatabaseName: p.DatabaseName,
			Readonly:     p.Readonly,
			SSLMode:      p.SSLMode,
		})
	}
	// Save config
	if err := config.SaveConfig(s.ConfigPath, cfg); err != nil {
		structErr := s.errorAnalyzer.AnalyzeError(err, map[string]interface{}{
			"profile_name": p.ProfileName,
			"operation":    "save_config",
		})
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: structErr.ToJSON(),
				},
			},
		}, nil, nil
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: "Profile configured successfully.",
			},
		},
	}, nil, nil
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
	// Validate required parameters
	if p.ProfileName == "" || p.DatabaseName == "" {
		structErr := NewStructuredError(
			ErrorCodeMissingParameter,
			"Missing required parameters",
			"Both profile_name and database_name are required",
		).WithSuggestions(
			ErrorSuggestion{
				Action:      "Provide all required parameters",
				Description: "Ensure profile_name and database_name are included",
				Example:     `{"profile_name": "mydb", "database_name": "mydb", "sql": "SELECT 1"}`,
			},
		)
		return errorResult(structErr), nil, nil
	}
	cfg, prof, err := s.findProfile(p.ProfileName)
	if err != nil {
		if cfg == nil || len(cfg.Profiles) == 0 {
			log.JSONLog("error", "No database profiles configured", map[string]interface{}{"error": err})
			structErr := NewStructuredError(
				ErrorCodeConfigNotFound,
				"No database profiles configured",
				"Configuration file is missing or contains no profiles",
			).WithSuggestions(
				ErrorSuggestion{
					Tool:        "configure-profile",
					Description: "Create a new database profile",
					Example:     `{"profile_name": "mydb", "db_type": "mysql", "host": "localhost", "port": 3306, "username": "user", "password": "pass", "database_name": "mydb"}`,
				},
			)
			return errorResult(structErr), nil, nil
		}
		log.JSONLog("error", "Profile not found", map[string]interface{}{"profile_name": p.ProfileName})
		return nil, nil, fmt.Errorf("profile not found")
	}
	// Strengthened read-only enforcement (supports WITH CTEs, blocks multi-statement & dangerous verbs)
	if prof.Readonly {
		origSQL := p.SQL
		sqlNorm := strings.TrimSpace(origSQL)

		// Strip leading comments (simple approach)
		for {
			trimmed := strings.TrimSpace(sqlNorm)
			switch {
			case strings.HasPrefix(trimmed, "--"):
				if idx := strings.Index(trimmed, "\n"); idx != -1 {
					sqlNorm = trimmed[idx+1:]
				} else {
					sqlNorm = ""
				}
			case strings.HasPrefix(trimmed, "/*"):
				if idx := strings.Index(trimmed, "*/"); idx != -1 {
					sqlNorm = trimmed[idx+2:]
				} else {
					sqlNorm = ""
				}
			default:
				sqlNorm = trimmed
				goto comments_done
			}
		}
	comments_done:
		// Sanitize out single-quoted strings to reduce false positives for verbs inside literals
		sanitize := func(s string) string {
			var b strings.Builder
			inSingle := false
			for i := 0; i < len(s); i++ {
				ch := s[i]
				if ch == '\'' {
					inSingle = !inSingle
					b.WriteByte(' ') // mask quote
					continue
				}
				if inSingle {
					b.WriteByte(' ') // mask string content
				} else {
					// lower for uniform scanning
					b.WriteByte(byte(unicode.ToLower(rune(ch))))
				}
			}
			return b.String()
		}
		sanitized := sanitize(sqlNorm)

		// Multi-statement detection (ignore trailing semicolon)
		statements := []string{}
		for _, part := range strings.Split(sanitized, ";") {
			pt := strings.TrimSpace(part)
			if pt != "" {
				statements = append(statements, pt)
			}
		}
		if len(statements) > 1 {
			log.JSONLog("warn", "Blocked multi-statement query on readonly profile", map[string]interface{}{
				"profile_name": p.ProfileName,
				"statements":   len(statements),
			})
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
			).WithContext("profile_name", p.ProfileName).
				WithContext("query", origSQL)
			return errorResult(structErr), nil, fmt.Errorf("blocked by readonly profile")
		}

		// Allowed starting tokens (WITH allowed; must resolve to SELECT/EXPLAIN/SHOW/DESCRIBE/PRAGMA)
		allowedStarters := []string{"select", "show", "describe", "explain", "pragma", "with"}
		isAllowedStart := false
		for _, a := range allowedStarters {
			if strings.HasPrefix(strings.TrimLeft(sanitized, "("), a) { // allow leading parenthesis
				isAllowedStart = true
				break
			}
		}

		// WITH queries are allowed; read-only safety is still enforced by disallowed verb scan below.

		// Disallowed verbs (word boundaries)
		disallowed := []string{
			"insert", "update", "delete", "alter", "create", "drop", "truncate",
			"grant", "revoke", "replace", "merge", "call", "do", "attach", "detach", "vacuum",
		}
		disallowedHit := ""
		for _, dv := range disallowed {
			re := regexp.MustCompile(`\b` + dv + `\b`)
			if re.MatchString(sanitized) {
				disallowedHit = dv
				break
			}
		}

		if !isAllowedStart || disallowedHit != "" {
			reason := "Write or unsafe operations are not allowed on readonly profiles"
			if disallowedHit != "" {
				reason = fmt.Sprintf("Detected disallowed verb '%s' in query for readonly profile", disallowedHit)
			}
			log.JSONLog("warn", "Blocked unsafe SQL on readonly profile", map[string]interface{}{
				"profile_name": p.ProfileName,
				"sql":          origSQL,
				"reason":       reason,
			})
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
				"profile_name": p.ProfileName,
				"query":        origSQL,
			})
			return errorResult(structErr), nil, fmt.Errorf("blocked by readonly profile")
		}
	}
	// Use requested database if provided, else profile default
	dbName := prof.DatabaseName
	if p.DatabaseName != "" {
		dbName = p.DatabaseName
	}
	// Build DSN and connect
	dsn := db.DSN(prof.DBType, prof.Host, prof.Port, prof.Username, prof.Password, dbName, prof.SSLMode)
	conn, err := db.OpenConnectionWithPool(ctx, prof.DBType, dsn, cfg.MaxPoolSize)
	if err != nil {
		log.JSONLog("error", "Failed to connect to database", map[string]interface{}{"error": err})
		structErr := s.errorAnalyzer.AnalyzeError(err, map[string]interface{}{
			"profile_name": p.ProfileName,
			"operation":    "connect",
			"db_type":      prof.DBType,
			"database":     dbName,
		})
		return errorResult(structErr), nil, nil
	}
	defer conn.Close() //nolint:errcheck // Standard pattern: error in deferred close is not critical
	// For MySQL/MariaDB, optionally switch database if needed
	if result := s.switchMySQLDatabase(ctx, conn, prof, p.ProfileName, p.DatabaseName); result != nil {
		return result, nil, nil
	}
	// Try query with parameters
	var rows *sql.Rows
	if len(p.Params) > 0 {
		stmt, err := conn.PrepareContext(ctx, p.SQL)
		if err != nil {
			log.JSONLog("error", "Failed to prepare statement", map[string]interface{}{"sql": p.SQL, "error": err})
			structErr := s.errorAnalyzer.AnalyzeError(err, map[string]interface{}{
				"profile_name": p.ProfileName,
				"sql":          p.SQL,
				"operation":    "prepare_statement",
				"db_type":      prof.DBType,
			})
			return errorResult(structErr), nil, nil
		}
		defer stmt.Close()                  //nolint:errcheck // Standard pattern: error in deferred close is not critical
		rows, err = stmt.Query(p.Params...) //nolint:noctx // Prepared with PrepareContext, context already bound
		if err != nil {
			log.JSONLog("error", "Failed to execute prepared query", map[string]interface{}{"sql": p.SQL, "params": p.Params, "error": err})
			structErr := s.errorAnalyzer.AnalyzeError(err, map[string]interface{}{
				"profile_name": p.ProfileName,
				"sql":          p.SQL,
				"operation":    "prepared_query",
				"db_type":      prof.DBType,
			})
			return errorResult(structErr), nil, nil
		}
	} else {
		rows, err = conn.QueryContext(ctx, p.SQL)
	}
	if err != nil {
		log.JSONLog("error", "Query failed", map[string]interface{}{"sql": p.SQL, "params": p.Params, "error": err})
		// Don't return here, we'll try Exec next
	}
	if err == nil {
		defer rows.Close() //nolint:errcheck // Standard pattern: error in deferred close is not critical
		cols, _ := rows.Columns()
		var results [][]interface{}
		// --- Type mapping logic start ---
		typeMap := make(map[string]string)
		// Try to extract table name from SQL if possible (simple SELECT ... FROM ...)
		tableName := ""
		sqlLower := strings.ToLower(p.SQL)
		if strings.HasPrefix(sqlLower, "select") {
			fromIdx := strings.Index(sqlLower, "from ")
			if fromIdx != -1 {
				afterFrom := sqlLower[fromIdx+5:]
				parts := strings.Fields(afterFrom)
				if len(parts) > 0 {
					tableName = strings.Trim(parts[0], "`")
				}
			}
		}
		if tableName != "" {
			// Query DESCRIBE to get types
			typeRows, err := conn.QueryContext(ctx, "DESCRIBE "+tableName) // NOSONAR
			if err == nil {
				for typeRows.Next() {
					var field, typ, null, key, def, extra string
					if err := typeRows.Scan(&field, &typ, &null, &key, &def, &extra); err == nil {
						typeMap[field] = typ
					}
				}
				if err := typeRows.Close(); err != nil {
					log.JSONLog("warn", "Failed to close describe rows", map[string]interface{}{"table": tableName, "error": err.Error()})
				}
			}
		}
		// --- Type mapping logic end ---
		for rows.Next() {
			row := make([]interface{}, len(cols))
			ptrs := make([]interface{}, len(cols))
			for i := range row {
				ptrs[i] = &row[i]
			}
			if err := rows.Scan(ptrs...); err != nil {
				return nil, nil, err
			}
			// Convert []byte to correct type for each column
			for i, v := range row {
				colName := cols[i]
				colType := typeMap[colName]
				switch val := v.(type) {
				case []byte:
					switch {
					case strings.HasPrefix(colType, "int"), strings.HasPrefix(colType, "tinyint"), strings.HasPrefix(colType, "bigint"), strings.HasPrefix(colType, "smallint"), strings.HasPrefix(colType, "mediumint"):
						// Integer types
						intVal, _ := strconv.ParseInt(string(val), 10, 64)
						row[i] = intVal
					case strings.HasPrefix(colType, "float"), strings.HasPrefix(colType, "double"), strings.HasPrefix(colType, "decimal"):
						// Float types
						floatVal, _ := strconv.ParseFloat(string(val), 64)
						row[i] = floatVal
					case strings.HasPrefix(colType, "date"), strings.HasPrefix(colType, "time"), strings.HasPrefix(colType, "timestamp"), strings.HasPrefix(colType, "datetime"):
						// Date/time types as string
						row[i] = string(val)
					default:
						// Default to string
						row[i] = string(val)
					}
				default:
					row[i] = val
				}
			}
			results = append(results, row)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: string(mustJSONMarshal(ExecuteSQLResult{
						Columns: cols,
						Rows:    results,
					})),
				},
			},
		}, nil, nil
	}
	// If not a query, try Exec
	var res sql.Result
	if len(p.Params) > 0 {
		stmt, err := conn.PrepareContext(ctx, p.SQL)
		if err != nil {
			return nil, nil, err
		}
		defer stmt.Close()                //nolint:errcheck // Standard pattern: error in deferred close is not critical
		res, err = stmt.Exec(p.Params...) //nolint:noctx // Prepared with PrepareContext, context already bound
		if err != nil {
			log.JSONLog("error", "Failed to execute prepared statement", map[string]interface{}{"sql": p.SQL, "params": p.Params, "error": err})
			structErr := s.errorAnalyzer.AnalyzeError(err, map[string]interface{}{
				"profile_name": p.ProfileName,
				"sql":          p.SQL,
				"operation":    "prepared_exec",
				"db_type":      prof.DBType,
			})
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{
						Text: structErr.ToJSON(),
					},
				},
			}, nil, nil
		}
	} else {
		res, err = conn.ExecContext(ctx, p.SQL)
	}
	if err != nil {
		log.JSONLog("error", "SQL execution failed", map[string]interface{}{"sql": p.SQL, "error": err})
		structErr := s.errorAnalyzer.AnalyzeError(err, map[string]interface{}{
			"profile_name":  p.ProfileName,
			"sql":           p.SQL,
			"query":         p.SQL,
			"operation":     "execute_sql",
			"db_type":       prof.DBType,
			"database_name": dbName,
		})
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: structErr.ToJSON(),
				},
			},
		}, nil, nil
	}
	affected, _ := res.RowsAffected()
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

// mustJSONMarshal is a helper for panic-free JSON marshaling.
func mustJSONMarshal(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}

func (s *MCPServer) handleListTables(ctx context.Context, _ *mcp.CallToolRequest, input ListTablesParams) (*mcp.CallToolResult, any, error) {
	p := input
	// Validate required parameters
	if p.ProfileName == "" || p.DatabaseName == "" {
		structErr := NewStructuredError(
			ErrorCodeMissingParameter,
			"Missing required parameters",
			"Both profile_name and database_name are required",
		).WithSuggestions(
			ErrorSuggestion{
				Action:      "Provide all required parameters",
				Description: "Ensure profile_name and database_name are included",
				Example:     `{"profile_name": "mydb", "database_name": "mydb"}`,
			},
		)
		return errorResult(structErr), nil, nil
	}
	cfg, prof, err := s.findProfile(p.ProfileName)
	if err != nil {
		if cfg == nil {
			structErr := NewStructuredError(
				ErrorCodeConfigNotFound,
				"Failed to load configuration",
				err.Error(),
			).WithSuggestions(
				ErrorSuggestion{
					Action:      "Initialize configuration",
					Description: "Run the server to create initial configuration",
				},
			)
			return errorResult(structErr), nil, nil
		}
		availableProfiles := make([]string, 0, len(cfg.Profiles))
		for _, profile := range cfg.Profiles {
			availableProfiles = append(availableProfiles, profile.ProfileName)
		}
		structErr := NewStructuredError(
			ErrorCodeProfileNotFound,
			fmt.Sprintf("Profile '%s' not found", p.ProfileName),
			"The specified database profile does not exist",
		).WithSuggestions(
			ErrorSuggestion{
				Tool:        "list-profiles",
				Description: "List all available database profiles",
			},
		).WithContext("available_profiles", availableProfiles)
		return errorResult(structErr), nil, nil
	}
	// Use requested database if provided, else profile default
	dbName := prof.DatabaseName
	if p.DatabaseName != "" {
		dbName = p.DatabaseName
	}
	dsn := db.DSN(prof.DBType, prof.Host, prof.Port, prof.Username, prof.Password, dbName, prof.SSLMode)
	conn, err := db.OpenConnectionWithPool(ctx, prof.DBType, dsn, cfg.MaxPoolSize)
	if err != nil {
		log.JSONLog("error", "Failed to open database connection", map[string]interface{}{"profile": p.ProfileName, "error": err})
		structErr := s.errorAnalyzer.AnalyzeError(err, map[string]interface{}{
			"profile_name": p.ProfileName,
			"operation":    "list_tables",
			"db_type":      prof.DBType,
		})
		return errorResult(structErr), nil, nil
	}
	defer conn.Close() //nolint:errcheck // Standard pattern: error in deferred close is not critical
	// For MySQL/MariaDB, optionally switch database if needed
	if result := s.switchMySQLDatabase(ctx, conn, prof, p.ProfileName, p.DatabaseName); result != nil {
		return result, nil, nil
	}
	var query string
	switch prof.DBType {
	case "mysql", "mariadb":
		query = "SHOW FULL TABLES"
	case "postgres":
		query = "SELECT table_name FROM information_schema.tables WHERE table_schema='public'"
	case "sqlite":
		query = sqliteListTablesQuery
	default:
		return nil, nil, fmt.Errorf("unsupported db_type")
	}
	rows, err := conn.QueryContext(ctx, query)
	if err != nil {
		log.JSONLog("error", "Failed to list tables", map[string]interface{}{"profile": p.ProfileName, "error": err})
		structErr := s.errorAnalyzer.AnalyzeError(err, map[string]interface{}{
			"profile_name":  p.ProfileName,
			"database_name": dbName,
			"operation":     "list_tables",
			"db_type":       prof.DBType,
		})
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: structErr.ToJSON(),
				},
			},
		}, nil, nil
	}
	defer rows.Close() //nolint:errcheck // Standard pattern: error in deferred close is not critical
	var tables []string
	for rows.Next() {
		if prof.DBType == "mysql" || prof.DBType == "mariadb" {
			var name, tableType string
			if err := rows.Scan(&name, &tableType); err != nil {
				return nil, nil, err
			}
			tables = append(tables, name)
		} else {
			var name string
			if err := rows.Scan(&name); err != nil {
				return nil, nil, err
			}
			tables = append(tables, name)
		}
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

func (s *MCPServer) handleDescribeTable(ctx context.Context, _ *mcp.CallToolRequest, input DescribeTableParams) (*mcp.CallToolResult, any, error) {
	p := input
	cfg, prof, err := s.findProfile(p.ProfileName)
	if err != nil {
		if cfg == nil {
			structErr := NewStructuredError(
				ErrorCodeConfigNotFound,
				"Failed to load configuration",
				err.Error(),
			).WithSuggestions(
				ErrorSuggestion{
					Action:      "Initialize configuration",
					Description: "Run the server to create initial configuration",
				},
			)
			return errorResult(structErr), nil, nil
		}
		return nil, nil, fmt.Errorf("profile not found")
	}
	// Always require both database name and table name from user
	if p.DatabaseName == "" || p.TableName == "" {
		structErr := NewStructuredError(
			ErrorCodeMissingParameter,
			"Missing required parameters",
			"Both database_name and table_name are required",
		).WithSuggestions(
			ErrorSuggestion{
				Action:      "Provide all required parameters",
				Description: "Specify both database_name and table_name",
				Example:     `{"profile_name": "mydb", "database_name": "mydb", "table_name": "users"}`,
			},
		)
		return errorResult(structErr), nil, nil
	}
	dsn := db.DSN(prof.DBType, prof.Host, prof.Port, prof.Username, prof.Password, p.DatabaseName, prof.SSLMode)
	conn, err := db.OpenConnectionWithPool(ctx, prof.DBType, dsn, cfg.MaxPoolSize)
	if err != nil {
		log.JSONLog("error", "Failed to connect to database", map[string]interface{}{"error": err})
		structErr := s.errorAnalyzer.AnalyzeError(err, map[string]interface{}{
			"profile_name":  p.ProfileName,
			"database_name": p.DatabaseName,
			"table_name":    p.TableName,
			"operation":     "describe_table",
			"db_type":       prof.DBType,
		})
		return errorResult(structErr), nil, nil
	}
	defer conn.Close() //nolint:errcheck // Standard pattern: error in deferred close is not critical

	var columns []ColumnInfo
	var query string

	switch prof.DBType {
	case "mysql", "mariadb":
		// Enhanced MySQL/MariaDB query with comments and metadata
		query = `SELECT
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

		rows, err := conn.QueryContext(ctx, query, p.DatabaseName, p.TableName)
		if err != nil {
			log.JSONLog("error", "Failed to describe table", map[string]interface{}{"table": p.TableName, "error": err})
			structErr := s.errorAnalyzer.AnalyzeError(err, map[string]interface{}{
				"profile_name":  p.ProfileName,
				"database_name": p.DatabaseName,
				"table_name":    p.TableName,
				"operation":     "describe_table",
				"db_type":       prof.DBType,
			})
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{
						Text: structErr.ToJSON(),
					},
				},
			}, nil, nil
		}
		defer rows.Close() //nolint:errcheck // Standard pattern: error in deferred close is not critical

		for rows.Next() {
			var name, typ, nullable, keyType, extra string
			var defaultVal, comment, characterSet, collation sql.NullString
			var maxLength, precision, scale sql.NullInt64

			if err := rows.Scan(&name, &typ, &nullable, &keyType, &defaultVal, &comment, &extra, &characterSet, &collation, &maxLength, &precision, &scale); err != nil {
				return nil, nil, err
			}

			col := ColumnInfo{
				Name:          name,
				Type:          typ,
				Nullable:      nullable == "YES",
				Key:           keyType,
				Extra:         extra,
				AutoIncrement: strings.Contains(extra, "auto_increment"),
			}

			if defaultVal.Valid {
				col.Default = &defaultVal.String
			}
			if comment.Valid {
				col.Comment = comment.String
			}
			if characterSet.Valid {
				col.CharacterSet = characterSet.String
			}
			if collation.Valid {
				col.Collation = collation.String
			}
			if maxLength.Valid {
				col.MaxLength = &maxLength.Int64
			}
			if precision.Valid {
				col.Precision = &precision.Int64
			}
			if scale.Valid {
				col.Scale = &scale.Int64
			}

			columns = append(columns, col)
		}

	case "postgres":
		// Enhanced PostgreSQL query with comments and metadata
		query = `SELECT
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
		WHERE c.table_schema = $1 AND c.table_name = $2
		ORDER BY c.ordinal_position`

		// Use 'public' as default schema for PostgreSQL
		schema := "public"
		rows, err := conn.QueryContext(ctx, query, schema, p.TableName)
		if err != nil {
			log.JSONLog("error", "Failed to describe table", map[string]interface{}{"table": p.TableName, "error": err})
			structErr := s.errorAnalyzer.AnalyzeError(err, map[string]interface{}{
				"profile_name":  p.ProfileName,
				"database_name": p.DatabaseName,
				"table_name":    p.TableName,
				"operation":     "describe_table",
				"db_type":       prof.DBType,
			})
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{
						Text: structErr.ToJSON(),
					},
				},
			}, nil, nil
		}
		defer rows.Close() //nolint:errcheck // Standard pattern: error in deferred close is not critical

		for rows.Next() {
			var name, typ, nullable, keyType string
			var defaultVal, comment sql.NullString
			var maxLength, precision, scale sql.NullInt64

			if err := rows.Scan(&name, &typ, &nullable, &defaultVal, &comment, &maxLength, &precision, &scale, &keyType); err != nil {
				return nil, nil, err
			}

			col := ColumnInfo{
				Name:          name,
				Type:          typ,
				Nullable:      nullable == "YES",
				Key:           keyType,
				AutoIncrement: strings.Contains(defaultVal.String, "nextval"),
			}

			if defaultVal.Valid {
				col.Default = &defaultVal.String
			}
			if comment.Valid {
				col.Comment = comment.String
			}
			if maxLength.Valid {
				col.MaxLength = &maxLength.Int64
			}
			if precision.Valid {
				col.Precision = &precision.Int64
			}
			if scale.Valid {
				col.Scale = &scale.Int64
			}

			columns = append(columns, col)
		}

	case "sqlite":
		// SQLite uses PRAGMA table_xinfo for extended information
		query = fmt.Sprintf("PRAGMA table_xinfo('%s')", p.TableName)

		rows, err := conn.QueryContext(ctx, query)
		if err != nil {
			log.JSONLog("error", "Failed to list databases", map[string]interface{}{"error": err})
			structErr := s.errorAnalyzer.AnalyzeError(err, map[string]interface{}{
				"profile_name": p.ProfileName,
				"operation":    "list_databases",
				"db_type":      prof.DBType,
			})
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{
						Text: structErr.ToJSON(),
					},
				},
			}, nil, nil
		}
		defer rows.Close() //nolint:errcheck // Standard pattern: error in deferred close is not critical

		for rows.Next() {
			var cid, notnull, pk, hidden int
			var name, typ string
			var dflt sql.NullString

			if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk, &hidden); err != nil {
				return nil, nil, err
			}

			col := ColumnInfo{
				Name:          name,
				Type:          typ,
				Nullable:      notnull == 0,
				AutoIncrement: false, // SQLite doesn't have traditional auto_increment
			}

			if pk > 0 {
				col.Key = "PRI"
			}

			if dflt.Valid {
				col.Default = &dflt.String
			}

			// SQLite doesn't support column comments natively
			col.Comment = ""

			columns = append(columns, col)
		}

	default:
		return nil, nil, fmt.Errorf("unsupported db_type")
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

func (s *MCPServer) handleListDatabases(ctx context.Context, _ *mcp.CallToolRequest, input ListDatabasesParams) (*mcp.CallToolResult, any, error) {
	p := input
	cfg, prof, err := s.findProfile(p.ProfileName)
	if err != nil {
		log.JSONLog("error", "Failed to load configuration", map[string]interface{}{"error": err})
		if cfg == nil {
			structErr := NewStructuredError(
				ErrorCodeConfigNotFound,
				"Failed to load configuration",
				err.Error(),
			).WithSuggestions(
				ErrorSuggestion{
					Action:      "Initialize configuration",
					Description: "Run the server to create initial configuration",
				},
			)
			return errorResult(structErr), nil, nil
		}
		log.JSONLog("error", "Profile not found", map[string]interface{}{"profile_name": p.ProfileName})
		structErr := NewStructuredError(
			ErrorCodeProfileNotFound,
			fmt.Sprintf("Profile '%s' not found", p.ProfileName),
			"The specified database profile does not exist",
		).WithSuggestions(
			ErrorSuggestion{
				Tool:        "list-profiles",
				Description: "List all available database profiles",
			},
			ErrorSuggestion{
				Tool:        "configure-profile",
				Description: "Create a new database profile",
				Example:     fmt.Sprintf(`{"profile_name": "%s", "db_type": "mysql", ...}`, p.ProfileName),
			},
		)
		return errorResult(structErr), nil, nil
	}
	dsn := db.DSN(prof.DBType, prof.Host, prof.Port, prof.Username, prof.Password, prof.DatabaseName, prof.SSLMode)
	conn, err := db.OpenConnectionWithPool(ctx, prof.DBType, dsn, cfg.MaxPoolSize)
	if err != nil {
		log.JSONLog("error", "Failed to connect to database", map[string]interface{}{"error": err})
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
		return nil, nil, fmt.Errorf("unsupported db_type")
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
	// Input validation
	cfg, prof, err := s.findProfile(p.ProfileName)
	if err != nil {
		if cfg == nil {
			log.JSONLog("error", "Failed to load config for sample data", map[string]interface{}{"error": err.Error()})
			structErr := NewStructuredError(
				ErrorCodeConfigNotFound,
				"Failed to load configuration",
				err.Error(),
			).WithSuggestions(
				ErrorSuggestion{
					Action:      "Initialize configuration",
					Description: "Run the server to create initial configuration",
				},
			)
			return errorResult(structErr), nil, nil
		}
		log.JSONLog("error", "Profile not found for sample data", map[string]interface{}{"profile_name": p.ProfileName})
		return nil, nil, fmt.Errorf("profile not found")
	}
	switch prof.DBType {
	case "mysql", "mariadb", "postgres", "sqlite":
		// Supported, continue
	default:
		log.JSONLog("error", "Unsupported database type for sample data", map[string]interface{}{"db_type": prof.DBType})
		structErr := NewStructuredError(
			ErrorCodeUnsupportedDB,
			fmt.Sprintf("Unsupported database type: %s", prof.DBType),
			"Sample data is only supported for MySQL, MariaDB, PostgreSQL, and SQLite",
		).WithContext("supported_types", []string{"mysql", "mariadb", "postgres", "sqlite"})
		return errorResult(structErr), nil, fmt.Errorf("unsupported db_type for sample data")
	}
	if p.ProfileName == "" || p.DatabaseName == "" || p.TableName == "" {
		structErr := NewStructuredError(
			ErrorCodeMissingParameter,
			"Missing required parameters",
			"All of profile_name, database_name, and table_name are required",
		).WithSuggestions(
			ErrorSuggestion{
				Action:      "Provide all required parameters",
				Description: "Ensure profile_name, database_name, and table_name are included",
				Example:     `{"profile_name": "mydb", "database_name": "mydb", "table_name": "users", "sample_size": 5}`,
			},
		)
		return errorResult(structErr), nil, fmt.Errorf("missing required parameters")
	}

	// Set default sample size if not provided
	sampleSize := p.SampleSize
	if sampleSize <= 0 {
		sampleSize = 3 // Default to 3 rows
	}
	if sampleSize > 100 {
		sampleSize = 100 // Cap at 100 rows for performance
	}

	// 1. Determine database name
	dbName := prof.DatabaseName
	if p.DatabaseName != "" {
		dbName = p.DatabaseName
	}

	// 2. Connect to database
	dsn := db.DSN(prof.DBType, prof.Host, prof.Port, prof.Username, prof.Password, dbName, prof.SSLMode)
	conn, err := db.OpenConnectionWithPool(ctx, prof.DBType, dsn, cfg.MaxPoolSize)
	if err != nil {
		log.JSONLog("error", "Failed to connect for sample data", map[string]interface{}{"error": err.Error(), "profile": p.ProfileName})
		structErr := s.errorAnalyzer.AnalyzeError(err, map[string]interface{}{
			"profile_name":  p.ProfileName,
			"operation":     "sample_data",
			"db_type":       prof.DBType,
			"database_name": dbName,
		})
		return errorResult(structErr), nil, nil
	}
	defer conn.Close() //nolint:errcheck // Standard pattern: error in deferred close is not critical

	// 3. Switch database context for MySQL/MariaDB if database_name is specified
	if result := s.switchMySQLDatabase(ctx, conn, prof, p.ProfileName, p.DatabaseName); result != nil {
		return result, nil, nil
	}

	// 4. Build sample query based on database type
	var sampleQuery string
	switch prof.DBType {
	case "mysql", "mariadb":
		sampleQuery = fmt.Sprintf("SELECT * FROM `%s` LIMIT %d", p.TableName, sampleSize)
	case "postgres":
		sampleQuery = fmt.Sprintf("SELECT * FROM \"%s\" LIMIT %d", p.TableName, sampleSize)
	case "sqlite":
		sampleQuery = fmt.Sprintf("SELECT * FROM '%s' LIMIT %d", p.TableName, sampleSize)
	default:
		log.JSONLog("error", "Unsupported database type for sample data", map[string]interface{}{"db_type": prof.DBType})
		structErr := NewStructuredError(
			ErrorCodeUnsupportedDB,
			fmt.Sprintf("Unsupported database type: %s", prof.DBType),
			"Sample data is only supported for MySQL, MariaDB, PostgreSQL, and SQLite",
		).WithContext("supported_types", []string{"mysql", "mariadb", "postgres", "sqlite"})
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: structErr.ToJSON(),
				},
			},
		}, nil, fmt.Errorf("unsupported db_type for sample data")
	}

	// 6. Execute sample query
	log.JSONLog("debug", "Executing sample data query", map[string]interface{}{"query": sampleQuery, "table": p.TableName})
	rows, err := conn.QueryContext(ctx, sampleQuery)
	if err != nil {
		log.JSONLog("error", "Sample data query failed", map[string]interface{}{"query": sampleQuery, "error": err.Error()})
		structErr := s.errorAnalyzer.AnalyzeError(err, map[string]interface{}{
			"profile_name":  p.ProfileName,
			"table_name":    p.TableName,
			"database_name": dbName,
			"operation":     "sample_data",
			"db_type":       prof.DBType,
			"query":         sampleQuery,
		})
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: structErr.ToJSON(),
				},
			},
		}, nil, nil
	}
	defer rows.Close() //nolint:errcheck // Standard pattern: error in deferred close is not critical

	// 7. Extract column names
	columns, err := rows.Columns()
	if err != nil {
		log.JSONLog("error", "Failed to get column names for sample data", map[string]interface{}{"table": p.TableName, "error": err.Error()})
		return nil, nil, err
	}

	// 8. Fetch sample rows
	var sampleRows [][]interface{}
	for rows.Next() {
		row := make([]interface{}, len(columns))
		ptrs := make([]interface{}, len(columns))
		for i := range row {
			ptrs[i] = &row[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			log.JSONLog("error", "Failed to scan sample data row", map[string]interface{}{"table": p.TableName, "error": err.Error()})
			return nil, nil, err
		}

		// Convert []byte to strings for better JSON representation
		for i, v := range row {
			if bytes, ok := v.([]byte); ok {
				row[i] = string(bytes)
			}
		}
		sampleRows = append(sampleRows, row)
	}

	// 9. Check for iteration errors
	if err := rows.Err(); err != nil {
		log.JSONLog("error", "Error during sample data iteration", map[string]interface{}{"table": p.TableName, "error": err.Error()})
		return nil, nil, err
	}

	// 10. Build result
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

// handleAnalyzeSchema implements the MCP handler for comprehensive schema analysis.
func (s *MCPServer) handleAnalyzeSchema(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input AnalyzeSchemaParams,
) (*mcp.CallToolResult, any, error) {
	startTime := time.Now()
	p := input
	if err := p.Validate(); err != nil {
		structErr := NewStructuredError(
			ErrorCodeMissingParameter,
			"Missing or invalid required parameter",
			err.Error(),
		).WithSuggestions(
			ErrorSuggestion{
				Action:      "Specify analysis_level",
				Description: "analysis_level is required and must be one of: basic, detailed, comprehensive",
				Example:     `{"profile_name": "analytics_db", "analysis_level": "detailed"}`,
			},
		)
		return errorResult(structErr), nil, fmt.Errorf("missing or invalid analysis_level")
	}

	// 1. Load config and profile
	cfg, prof, err := s.findProfile(p.ProfileName)
	if err != nil {
		if cfg == nil {
			structErr := NewStructuredError(
				ErrorCodeConfigNotFound,
				"Failed to load configuration",
				err.Error(),
			)
			return errorResult(structErr), nil, err
		}
		structErr := NewStructuredError(
			ErrorCodeProfileNotFound,
			fmt.Sprintf("Profile '%s' not found", p.ProfileName),
			"The specified database profile does not exist",
		)
		return errorResult(structErr), nil, fmt.Errorf("profile not found")
	}
	dbName := prof.DatabaseName
	if p.DatabaseName != "" {
		dbName = p.DatabaseName
	}

	// 2. Connect to database
	dsn := db.DSN(prof.DBType, prof.Host, prof.Port, prof.Username, prof.Password, dbName, prof.SSLMode)
	conn, err := db.OpenConnectionWithPool(ctx, prof.DBType, dsn, cfg.MaxPoolSize)
	if err != nil {
		log.JSONLog("error", "Failed to connect to database", map[string]interface{}{"error": err})
		structErr := s.errorAnalyzer.AnalyzeError(err, map[string]interface{}{
			"profile_name": p.ProfileName,
			"operation":    "analyze_schema",
			"db_type":      prof.DBType,
		})
		return errorResult(structErr), nil, err
	}
	defer conn.Close() //nolint:errcheck // Standard pattern: error in deferred close is not critical

	// 3. Table list (reuse handleListTables logic)
	var tables []string
	{
		var query string
		switch prof.DBType {
		case "mysql", "mariadb":
			query = "SHOW FULL TABLES"
		case "postgres":
			query = "SELECT table_name FROM information_schema.tables WHERE table_schema='public'"
		case "sqlite":
			query = sqliteListTablesQuery
		default:
			return nil, nil, fmt.Errorf("unsupported db_type")
		}
		rows, err := conn.QueryContext(ctx, query)
		if err != nil {
			log.JSONLog("error", "Failed to list tables", map[string]interface{}{"error": err})
			structErr := s.errorAnalyzer.AnalyzeError(err, map[string]interface{}{
				"profile_name":  p.ProfileName,
				"database_name": dbName,
				"operation":     "list_tables",
				"db_type":       prof.DBType,
			})
			return errorResult(structErr), nil, err
		}
		defer rows.Close() //nolint:errcheck // Standard pattern: error in deferred close is not critical
		for rows.Next() {
			if prof.DBType == "mysql" || prof.DBType == "mariadb" {
				var name, tableType string
				if err := rows.Scan(&name, &tableType); err == nil {
					tables = append(tables, name)
				} else {
					log.JSONLog("warn", "Failed to scan table name during analysis", map[string]interface{}{"db_type": prof.DBType, "error": err.Error(), "operation": "analyze_schema_list_tables"})
				}
			} else {
				var name string
				if err := rows.Scan(&name); err == nil {
					tables = append(tables, name)
				} else {
					log.JSONLog("warn", "Failed to scan table name during analysis", map[string]interface{}{"db_type": prof.DBType, "error": err.Error(), "operation": "analyze_schema_list_tables"})
				}
			}
		}
	}

	// Filter tables if include/exclude specified
	tableSet := map[string]bool{}
	if len(p.IncludeTables) > 0 {
		for _, t := range p.IncludeTables {
			tableSet[strings.ToLower(t)] = true
		}
	}
	excludeSet := map[string]bool{}
	if len(p.ExcludeTables) > 0 {
		for _, t := range p.ExcludeTables {
			excludeSet[strings.ToLower(t)] = true
		}
	}
	filteredTables := []string{}
	for _, t := range tables {
		tLower := strings.ToLower(t)
		if len(tableSet) > 0 && !tableSet[tLower] {
			continue
		}
		if excludeSet[tLower] {
			continue
		}
		filteredTables = append(filteredTables, t)
	}
	if len(filteredTables) == 0 {
		filteredTables = tables
	}

	// 4. Table schemas and sample data
	tableSchemas := map[string]TableInfo{}
	relationshipCandidates := map[string]TableInfo{}
	sampleDataMap := map[string][]map[string]interface{}{}
	totalColumns := 0
	sampleSize := p.SampleSize
	if sampleSize <= 0 {
		sampleSize = 10
	}
	for _, tbl := range filteredTables {
		// Describe table (reuse handleDescribeTable logic)
		var columns []SchemaColumnInfo
		var keyCols KeyColumns
		var colQuery string
		switch prof.DBType {
		case "mysql", "mariadb":
			colQuery = fmt.Sprintf("SHOW COLUMNS FROM `%s`", tbl)
		case "postgres":
			// Enriched PostgreSQL metadata query (includes nullability, default, constraint type)
			colQuery = fmt.Sprintf(`SELECT
				c.column_name,
				c.data_type,
				c.is_nullable,
				c.column_default,
				COALESCE(tc.constraint_type,'') as constraint_type
			FROM information_schema.columns c
			LEFT JOIN information_schema.key_column_usage kcu
			  ON kcu.table_name = c.table_name AND kcu.table_schema = c.table_schema AND kcu.column_name = c.column_name
			LEFT JOIN information_schema.table_constraints tc
			  ON tc.constraint_name = kcu.constraint_name AND tc.table_schema = kcu.table_schema
			WHERE c.table_name = '%s' AND c.table_schema='public'
			ORDER BY c.ordinal_position`, tbl)
		case "sqlite":
			colQuery = fmt.Sprintf("PRAGMA table_info('%s')", tbl)
		default:
			continue
		}
		rows, err := conn.QueryContext(ctx, colQuery)
		if err != nil {
			log.JSONLog("warn", "Failed to query column metadata", map[string]interface{}{"table": tbl, "error": err.Error(), "db_type": prof.DBType})
			continue
		}
		for rows.Next() {
			switch prof.DBType {
			case "mysql", "mariadb":
				var field, typ, null, key, def, extra string
				if err := rows.Scan(&field, &typ, &null, &key, &def, &extra); err != nil {
					log.JSONLog("warn", "Scan column metadata failed", map[string]interface{}{"table": tbl, "error": err.Error(), "db_type": prof.DBType})
					continue
				}
				colInfo := SchemaColumnInfo{
					ColumnName:   field,
					DataType:     typ,
					IsNullable:   null == "YES",
					IsPrimaryKey: key == "PRI",
					Unique:       key == "UNI",
					Indexed:      key == "MUL",
					DefaultValue: def,
					Description:  extra,
				}
				columns = append(columns, colInfo)
				if key == "PRI" {
					keyCols.PrimaryKey = field
				}
				if key == "MUL" {
					keyCols.IndexedColumns = append(keyCols.IndexedColumns, field)
				}
				if key == "UNI" {
					keyCols.UniqueColumns = append(keyCols.UniqueColumns, field)
				}
			case "postgres":
				var columnName, dataType, isNullable string
				var defaultVal sql.NullString
				var constraintType string
				if err := rows.Scan(&columnName, &dataType, &isNullable, &defaultVal, &constraintType); err != nil {
					log.JSONLog("warn", "Scan PostgreSQL column metadata failed", map[string]interface{}{"table": tbl, "error": err.Error()})
					continue
				}
				colInfo := SchemaColumnInfo{
					ColumnName: columnName,
					DataType:   dataType,
					IsNullable: isNullable == "YES",
				}
				if defaultVal.Valid {
					colInfo.DefaultValue = defaultVal.String
				}
				switch constraintType {
				case "PRIMARY KEY":
					colInfo.IsPrimaryKey = true
					keyCols.PrimaryKey = columnName
				case "UNIQUE":
					colInfo.Unique = true
					keyCols.UniqueColumns = append(keyCols.UniqueColumns, columnName)
				case "FOREIGN KEY":
					colInfo.IsForeignKey = true
					keyCols.ForeignKeys = append(keyCols.ForeignKeys, columnName)
				}
				columns = append(columns, colInfo)
			case "sqlite":
				var cid int
				var name, typ string
				var notnull, pk int
				var dflt interface{}
				if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
					log.JSONLog("warn", "Scan SQLite column metadata failed", map[string]interface{}{"table": tbl, "error": err.Error()})
					continue
				}
				colInfo := SchemaColumnInfo{
					ColumnName:   name,
					DataType:     typ,
					IsNullable:   notnull == 0,
					IsPrimaryKey: pk == 1,
					DefaultValue: dflt,
				}
				columns = append(columns, colInfo)
				if pk == 1 {
					keyCols.PrimaryKey = name
				}
			}
		}
		if err := rows.Close(); err != nil {
			log.JSONLog("warn", "Failed to close column metadata rows", map[string]interface{}{"table": tbl, "error": err.Error()})
		}
		totalColumns += len(columns)
		relationshipCandidates[tbl] = TableInfo{
			ColumnCount: len(columns),
			KeyColumns:  keyCols,
			Columns:     columns,
		}

		// Sample data (reuse handleSampleData logic)
		var sampleQuery string
		switch prof.DBType {
		case "mysql", "mariadb":
			sampleQuery = fmt.Sprintf("SELECT * FROM `%s` LIMIT %d", tbl, sampleSize)
		case "postgres":
			sampleQuery = fmt.Sprintf("SELECT * FROM \"%s\" LIMIT %d", tbl, sampleSize)
		case "sqlite":
			sampleQuery = fmt.Sprintf("SELECT * FROM '%s' LIMIT %d", tbl, sampleSize)
		default:
			continue
		}
		sampleRows := []map[string]interface{}{}
		sampleRowsRaw, err := conn.QueryContext(ctx, sampleQuery)
		if err == nil {
			cols, _ := sampleRowsRaw.Columns()
			for sampleRowsRaw.Next() {
				row := make([]interface{}, len(cols))
				ptrs := make([]interface{}, len(cols))
				for i := range row {
					ptrs[i] = &row[i]
				}
				if err := sampleRowsRaw.Scan(ptrs...); err == nil {
					rowMap := map[string]interface{}{}
					for i, v := range row {
						if bytes, ok := v.([]byte); ok {
							rowMap[cols[i]] = string(bytes)
						} else {
							rowMap[cols[i]] = v
						}
					}
					sampleRows = append(sampleRows, rowMap)
				} else {
					log.JSONLog("warn", "Failed to scan sample row during analysis", map[string]interface{}{"table": tbl, "error": err.Error()})
				}
			}
			if err := sampleRowsRaw.Err(); err != nil {
				log.JSONLog("warn", "Iteration error while reading sample rows during analysis", map[string]interface{}{"table": tbl, "error": err.Error()})
			}
			if err := sampleRowsRaw.Close(); err != nil {
				log.JSONLog("warn", "Failed to close sample rows during analysis", map[string]interface{}{"table": tbl, "error": err.Error()})
			}
		} else {
			log.JSONLog("warn", "Failed to fetch sample rows during analysis", map[string]interface{}{"table": tbl, "error": err.Error(), "query": sampleQuery})
		}
		sampleDataMap[tbl] = sampleRows
		tableSchemas[tbl] = TableInfo{
			ColumnCount:  len(columns),
			KeyColumns:   keyCols,
			Columns:      columns,
			DataPatterns: map[string]DataPattern{}, // Could fill with analyzeDataPatterns
		}
	}

	// 5. Relationship discovery
	relGraph := s.mapSemanticRelationships(relationshipCandidates, nil)
	// Build visual graph representation immediately for inclusion in result payload
	relGraphVisual := s.buildRelationshipGraph(relGraph)

	// 6. Business context inference (comprehensive only)
	var businessCtx *BusinessContext
	var domain string
	var confidence float64
	var businessDesc string
	if p.AnalysisLevel == AnalysisLevelComprehensive {
		businessCtx = s.inferBusinessContext(relationshipCandidates)
		for k, v := range businessCtx.DomainIndicators {
			domain = k
			confidence = v
			businessDesc = s.generateBusinessDescription(domain, businessCtx.EntityRelationships.CentralEntities, confidence)
			break
		}
		// --- Advanced helpers integration ---
		// 1. Data pattern analysis for each table
		for tbl, schema := range tableSchemas {
			tableSample := sampleDataMap[tbl]
			// Copy existing schema
			tableSchemas[tbl] = TableInfo{
				ColumnCount:  schema.ColumnCount,
				KeyColumns:   schema.KeyColumns,
				Columns:      schema.Columns,
				DataPatterns: map[string]DataPattern{},
			}
			patterns := s.analyzeDataPatterns(tbl, tableSample, schema.Columns)
			for i, col := range schema.Columns {
				if i < len(patterns) {
					dp := patterns[i]
					tableSchemas[tbl].DataPatterns[col.ColumnName] = dp
					// Propagate pattern metrics onto column structure for downstream quality metrics
					for ci := range tableSchemas[tbl].Columns {
						if tableSchemas[tbl].Columns[ci].ColumnName == col.ColumnName {
							tableSchemas[tbl].Columns[ci].PatternType = dp.PatternType
							tableSchemas[tbl].Columns[ci].ValidationRegex = dp.ValidationRegex
							tableSchemas[tbl].Columns[ci].Uniqueness = dp.Uniqueness
							tableSchemas[tbl].Columns[ci].NullPercentage = dp.NullPercentage
							tableSchemas[tbl].Columns[ci].Distribution = dp.Distribution
							break
						}
					}
				}
			}
		}
		// 2. Correlate data values between tables for relationship confidence
		for srcName, srcSchema := range tableSchemas {
			for tgtName := range tableSchemas {
				if srcName == tgtName {
					continue
				}
				conf := s.correlateDataValues(srcName, srcSchema, tgtName, sampleDataMap)
				_ = conf
			}
		}
	}

	// 7. Query suggestions (comprehensive only)
	var aiQuerySuggestions AIQuerySuggestions
	if p.AnalysisLevel == AnalysisLevelComprehensive && p.IncludeQueries {
		// Example: Generate one suggestion per table
		for _, tbl := range filteredTables {
			suggestion, err := s.generateQuerySuggestionViaSmartBuilder(ctx, nil, p.ProfileName, fmt.Sprintf("Show all rows in %s", tbl), dbName, []string{tbl})
			if err == nil && suggestion != nil {
				aiQuerySuggestions.DataExploration = append(aiQuerySuggestions.DataExploration, QuerySuggestion{
					Category:   "exploration",
					Question:   fmt.Sprintf("Show all rows in %s", tbl),
					SQL:        suggestion.SQL,
					Complexity: "easy",
				})
			}
		}
	}

	// 8. Assemble result
	dataQualityMetrics := make(map[string]QualityMetrics)
	if p.AnalysisLevel == AnalysisLevelDetailed || p.AnalysisLevel == AnalysisLevelComprehensive {
		for tbl, schema := range tableSchemas {
			metrics := s.generateDataQualityMetrics(sampleDataMap[tbl], schema.Columns)
			// Aggregate table-level metrics (average of columns)
			if len(metrics) > 0 {
				var sum, count float64
				for _, qm := range metrics {
					sum += qm.OverallScore
					count++
				}
				avg := 0.0
				if count > 0 {
					avg = sum / count
				}
				metrics["__table__"] = QualityMetrics{
					OverallScore: avg,
					Issues:       []string{},
				}
			}
			// Flatten into main map with table.column keys
			for col, qm := range metrics {
				key := tbl
				if col != "__table__" {
					key += "." + col
				}
				dataQualityMetrics[key] = qm
			}
		}
		// Database-level aggregate
		var dbSum, dbCount float64
		for k, qm := range dataQualityMetrics {
			if !strings.HasSuffix(k, "__table__") {
				dbSum += qm.OverallScore
				dbCount++
			}
		}
		if dbCount > 0 {
			dataQualityMetrics["__database__"] = QualityMetrics{
				OverallScore: dbSum / dbCount,
				Issues:       []string{},
			}
		}
	}

	// Categorize tables for TableCatalog
	tableCatalog := s.categorizeTables(filteredTables, tableSchemas)
	log.JSONLog("info", "Assembling AnalyzeSchemaResult", map[string]interface{}{
		"tables":         len(filteredTables),
		"columns":        totalColumns,
		"relationships":  len(relGraph.ForeignKeys) + len(relGraph.SemanticRelationships),
		"analysis_level": p.AnalysisLevel,
	})
	result := AnalyzeSchemaResult{
		AnalysisMetadata: AnalysisMetadata{
			AnalysisLevel:      p.AnalysisLevel,
			DatabaseType:       prof.DBType,
			AnalysisTimestamp:  startTime,
			ToolsUsed:          []string{"list-tables", "describe-table", "sample-data", "discover-joins"},
			AnalysisDurationMs: int(time.Since(startTime).Milliseconds()),
		},
		DatabaseOverview: DatabaseOverview{
			DatabaseCount:           1,
			TotalTables:             len(filteredTables),
			TotalColumns:            totalColumns,
			TotalRelationships:      len(relGraph.ForeignKeys) + len(relGraph.SemanticRelationships),
			EstimatedBusinessDomain: domain,
			ConfidenceScore:         confidence,
			BusinessModelInsights:   []string{},
			Summary:                 fmt.Sprintf("Analyzed %d tables and %d columns.", len(filteredTables), totalColumns),
		},
		TableCatalog:            tableCatalog,
		TableSchemas:            tableSchemas,
		RelationshipGraph:       relGraph,
		RelationshipGraphVisual: relGraphVisual,
		BusinessContext:         BusinessContext{},
		AIQuerySuggestions:      aiQuerySuggestions,
		DataQualityMetrics:      dataQualityMetrics,
		QuickInsights:           []string{fmt.Sprintf("Schema analysis completed for %d tables.", len(filteredTables))},
	}

	if businessCtx != nil {
		result.BusinessContext = *businessCtx
		// Add business description summary
		if result.DatabaseOverview.BusinessModelInsights == nil {
			result.DatabaseOverview.BusinessModelInsights = []string{}
		}
		result.DatabaseOverview.BusinessModelInsights = append(result.DatabaseOverview.BusinessModelInsights, businessDesc)
	}

	b, err := json.Marshal(result)
	if err != nil {
		log.JSONLog("error", "Failed to serialize AnalyzeSchemaResult", map[string]interface{}{"error": err.Error()})
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

// Business Context Inference Helpers

func (s *MCPServer) inferBusinessContext(tableSchemas map[string]TableInfo) *BusinessContext {
	// Collect table names
	tableNames := make([]string, 0, len(tableSchemas))
	tables := make([]TableInfo, 0, len(tableSchemas))
	for name, t := range tableSchemas {
		tableNames = append(tableNames, name)
		tables = append(tables, t)
	}

	domain, confidence := s.detectDomain(tableNames)
	naming := s.analyzeNamingConventions(tables)
	entities := s.identifyEntityTypes(tableNames)
	configuredDomains := loadConfiguredDomains(s.ConfigPath)

	// Defensive extraction helpers
	getString := func(m map[string]interface{}, key, def string) string {
		if v, ok := m[key]; ok {
			if s, ok2 := v.(string); ok2 {
				return s
			}
		}
		return def
	}
	getFloat := func(m map[string]interface{}, key string, def float64) float64 {
		if v, ok := m[key]; ok {
			switch tv := v.(type) {
			case float64:
				return tv
			case float32:
				return float64(tv)
			case int:
				return float64(tv)
			case int64:
				return float64(tv)
			}
		}
		return def
	}
	getStringSlice := func(m map[string]interface{}, key string) []string {
		if v, ok := m[key]; ok {
			switch arr := v.(type) {
			case []string:
				return arr
			case []interface{}:
				var out []string
				for _, iv := range arr {
					out = append(out, fmt.Sprintf("%v", iv))
				}
				return out
			}
		}
		return []string{}
	}

	pattern := getString(naming, "main_case", "unknown")
	consistencyScore := getFloat(naming, "consistency", 0.0)
	fkPattern := getString(naming, "fk_pattern", "unknown")
	auditCols := getStringSlice(naming, "timestampCols")

	return &BusinessContext{
		DomainIndicators: mergeDomainIndicators(domain, confidence, configuredDomains),
		NamingConventions: NamingConventions{
			Pattern:           pattern,
			ConsistencyScore:  consistencyScore,
			ForeignKeyPattern: fkPattern,
			AuditColumns:      auditCols,
		},
		EntityRelationships: EntityRelationships{
			CentralEntities:      entities,
			RelationshipDensity:  0.0, // Placeholder
			MaxRelationshipDepth: 0,   // Placeholder
		},
		DataPatterns: map[string]DataPattern{}, // Placeholder
	}
}

func (s *MCPServer) detectDomain(tableNames []string) (string, float64) {
	domainPatterns := map[string][]string{
		"e-commerce":         {"product", "order", "cart", "customer", "payment", "inventory"},
		"healthcare":         {"patient", "doctor", "appointment", "medical", "prescription", "diagnosis"},
		"finance":            {"account", "transaction", "ledger", "invoice", "payment", "balance"},
		"crm":                {"lead", "contact", "opportunity", "customer", "activity"},
		"project-management": {"project", "task", "milestone", "issue", "sprint"},
		"education":          {"student", "course", "grade", "teacher", "enrollment"},
		"logistics":          {"shipment", "warehouse", "delivery", "route", "tracking"},
	}
	domainScores := make(map[string]float64)
	for domain, patterns := range domainPatterns {
		score := 0.0
		for _, name := range tableNames {
			for _, pat := range patterns {
				if strings.Contains(strings.ToLower(name), pat) {
					score += 1.0
				}
			}
		}
		domainScores[domain] = score / float64(len(patterns))
	}
	var bestDomain string
	var bestScore float64
	for domain, score := range domainScores {
		if score > bestScore {
			bestDomain = domain
			bestScore = score
		}
	}
	if bestScore == 0 {
		return "unknown", 0.0
	}
	return bestDomain, bestScore
}

func loadConfiguredDomains(configPath string) []string {
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return nil
	}
	return cfg.NLP.BusinessDomains
}

func mergeDomainIndicators(domain string, confidence float64, configured []string) map[string]float64 {
	indicators := map[string]float64{}
	if domain != "" {
		indicators[domain] = confidence
	}
	for _, cfgDomain := range configured {
		key := strings.ToLower(strings.TrimSpace(cfgDomain))
		if key == "" {
			continue
		}
		if _, ok := indicators[key]; !ok {
			indicators[key] = 1.0
		}
	}
	if len(indicators) == 0 {
		indicators["unknown"] = 0.0
	}
	return indicators
}

func (s *MCPServer) analyzeNamingConventions(tables []TableInfo) map[string]interface{} {
	cases := map[string]int{"snake_case": 0, "camelCase": 0, "PascalCase": 0}
	prefixes := map[string]int{}
	suffixes := map[string]int{}
	timestampCols := []string{}

	// FK pattern detection
	fkSuffix := 0
	fkPrefix := 0
	fkTotal := 0

	for _, t := range tables {
		for _, col := range t.Columns {
			name := col.ColumnName
			if strings.Contains(name, "_") {
				cases["snake_case"]++
			} else if len(name) > 1 && unicode.IsUpper(rune(name[0])) {
				cases["PascalCase"]++
			} else if len(name) > 1 && unicode.IsLower(rune(name[0])) && strings.IndexFunc(name, unicode.IsUpper) > 0 {
				cases["camelCase"]++
			}
			parts := strings.Split(name, "_")
			if len(parts) > 1 {
				prefixes[parts[0]]++
				suffixes[parts[len(parts)-1]]++
			}
			// Audit / timestamp style columns
			if strings.HasSuffix(name, "created_at") || strings.HasSuffix(name, "updated_at") ||
				strings.HasSuffix(name, "timestamp") || strings.HasSuffix(name, "created") || strings.HasSuffix(name, "modified") {
				timestampCols = append(timestampCols, name)
			}
			// Foreign key style detection
			if strings.HasSuffix(name, "_id") && len(name) > 3 {
				fkSuffix++
				fkTotal++
			} else if strings.HasPrefix(name, "id_") && len(name) > 3 {
				fkPrefix++
				fkTotal++
			}
		}
	}

	// Determine dominant case & consistency
	totalCaseCount := cases["snake_case"] + cases["camelCase"] + cases["PascalCase"]
	mainCase := "unknown"
	consistency := 0.0
	if totalCaseCount > 0 {
		// find max
		type kv struct {
			k string
			v int
		}
		var top kv
		for k, v := range cases {
			if v > top.v {
				top = kv{k, v}
			}
		}
		mainCase = top.k
		consistency = float64(top.v) / float64(totalCaseCount)
	}

	// Foreign key naming pattern classification
	fkPattern := "none"
	if fkTotal > 0 {
		switch {
		case fkSuffix > 0 && fkPrefix > 0:
			fkPattern = "mixed"
		case fkSuffix > 0:
			fkPattern = "suffix"
		case fkPrefix > 0:
			fkPattern = "prefix"
		}
	}

	return map[string]interface{}{
		"cases":         cases,
		"prefixes":      prefixes,
		"suffixes":      suffixes,
		"timestampCols": timestampCols,
		"main_case":     mainCase,
		"consistency":   consistency,
		"fk_pattern":    fkPattern,
	}
}

func (s *MCPServer) identifyEntityTypes(tableNames []string) []string {
	types := []string{}
	for _, name := range tableNames {
		lower := strings.ToLower(name)
		if strings.Contains(lower, "log") || strings.Contains(lower, "audit") {
			types = append(types, "log")
		} else if strings.Contains(lower, "lookup") || strings.HasSuffix(lower, "_type") || strings.HasSuffix(lower, "_status") {
			types = append(types, "lookup")
		} else if strings.Contains(lower, "transaction") || strings.Contains(lower, "order") || strings.Contains(lower, "invoice") {
			types = append(types, "transactional")
		} else if strings.Contains(lower, "user") || strings.Contains(lower, "product") || strings.Contains(lower, "customer") {
			types = append(types, "master_data")
		} else {
			types = append(types, "other")
		}
	}
	return types
}

func (s *MCPServer) generateBusinessDescription(domain string, entities []string, confidence float64) string {
	if domain == "unknown" || confidence < 0.2 {
		return "The database schema does not match any well-known business domain. It may be custom or generic."
	}
	entitySummary := map[string]int{}
	for _, e := range entities {
		entitySummary[e]++
	}
	entityDesc := []string{}
	for k, v := range entitySummary {
		entityDesc = append(entityDesc, fmt.Sprintf("%d %s tables", v, k))
	}
	return fmt.Sprintf("This database appears to represent a %s system, with %s. Domain detection confidence: %.2f.", domain, strings.Join(entityDesc, ", "), confidence)
}

// --- Data Pattern Recognition Helpers ---

// analyzeDataPatterns analyzes sample data for each column to detect patterns.
func (s *MCPServer) analyzeDataPatterns(tableName string, sampleData []map[string]interface{}, columns []SchemaColumnInfo) []DataPattern {
	patterns := make([]DataPattern, len(columns))
	for i, col := range columns {
		var values []interface{}
		for _, row := range sampleData {
			if v, ok := row[col.ColumnName]; ok {
				values = append(values, v)
			}
		}
		pattern := s.detectColumnPattern(tableName, col.ColumnName, values)
		if pattern != nil {
			patterns[i] = *pattern
		} else {
			patterns[i] = DataPattern{}
		}
	}
	return patterns
}

// detectColumnPattern analyzes individual column data to detect value distribution, patterns, ranges, and examples.
func (s *MCPServer) detectColumnPattern(tableName string, columnName string, values []interface{}) *DataPattern {
	dist := s.analyzeValueDistribution(values)
	nullCount := 0
	uniqueSet := map[string]struct{}{}
	var min, max interface{}
	var minSet, maxSet bool
	var decimalPlaces int
	var enumValues []string

	// Use detectDataType for semantic inference
	semanticType := s.detectDataType(values)

	// Regex patterns for semantic types
	regexes := map[string]string{
		"email":    `^[\w\.\-]+@[\w\.\-]+\.\w+$`,
		"phone":    `^\+?\d[\d\-\s]{7,}$`,
		"url":      `^https?://[^\s]+$`,
		"id":       `^[a-fA-F0-9\-]{8,}$`,
		"uuid":     `^[a-fA-F0-9\-]{36}$`,
		"date":     `^\d{4}-\d{2}-\d{2}`,
		"currency": `^\$?\d+(\.\d{2})?$`,
	}

	patternType := semanticType
	validationRegex := ""
	for _, v := range values {
		if v == nil {
			nullCount++
			continue
		}
		strVal := fmt.Sprintf("%v", v)
		uniqueSet[strVal] = struct{}{}
		// Numeric min/max
		switch val := v.(type) {
		case int, int32, int64, float32, float64:
			f, err := toFloat64(val)
			if err == nil {
				if !minSet || f < min.(float64) {
					min = f
					minSet = true
				}
				if !maxSet || f > max.(float64) {
					max = f
					maxSet = true
				}
				dec := countDecimalPlaces(strVal)
				if dec > decimalPlaces {
					decimalPlaces = dec
				}
			}
		case string:
			// Pattern detection
			for typ, rx := range regexes {
				if matchRegex(rx, val) {
					patternType = typ
					validationRegex = rx
					break
				}
			}
		}
	}
	// Enum detection: if unique count is small and all are strings
	if len(uniqueSet) > 0 && len(uniqueSet) <= 10 {
		for k := range uniqueSet {
			enumValues = append(enumValues, k)
		}
	}
	// Range for numbers
	var rng *ValueRange
	if minSet && maxSet {
		rng = &ValueRange{Min: min, Max: max}
	}
	// Null percentage
	nullPct := float64(nullCount) / float64(len(values))
	// Uniqueness
	uniqRatio := float64(len(uniqueSet)) / float64(len(values))
	// Distribution type
	distType := dist["distribution"].(string)
	// Enhanced logging for tableName and columnName
	log.JSONLog("debug", "Pattern analysis", map[string]interface{}{"table": tableName, "column": columnName, "patternType": patternType})
	return &DataPattern{
		PatternType:     patternType,
		ValidationRegex: validationRegex,
		Uniqueness:      uniqRatio,
		NullPercentage:  nullPct,
		DecimalPlaces:   decimalPlaces,
		Range:           rng,
		Distribution:    distType,
		Values:          enumValues,
	}
}

// analyzeValueDistribution calculates statistics: unique values, null count, most common values.
func (s *MCPServer) analyzeValueDistribution(values []interface{}) map[string]interface{} {
	stats := map[string]interface{}{}
	counts := map[string]int{}
	nullCount := 0
	for _, v := range values {
		if v == nil {
			nullCount++
			continue
		}
		strVal := fmt.Sprintf("%v", v)
		counts[strVal]++
	}
	stats["unique_count"] = len(counts)
	stats["null_count"] = nullCount
	// Most common values
	type kv struct {
		Key   string
		Value int
	}
	var freq []kv
	for k, v := range counts {
		freq = append(freq, kv{k, v})
	}
	// Sort by frequency
	sort.Slice(freq, func(i, j int) bool { return freq[i].Value > freq[j].Value })
	var common []string
	for i := 0; i < len(freq) && i < 3; i++ {
		common = append(common, freq[i].Key)
	}
	stats["most_common"] = common
	// Distribution type
	if len(counts) == 1 {
		stats["distribution"] = "constant"
	} else if len(counts) < len(values)/2 {
		stats["distribution"] = "categorical"
	} else {
		stats["distribution"] = "variable"
	}
	return stats
}

// detectDataType infers semantic data type beyond SQL type.
func (s *MCPServer) detectDataType(values []interface{}) string {
	regexes := map[string]string{
		"email":    `^[\w\.\-]+@[\w\.\-]+\.\w+$`,
		"phone":    `^\+?\d[\d\-\s]{7,}$`,
		"url":      `^https?://[^\s]+$`,
		"id":       `^[a-fA-F0-9\-]{8,}$`,
		"uuid":     `^[a-fA-F0-9\-]{36}$`,
		"date":     `^\d{4}-\d{2}-\d{2}`,
		"currency": `^\$?\d+(\.\d{2})?$`,
	}
	typeCounts := map[string]int{}
	for _, v := range values {
		if v == nil {
			continue
		}
		strVal := fmt.Sprintf("%v", v)
		for typ, rx := range regexes {
			if matchRegex(rx, strVal) {
				typeCounts[typ]++
			}
		}
	}
	// Pick the most frequent type
	maxType := ""
	maxCount := 0
	for typ, cnt := range typeCounts {
		if cnt > maxCount {
			maxType = typ
			maxCount = cnt
		}
	}
	if maxType != "" && float64(maxCount)/float64(len(values)) > 0.5 {
		return maxType
	}
	return "unknown"
}

// generateDataQualityMetrics calculates completeness, consistency, validity.
func (s *MCPServer) generateDataQualityMetrics(
	sampleData []map[string]interface{},
	columns []SchemaColumnInfo,
) map[string]QualityMetrics {
	metrics := make(map[string]QualityMetrics)
	for _, col := range columns {
		var values []interface{}
		for _, row := range sampleData {
			if v, ok := row[col.ColumnName]; ok {
				values = append(values, v)
			}
		}
		total := len(values)
		if total == 0 {
			metrics[col.ColumnName] = QualityMetrics{
				Issues:       []string{"No sample data available"},
				OverallScore: 0,
			}
			continue
		}
		nonNull := 0
		valid := 0
		uniqueSet := map[string]struct{}{}
		temporalConsistent := 0
		businessRuleCompliant := 0
		consistency := 1.0 // Placeholder: could be calculated from repeated values, etc.
		issues := []string{}

		// Temporal consistency: for date/time columns, check if values are in order
		isTemporal := col.DataType == "date" || col.DataType == "datetime" || col.DataType == "timestamp"
		var lastTime time.Time
		var temporalOrderBroken bool

		for i, v := range values {
			if v != nil {
				nonNull++
				uniqueSet[fmt.Sprintf("%v", v)] = struct{}{}
				// Validity: check pattern if available
				if col.PatternType != "" && col.ValidationRegex != "" {
					if matchRegex(col.ValidationRegex, fmt.Sprintf("%v", v)) {
						valid++
					} else {
						issues = append(issues, fmt.Sprintf("Invalid value for %s: %v", col.ColumnName, v))
					}
				} else {
					valid++
				}
				// Temporal consistency
				if isTemporal {
					strVal := fmt.Sprintf("%v", v)
					t, err := time.Parse("2006-01-02", strVal)
					if err == nil {
						if i > 0 && !lastTime.IsZero() && t.Before(lastTime) {
							temporalOrderBroken = true
						}
						lastTime = t
						temporalConsistent++
					}
				}
				// Business rule compliance: placeholder, always 1 for now
				businessRuleCompliant++
			} else {
				issues = append(issues, fmt.Sprintf("Null value in %s", col.ColumnName))
			}
		}
		completeness := float64(nonNull) / float64(total)
		uniqueness := float64(len(uniqueSet)) / float64(total)
		validity := float64(valid) / float64(total)
		temporalConsistency := 1.0
		if isTemporal && temporalOrderBroken {
			temporalConsistency = 0.0
			issues = append(issues, "Temporal inconsistency detected")
		}
		businessRuleCompliance := float64(businessRuleCompliant) / float64(total)
		overall := (completeness + uniqueness + validity + consistency + temporalConsistency + businessRuleCompliance) / 6.0

		// Cap issues slice length to avoid large payloads and memory growth
		if len(issues) > maxQualityIssuesPerColumn {
			truncated := issues[:maxQualityIssuesPerColumn]
			truncated = append(truncated, fmt.Sprintf("... %d more issues truncated", len(issues)-maxQualityIssuesPerColumn))
			issues = truncated
		}
		metrics[col.ColumnName] = QualityMetrics{
			Completeness:           completeness,
			Uniqueness:             uniqueness,
			Validity:               validity,
			Consistency:            consistency,
			TemporalConsistency:    temporalConsistency,
			BusinessRuleCompliance: businessRuleCompliance,
			OverallScore:           overall,
			Issues:                 issues,
		}
	}
	return metrics
}

// categorizeTables classifies tables for TableCatalog (core, lookup, junction, audit)
func (s *MCPServer) categorizeTables(tableNames []string, schemas map[string]TableInfo) TableCatalog {
	var coreEntities, lookupTables, junctionTables, auditTables []TableEntity
	for _, tbl := range tableNames {
		info := schemas[tbl]
		entity := TableEntity{
			TableName:   tbl,
			ColumnCount: info.ColumnCount,
			PrimaryKey:  info.KeyColumns.PrimaryKey,
		}
		lower := strings.ToLower(tbl)
		switch {
		case strings.Contains(lower, "log") || strings.Contains(lower, "audit"):
			entity.BusinessRole = "audit"
			auditTables = append(auditTables, entity)
		case strings.Contains(lower, "lookup") || strings.HasSuffix(lower, "_type") || strings.HasSuffix(lower, "_status"):
			entity.BusinessRole = "lookup"
			lookupTables = append(lookupTables, entity)
		case strings.Contains(lower, "junction") || strings.Contains(lower, "join"):
			entity.BusinessRole = "junction"
			junctionTables = append(junctionTables, entity)
		default:
			entity.BusinessRole = "core"
			coreEntities = append(coreEntities, entity)
		}
	}
	return TableCatalog{
		CoreEntities:   coreEntities,
		LookupTables:   lookupTables,
		JunctionTables: junctionTables,
		AuditTables:    auditTables,
	}
}

// --- Utility functions ---

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

func countDecimalPlaces(s string) int {
	parts := strings.Split(s, ".")
	if len(parts) == 2 {
		return len(parts[1])
	}
	return 0
}

func matchRegex(pattern, value string) bool {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return false
	}
	return re.MatchString(value)
}
