// server.go
// Author: guyinwonder
// Version: v1.0.0
// Project created using OpenAI GPT-4.1 via VSCode Kilocode AI code assistant extension.
// MCP server implementation for the database-mcp-provider project.
// Provides MCP actions for profile management, SQL execution, table/DB listing, and uses structured JSON logging.

package mcp

import (
	"context"
	"database-mcp-provider/internal/config"
	"database-mcp-provider/internal/db"
	"database-mcp-provider/internal/log"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

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
	mcp.AddTool(s.server, &mcp.Tool{
		Name: "configure-profile",
		Description: `Create or update a database connection profile. Required for all database actions.
Example:
{"profile_name":"some-profile-name","db_type":"mariadb","host":"localhost","port":3306,"username":"app","password":"secret","database_name":"mysql","readonly":false}`,
	}, s.handleConfigureProfile)
	log.JSONLog("debug", "Registered MCP tool", map[string]interface{}{"name": "configure-profile", "description": "Configure a DB profile"})

	mcp.AddTool(s.server, &mcp.Tool{
		Name: "list-profiles",
		Description: `List all configured database profiles.
Example:
{}`,
	}, s.handleListProfiles)
	log.JSONLog("debug", "Registered MCP tool", map[string]interface{}{"name": "list-profiles", "description": "List DB profiles"})

	mcp.AddTool(s.server, &mcp.Tool{
		Name: "execute-sql",
		Description: `Execute an arbitrary SQL query or statement. Use the 'database_name' parameter to select a database if needed.
Note: For cross-database queries or describing tables in another database, use fully qualified table names (e.g., db.table).
Example:
{"profile_name":"some-profile-name","database_name":"some-database-name","sql":"SELECT * FROM some-table-name WHERE some-field-name=34;"}
{"profile_name":"some-profile-name","sql":"DESCRIBE some-database-name.some-table-name"}`,
	}, s.handleExecuteSQL)
	log.JSONLog("debug", "Registered MCP tool", map[string]interface{}{"name": "execute-sql", "description": "Execute SQL"})

	mcp.AddTool(s.server, &mcp.Tool{
		Name: "list-tables",
		Description: `List all tables in the selected database. Use 'database_name' to override the profile's default database.
Example:
{"profile_name":"some-profile-name","database_name":"some-database-name"}`,
	}, s.handleListTables)
	log.JSONLog("debug", "Registered MCP tool", map[string]interface{}{"name": "list-tables", "description": "List tables"})

	mcp.AddTool(s.server, &mcp.Tool{
		Name: "describe-table",
		Description: `Describe the comprehensive schema of a table including columns, types, constraints, comments, and metadata. Returns detailed information to enable AI/agents to understand table structure and build intelligent queries.
Returns: column names, data types, nullable status, key constraints, default values, column comments, character sets, collation, auto-increment status, max length, precision, and scale.
Example:
{"profile_name":"some-profile-name","database_name":"some-database-name","table_name":"some-table-name"}`,
	}, s.handleDescribeTable)
	log.JSONLog("debug", "Registered MCP tool", map[string]interface{}{"name": "describe-table", "description": "Describe table"})

	mcp.AddTool(s.server, &mcp.Tool{
		Name: "list-databases",
		Description: `List all databases/schemas available to the profile.
Example:
{"profile_name":"some-profile-name"}`,
	}, s.handleListDatabases)
	log.JSONLog("debug", "Registered MCP tool", map[string]interface{}{"name": "list-databases", "description": "List databases/schemas"})

	// Add version/author info tool
	mcp.AddTool(s.server, &mcp.Tool{
		Name: "mcp-info",
		Description: `Show MCP provider version and author.
Example:
{}`,
	}, s.handleMCPInfo)
	log.JSONLog("debug", "Registered MCP tool", map[string]interface{}{"name": "mcp-info", "description": "Show MCP provider version and author"})

	// Register Smart Query Builder Tool
	mcp.AddTool(s.server, &mcp.Tool{
		Name: "smart-query-builder",
		Description: `Generate optimized SQL from high-level intent and schema analysis.
Input: profile_name, intent (natural language), optional database_name/table_name(s).
Returns: generated SQL, explanation, and any errors.
Example:
{"profile_name":"some-profile-name","intent":"attendance dashboard"}`,
	}, s.handleSmartQueryBuilder)
	log.JSONLog("debug", "Registered MCP tool", map[string]interface{}{"name": "smart-query-builder", "description": "Generate SQL from intent"})
	// --- Serve docs/ folder at /docs endpoint for LLM auto-discovery ---
	go func() {
		// Documentation HTTP handler moved to main.go for embedded docs support
	}()
	log.JSONLog("info", "MCP server (SDK) running on stdio", nil)
	// Register the discover-joins MCP tool
	mcp.AddTool(s.server, &mcp.Tool{
		Name: "discover-joins",
		Description: `Discover joinable relationships (foreign keys) between tables and suggest JOIN SQL.
Input: profile_name (required), tables (optional).
Returns: list of join suggestions and summary.
Example:
{"profile_name":"analytics_db","tables":["orders","customers"]}`,
	}, s.handleDiscoverJoins)
	log.JSONLog("debug", "Registered MCP tool", map[string]interface{}{"name": "discover-joins", "description": "Discover joinable relationships"})

	// Register the sample-data MCP tool
	mcp.AddTool(s.server, &mcp.Tool{
		Name: "sample-data",
		Description: `Fetch sample rows from a table to help AI/agents infer data types, formats, and value ranges.
Input: profile_name (required), table_name (required), database_name (optional), sample_size (optional, default: 3).
Returns: sample rows with column names and values.
Example:
{"profile_name":"analytics_db","table_name":"users","sample_size":5}`,
	}, s.handleSampleData)
	log.JSONLog("debug", "Registered MCP tool", map[string]interface{}{"name": "sample-data", "description": "Fetch sample rows from table"})

	return s.server.Run(context.Background(), mcp.NewStdioTransport())
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
	session *mcp.ServerSession,
	params *mcp.CallToolParamsFor[DiscoverJoinsParams],
) (*mcp.CallToolResultFor[any], error) {
	p := params.Arguments

	// 1. Load config and profile
	cfg, err := config.LoadConfig(s.ConfigPath)
	if err != nil {
		return nil, err
	}
	var prof *config.Profile
	for i := range cfg.Profiles {
		if cfg.Profiles[i].ProfileName == p.ProfileName {
			prof = &cfg.Profiles[i]
			break
		}
	}
	if prof == nil {
		return nil, fmt.Errorf("profile not found")
	}

	// 2. Connect to DB
	dsn := db.DSN(prof.DBType, prof.Host, prof.Port, prof.Username, prof.Password, prof.DatabaseName)
	conn, err := db.OpenConnectionWithPool(prof.DBType, dsn, cfg.MaxPoolSize)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

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
		return nil, fmt.Errorf("unsupported db_type for join discovery")
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
			rows, err := conn.Query("SELECT name FROM sqlite_master WHERE type='table'")
			if err != nil {
				return nil, err
			}
			defer rows.Close()
			for rows.Next() {
				var name string
				if err := rows.Scan(&name); err == nil {
					tables = append(tables, name)
				}
			}
		}
		for _, tbl := range tables {
			rows, err := conn.Query(fmt.Sprintf("PRAGMA foreign_key_list('%s')", tbl))
			if err != nil {
				continue
			}
			defer rows.Close()
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
				}
			}
		}
	} else {
		var rows *sql.Rows
		if prof.DBType == "mysql" || prof.DBType == "mariadb" {
			rows, err = conn.Query(fkQuery, prof.DatabaseName)
		} else {
			rows, err = conn.Query(fkQuery)
		}
		if err != nil {
			return nil, err
		}
		defer rows.Close()
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
	return &mcp.CallToolResultFor[any]{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(b),
			},
		},
	}, nil
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

// --- MCP Handler Parameter Structs ---

type ConfigureProfileParams struct {
	ProfileName  string `json:"profile_name"`
	DBType       string `json:"db_type"`
	Host         string `json:"host,omitempty"`
	Port         int    `json:"port,omitempty"`
	Username     string `json:"username,omitempty"`
	Password     string `json:"password,omitempty"`
	DatabaseName string `json:"database_name"`
	Readonly     bool   `json:"readonly"`
}

type ListProfilesResult struct {
	Profiles []struct {
		ProfileName string `json:"profile_name"`
		DBType      string `json:"db_type"`
	} `json:"profiles"`
}

type ExecuteSQLParams struct {
	ProfileName  string        `json:"profile_name"`
	SQL          string        `json:"sql"`
	DatabaseName string        `json:"database_name,omitempty"`
	Params       []interface{} `json:"params,omitempty"`
}

type ExecuteSQLResult struct {
	Columns  []string        `json:"columns,omitempty"`
	Rows     [][]interface{} `json:"rows,omitempty"`
	Affected int             `json:"affected,omitempty"`
}

type ListTablesParams struct {
	ProfileName  string `json:"profile_name"`
	DatabaseName string `json:"database_name,omitempty"`
}

type ListTablesResult struct {
	Tables []string `json:"tables"`
}

type DescribeTableParams struct {
	ProfileName  string `json:"profile_name"`
	DatabaseName string `json:"database_name"`
	TableName    string `json:"table_name"`
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

type ListDatabasesResult struct {
	Databases []string `json:"databases"`
}

// --- Sample Data Types ---
type SampleDataParams struct {
	ProfileName  string `json:"profile_name"`
	TableName    string `json:"table_name"`
	DatabaseName string `json:"database_name,omitempty"`
	SampleSize   int    `json:"sample_size,omitempty"`
}

type SampleDataResult struct {
	TableName  string          `json:"table_name"`
	SampleSize int             `json:"sample_size"`
	Columns    []string        `json:"columns"`
	SampleRows [][]interface{} `json:"sample_rows"`
	Summary    string          `json:"summary"`
}

// --- MCP Handler Implementations ---

// handleSmartQueryBuilder generates SQL from high-level intent and schema.
func (s *MCPServer) handleSmartQueryBuilder(ctx context.Context, session *mcp.ServerSession, params *mcp.CallToolParamsFor[SmartQueryBuilderParams]) (*mcp.CallToolResultFor[any], error) {
	p := params.Arguments

	// 1. Load config and profile
	cfg, err := config.LoadConfig(s.ConfigPath)
	if err != nil {
		return nil, err
	}
	var prof *config.Profile
	for i := range cfg.Profiles {
		if cfg.Profiles[i].ProfileName == p.ProfileName {
			prof = &cfg.Profiles[i]
			break
		}
	}
	if prof == nil {
		return nil, fmt.Errorf("profile not found")
	}

	// 2. Fetch all table names
	dsn := db.DSN(prof.DBType, prof.Host, prof.Port, prof.Username, prof.Password, prof.DatabaseName)
	conn, err := db.OpenConnectionWithPool(prof.DBType, dsn, cfg.MaxPoolSize)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	var tables []string
	{
		var query string
		switch prof.DBType {
		case "mysql", "mariadb":
			query = "SHOW FULL TABLES"
		case "postgres":
			query = "SELECT table_name FROM information_schema.tables WHERE table_schema='public'"
		case "sqlite":
			query = "SELECT name FROM sqlite_master WHERE type='table'"
		default:
			return nil, fmt.Errorf("unsupported db_type")
		}
		rows, err := conn.Query(query)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			if prof.DBType == "mysql" || prof.DBType == "mariadb" {
				var name, tableType string
				if err := rows.Scan(&name, &tableType); err == nil {
					tables = append(tables, name)
				}
			} else {
				var name string
				if err := rows.Scan(&name); err == nil {
					tables = append(tables, name)
				}
			}
		}
	}

	// 3. Parse intent and match to table names
	table := ""
	if len(p.TableNames) > 0 {
		table = p.TableNames[0]
	} else if len(tables) > 0 {
		// Extract keywords from intent (simple split, lowercase, remove common stopwords)
		intent := strings.ToLower(p.Intent)
		words := strings.FieldsFunc(intent, func(r rune) bool {
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
		// Score tables by keyword substring match
		bestScore := 0
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
				table = t
			}
		}
		// Fallback to first table if no match
		if table == "" {
			table = tables[0]
		}
	}

	if table == "" {
		errMsg := "No table found matching the intent for query generation."
		suggestion := ""
		if len(tables) > 0 {
			suggestion = fmt.Sprintf("Available tables: %s.", strings.Join(tables, ", "))
		} else {
			suggestion = "No tables found in the database."
		}
		return &mcp.CallToolResultFor[any]{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf(`{"status":"error","error_code":"NO_TABLE_MATCH","message":"%s %s"}`, errMsg, suggestion),
				},
			},
		}, nil
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
			return nil, fmt.Errorf("unsupported db_type")
		}
		rows, err := conn.Query(colQuery)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var colName string
			switch prof.DBType {
			case "mysql", "mariadb":
				var field, typ, null, key, def, extra string
				if err := rows.Scan(&field, &typ, &null, &key, &def, &extra); err == nil {
					colName = field
				}
			case "postgres":
				if err := rows.Scan(&colName); err != nil {
					continue
				}
			case "sqlite":
				var cid int
				var typ, notnull, dflt_value, pk interface{}
				if err := rows.Scan(&cid, &colName, &typ, &notnull, &dflt_value, &pk); err != nil {
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

	result := SmartQueryBuilderResult{
		SQL:         sql,
		Explanation: explanation,
	}
	b, _ := json.Marshal(result)
	return &mcp.CallToolResultFor[any]{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(b),
			},
		},
	}, nil
}

func (s *MCPServer) handleConfigureProfile(ctx context.Context, session *mcp.ServerSession, params *mcp.CallToolParamsFor[ConfigureProfileParams]) (*mcp.CallToolResultFor[any], error) {
	// Load config, or create new if missing
	cfg, err := config.LoadConfig(s.ConfigPath)
	log.JSONLog("debug", "Loaded config for configure-profile", map[string]interface{}{"configPath": s.ConfigPath, "error": err})
	if err != nil {
		log.JSONLog("warn", "Failed to load config, creating new config", map[string]interface{}{"error": err.Error()})
		cfg = &config.Config{}
	}
	p := params.Arguments
	// Default database_name to "mysql" for MySQL/MariaDB if empty
	if (p.DBType == "mysql" || p.DBType == "mariadb") && p.DatabaseName == "" {
		p.DatabaseName = "mysql"
	}
	// Validate required fields
	if p.ProfileName == "" || p.DBType == "" || p.DatabaseName == "" {
		return nil, fmt.Errorf("profile_name, db_type, and database_name are required")
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
		})
	}
	// Save config
	if err := config.SaveConfig(s.ConfigPath, cfg); err != nil {
		return nil, err
	}
	return &mcp.CallToolResultFor[any]{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: "Profile configured successfully.",
			},
		},
	}, nil
}

func (s *MCPServer) handleListProfiles(ctx context.Context, session *mcp.ServerSession, params *mcp.CallToolParamsFor[any]) (*mcp.CallToolResultFor[any], error) {
	cfg, err := config.LoadConfig(s.ConfigPath)
	if err != nil || len(cfg.Profiles) == 0 {
		return &mcp.CallToolResultFor[any]{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: "No database profiles configured. Please use the 'configure-profile' tool to add a profile.",
				},
			},
		}, nil
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
		return nil, err
	}
	return &mcp.CallToolResultFor[any]{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(b),
			},
		},
	}, nil
}

func (s *MCPServer) handleExecuteSQL(ctx context.Context, session *mcp.ServerSession, params *mcp.CallToolParamsFor[ExecuteSQLParams]) (*mcp.CallToolResultFor[any], error) {
	cfg, err := config.LoadConfig(s.ConfigPath)
	if err != nil || len(cfg.Profiles) == 0 {
		log.JSONLog("error", "No database profiles configured", map[string]interface{}{"error": err})
		return &mcp.CallToolResultFor[any]{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: "No database profiles configured. Please use the 'configure-profile' tool to add a profile.",
				},
			},
		}, nil
	}
	p := params.Arguments
	var prof *config.Profile
	for i := range cfg.Profiles {
		if cfg.Profiles[i].ProfileName == p.ProfileName {
			prof = &cfg.Profiles[i]
			break
		}
	}
	if prof == nil {
		log.JSONLog("error", "Profile not found", map[string]interface{}{"profile_name": p.ProfileName})
		return nil, fmt.Errorf("profile not found")
	}
	// Enhanced read-only enforcement
	if prof.Readonly {
		sqlNorm := strings.TrimSpace(p.SQL)
		sqlNorm = strings.ToLower(sqlNorm)
		// Remove leading SQL comments
		for strings.HasPrefix(sqlNorm, "--") || strings.HasPrefix(sqlNorm, "/*") {
			if strings.HasPrefix(sqlNorm, "--") {
				if idx := strings.Index(sqlNorm, "\n"); idx != -1 {
					sqlNorm = strings.TrimSpace(sqlNorm[idx+1:])
				} else {
					sqlNorm = ""
				}
			} else if strings.HasPrefix(sqlNorm, "/*") {
				if idx := strings.Index(sqlNorm, "*/"); idx != -1 {
					sqlNorm = strings.TrimSpace(sqlNorm[idx+2:])
				} else {
					sqlNorm = ""
				}
			}
		}
		// Allow only safe statements
		allowed := false
		for _, prefix := range []string{"select", "show", "describe", "explain", "pragma"} {
			if strings.HasPrefix(sqlNorm, prefix) {
				allowed = true
				break
			}
		}
		if !allowed {
			log.JSONLog("warn", "Blocked unsafe SQL on readonly profile", map[string]interface{}{"profile_name": p.ProfileName, "sql": p.SQL})
			return nil, fmt.Errorf("write or unsafe operations are not allowed on readonly profiles")
		}
	}
	// Use requested database if provided, else profile default
	dbName := prof.DatabaseName
	if p.DatabaseName != "" {
		dbName = p.DatabaseName
	}
	// Build DSN and connect
	dsn := db.DSN(prof.DBType, prof.Host, prof.Port, prof.Username, prof.Password, dbName)
	conn, err := db.OpenConnectionWithPool(prof.DBType, dsn, cfg.MaxPoolSize)
	if err != nil {
		log.JSONLog("error", "Failed to prepare statement", map[string]interface{}{"sql": p.SQL, "error": err})
		return nil, err
	}
	defer conn.Close()
	// For MySQL/MariaDB, optionally switch database if needed
	if (prof.DBType == "mysql" || prof.DBType == "mariadb") && p.DatabaseName != "" && p.DatabaseName != prof.DatabaseName {
		if _, err := conn.Exec("USE " + p.DatabaseName); err != nil {
			return nil, fmt.Errorf("failed to switch to database %s: %w", p.DatabaseName, err)
		}
	}
	// Try query with parameters
	var rows *sql.Rows
	if len(p.Params) > 0 {
		stmt, err := conn.Prepare(p.SQL)
		if err != nil {
			return nil, err
		}
		defer stmt.Close()
		rows, err = stmt.Query(p.Params...)
	} else {
		rows, err = conn.Query(p.SQL)
	}
	if err != nil {
		log.JSONLog("error", "Query failed", map[string]interface{}{"sql": p.SQL, "params": p.Params, "error": err})
	}
	if err == nil {
		defer rows.Close()
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
			typeRows, err := conn.Query("DESCRIBE " + tableName)
			if err == nil {
				for typeRows.Next() {
					var field, typ, null, key, def, extra string
					if err := typeRows.Scan(&field, &typ, &null, &key, &def, &extra); err == nil {
						typeMap[field] = typ
					}
				}
				typeRows.Close()
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
				return nil, err
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
		return &mcp.CallToolResultFor[any]{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: string(mustJSONMarshal(ExecuteSQLResult{
						Columns: cols,
						Rows:    results,
					})),
				},
			},
		}, nil
	}
	// If not a query, try Exec
	var res sql.Result
	if len(p.Params) > 0 {
		stmt, err := conn.Prepare(p.SQL)
		if err != nil {
			return nil, err
		}
		defer stmt.Close()
		res, err = stmt.Exec(p.Params...)
	} else {
		res, err = conn.Exec(p.SQL)
	}
	if err != nil {
		return nil, err
	}
	affected, _ := res.RowsAffected()
	return &mcp.CallToolResultFor[any]{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(mustJSONMarshal(ExecuteSQLResult{
					Affected: int(affected),
				})),
			},
		},
	}, nil
}

// mustJSONMarshal is a helper for panic-free JSON marshaling.
func mustJSONMarshal(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}

func (s *MCPServer) handleListTables(ctx context.Context, session *mcp.ServerSession, params *mcp.CallToolParamsFor[ListTablesParams]) (*mcp.CallToolResultFor[any], error) {
	cfg, err := config.LoadConfig(s.ConfigPath)
	if err != nil {
		return nil, err
	}
	p := params.Arguments
	var prof *config.Profile
	for i := range cfg.Profiles {
		if cfg.Profiles[i].ProfileName == p.ProfileName {
			prof = &cfg.Profiles[i]
			break
		}
	}
	if prof == nil {
		return nil, fmt.Errorf("profile not found")
	}
	// Use requested database if provided, else profile default
	dbName := prof.DatabaseName
	if p.DatabaseName != "" {
		dbName = p.DatabaseName
	}
	dsn := db.DSN(prof.DBType, prof.Host, prof.Port, prof.Username, prof.Password, dbName)
	conn, err := db.OpenConnectionWithPool(prof.DBType, dsn, cfg.MaxPoolSize)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	// For MySQL/MariaDB, optionally switch database if needed
	if (prof.DBType == "mysql" || prof.DBType == "mariadb") && p.DatabaseName != "" && p.DatabaseName != prof.DatabaseName {
		if _, err := conn.Exec("USE " + p.DatabaseName); err != nil {
			return nil, fmt.Errorf("failed to switch to database %s: %w", p.DatabaseName, err)
		}
	}
	var query string
	switch prof.DBType {
	case "mysql", "mariadb":
		query = "SHOW FULL TABLES"
	case "postgres":
		query = "SELECT table_name FROM information_schema.tables WHERE table_schema='public'"
	case "sqlite":
		query = "SELECT name FROM sqlite_master WHERE type='table'"
	default:
		return nil, fmt.Errorf("unsupported db_type")
	}
	rows, err := conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tables []string
	for rows.Next() {
		if prof.DBType == "mysql" || prof.DBType == "mariadb" {
			var name, tableType string
			if err := rows.Scan(&name, &tableType); err != nil {
				return nil, err
			}
			tables = append(tables, name)
		} else {
			var name string
			if err := rows.Scan(&name); err != nil {
				return nil, err
			}
			tables = append(tables, name)
		}
	}
	result := ListTablesResult{Tables: tables}
	return &mcp.CallToolResultFor[any]{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(mustJSONMarshal(result)),
			},
		},
	}, nil
}

func (s *MCPServer) handleDescribeTable(ctx context.Context, session *mcp.ServerSession, params *mcp.CallToolParamsFor[DescribeTableParams]) (*mcp.CallToolResultFor[any], error) {
	cfg, err := config.LoadConfig(s.ConfigPath)
	if err != nil {
		return nil, err
	}
	p := params.Arguments
	var prof *config.Profile
	for i := range cfg.Profiles {
		if cfg.Profiles[i].ProfileName == p.ProfileName {
			prof = &cfg.Profiles[i]
			break
		}
	}
	if prof == nil {
		return nil, fmt.Errorf("profile not found")
	}
	// Always require both database name and table name from user
	if p.DatabaseName == "" || p.TableName == "" {
		return &mcp.CallToolResultFor[any]{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: "Both database_name and table_name must be provided. Please specify both.",
				},
			},
		}, nil
	}
	dsn := db.DSN(prof.DBType, prof.Host, prof.Port, prof.Username, prof.Password, p.DatabaseName)
	conn, err := db.OpenConnectionWithPool(prof.DBType, dsn, cfg.MaxPoolSize)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

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
			NUMERIC_PRECISION as precision,
			NUMERIC_SCALE as scale
		FROM INFORMATION_SCHEMA.COLUMNS
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
		ORDER BY ORDINAL_POSITION`

		rows, err := conn.Query(query, p.DatabaseName, p.TableName)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		for rows.Next() {
			var name, typ, nullable, keyType, extra string
			var defaultVal, comment, characterSet, collation sql.NullString
			var maxLength, precision, scale sql.NullInt64

			if err := rows.Scan(&name, &typ, &nullable, &keyType, &defaultVal, &comment, &extra, &characterSet, &collation, &maxLength, &precision, &scale); err != nil {
				return nil, err
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
		rows, err := conn.Query(query, schema, p.TableName)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		for rows.Next() {
			var name, typ, nullable, keyType string
			var defaultVal, comment sql.NullString
			var maxLength, precision, scale sql.NullInt64

			if err := rows.Scan(&name, &typ, &nullable, &defaultVal, &comment, &maxLength, &precision, &scale, &keyType); err != nil {
				return nil, err
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

		rows, err := conn.Query(query)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

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
		return nil, fmt.Errorf("unsupported db_type")
	}

	result := DescribeTableResult{Columns: columns}
	return &mcp.CallToolResultFor[any]{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(mustJSONMarshal(result)),
			},
		},
	}, nil
}

func (s *MCPServer) handleListDatabases(ctx context.Context, session *mcp.ServerSession, params *mcp.CallToolParamsFor[ListDatabasesParams]) (*mcp.CallToolResultFor[any], error) {
	cfg, err := config.LoadConfig(s.ConfigPath)
	if err != nil {
		return nil, err
	}
	p := params.Arguments
	var prof *config.Profile
	for i := range cfg.Profiles {
		if cfg.Profiles[i].ProfileName == p.ProfileName {
			prof = &cfg.Profiles[i]
			break
		}
	}
	if prof == nil {
		return nil, fmt.Errorf("profile not found")
	}
	dsn := db.DSN(prof.DBType, prof.Host, prof.Port, prof.Username, prof.Password, prof.DatabaseName)
	conn, err := db.OpenConnectionWithPool(prof.DBType, dsn, cfg.MaxPoolSize)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	var query string
	switch prof.DBType {
	case "mysql", "mariadb":
		query = "SHOW DATABASES"
	case "postgres":
		query = "SELECT datname FROM pg_database WHERE datistemplate = false"
	case "sqlite":
		return &mcp.CallToolResultFor[any]{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: string(mustJSONMarshal(ListDatabasesResult{
						Databases: []string{prof.DatabaseName},
					})),
				},
			},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported db_type")
	}
	rows, err := conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var dbs []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		dbs = append(dbs, name)
	}
	result := ListDatabasesResult{Databases: dbs}
	return &mcp.CallToolResultFor[any]{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(mustJSONMarshal(result)),
			},
		},
	}, nil
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
	session *mcp.ServerSession,
	params *mcp.CallToolParamsFor[SampleDataParams],
) (*mcp.CallToolResultFor[any], error) {
	p := params.Arguments

	// Input validation
	if p.ProfileName == "" {
		return nil, fmt.Errorf("profile_name is required")
	}
	if p.TableName == "" {
		return nil, fmt.Errorf("table_name is required")
	}

	// Set default sample size if not provided
	sampleSize := p.SampleSize
	if sampleSize <= 0 {
		sampleSize = 3 // Default to 3 rows
	}
	if sampleSize > 100 {
		sampleSize = 100 // Cap at 100 rows for performance
	}

	// 1. Load config and profile
	cfg, err := config.LoadConfig(s.ConfigPath)
	if err != nil {
		log.JSONLog("error", "Failed to load config for sample data", map[string]interface{}{"error": err.Error()})
		return nil, err
	}
	var prof *config.Profile
	for i := range cfg.Profiles {
		if cfg.Profiles[i].ProfileName == p.ProfileName {
			prof = &cfg.Profiles[i]
			break
		}
	}
	if prof == nil {
		log.JSONLog("error", "Profile not found for sample data", map[string]interface{}{"profile_name": p.ProfileName})
		return nil, fmt.Errorf("profile not found")
	}

	// 2. Determine database name
	dbName := prof.DatabaseName
	if p.DatabaseName != "" {
		dbName = p.DatabaseName
	}

	// 3. Connect to database
	dsn := db.DSN(prof.DBType, prof.Host, prof.Port, prof.Username, prof.Password, dbName)
	conn, err := db.OpenConnectionWithPool(prof.DBType, dsn, cfg.MaxPoolSize)
	if err != nil {
		log.JSONLog("error", "Failed to connect for sample data", map[string]interface{}{"error": err.Error(), "profile": p.ProfileName})
		return nil, err
	}
	defer conn.Close()

	// 4. Switch database if needed (MySQL/MariaDB)
	if (prof.DBType == "mysql" || prof.DBType == "mariadb") && p.DatabaseName != "" && p.DatabaseName != prof.DatabaseName {
		if _, err := conn.Exec("USE " + p.DatabaseName); err != nil {
			log.JSONLog("error", "Failed to switch database for sample data", map[string]interface{}{"database": p.DatabaseName, "error": err.Error()})
			return nil, fmt.Errorf("failed to switch to database %s: %w", p.DatabaseName, err)
		}
	}

	// 5. Build sample query based on database type
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
		return nil, fmt.Errorf("unsupported db_type for sample data: %s", prof.DBType)
	}

	// 6. Execute sample query
	log.JSONLog("debug", "Executing sample data query", map[string]interface{}{"query": sampleQuery, "table": p.TableName})
	rows, err := conn.Query(sampleQuery)
	if err != nil {
		log.JSONLog("error", "Sample data query failed", map[string]interface{}{"query": sampleQuery, "error": err.Error()})
		return nil, fmt.Errorf("failed to fetch sample data from table %s: %w", p.TableName, err)
	}
	defer rows.Close()

	// 7. Extract column names
	columns, err := rows.Columns()
	if err != nil {
		log.JSONLog("error", "Failed to get column names for sample data", map[string]interface{}{"table": p.TableName, "error": err.Error()})
		return nil, err
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
			return nil, err
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
		return nil, err
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
	return &mcp.CallToolResultFor[any]{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(b),
			},
		},
	}, nil
}
