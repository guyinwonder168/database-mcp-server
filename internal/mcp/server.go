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
{"profile_name":"local-mariadb","db_type":"mariadb","host":"localhost","port":3306,"username":"app","password":"secret","database_name":"mysql","readonly":false}`,
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
{"profile_name":"local-mariadb","database_name":"orangehrm_mysql","sql":"SELECT * FROM ohrm_attendance_record WHERE employee_id=34;"}
{"profile_name":"local-mariadb","sql":"DESCRIBE orangehrm_mysql.ohrm_attendance_record"}`,
	}, s.handleExecuteSQL)
	log.JSONLog("debug", "Registered MCP tool", map[string]interface{}{"name": "execute-sql", "description": "Execute SQL"})

	mcp.AddTool(s.server, &mcp.Tool{
		Name: "list-tables",
		Description: `List all tables in the selected database. Use 'database_name' to override the profile's default database.
Example:
{"profile_name":"local-mariadb","database_name":"orangehrm_mysql"}`,
	}, s.handleListTables)
	log.JSONLog("debug", "Registered MCP tool", map[string]interface{}{"name": "list-tables", "description": "List tables"})

	mcp.AddTool(s.server, &mcp.Tool{
		Name: "describe-table",
		Description: `Describe the columns and types of a table. Always use this before constructing queries on unfamiliar tables.
Note: To describe a table in a different database, either update the profile's default database or use a fully qualified table name in SQL (e.g., DESCRIBE db.table) via the execute-sql tool.
Example:
{"profile_name":"local-mariadb","table_name":"ohrm_attendance_record"}
To describe a table in another database:
{"profile_name":"local-mariadb","sql":"DESCRIBE orangehrm_mysql.ohrm_attendance_record"}`,
	}, s.handleDescribeTable)
	log.JSONLog("debug", "Registered MCP tool", map[string]interface{}{"name": "describe-table", "description": "Describe table"})

	mcp.AddTool(s.server, &mcp.Tool{
		Name: "list-databases",
		Description: `List all databases/schemas available to the profile.
Example:
{"profile_name":"local-mariadb"}`,
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

	// --- Serve docs/ folder at /docs endpoint for LLM auto-discovery ---
	go func() {
		// Documentation HTTP handler moved to main.go for embedded docs support
	}()
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
	ProfileName  string `json:"profile_name"`
	SQL          string `json:"sql"`
	DatabaseName string `json:"database_name,omitempty"`
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
	ProfileName string `json:"profile_name"`
	TableName   string `json:"table_name"`
}

type DescribeTableResult struct {
	Columns []struct {
		Name     string `json:"name"`
		Type     string `json:"type"`
		Nullable bool   `json:"nullable"`
		Key      string `json:"key,omitempty"`
	} `json:"columns"`
}

type ListDatabasesParams struct {
	ProfileName string `json:"profile_name"`
}

type ListDatabasesResult struct {
	Databases []string `json:"databases"`
}

// --- MCP Handler Implementations ---

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
		return nil, fmt.Errorf("profile not found")
	}
	// Read-only enforcement
	if prof.Readonly && (len(p.SQL) >= 6 && (p.SQL[:6] == "INSERT" || p.SQL[:6] == "UPDATE" || p.SQL[:6] == "DELETE" || p.SQL[:6] == "CREATE" || p.SQL[:6] == "ALTER" || p.SQL[:6] == "DROP")) {
		return nil, fmt.Errorf("write operations are not allowed on readonly profiles")
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
		return nil, err
	}
	defer conn.Close()
	// For MySQL/MariaDB, optionally switch database if needed
	if (prof.DBType == "mysql" || prof.DBType == "mariadb") && p.DatabaseName != "" && p.DatabaseName != prof.DatabaseName {
		if _, err := conn.Exec("USE " + p.DatabaseName); err != nil {
			return nil, fmt.Errorf("failed to switch to database %s: %w", p.DatabaseName, err)
		}
	}
	// Try query
	rows, err := conn.Query(p.SQL)
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
	res, err := conn.Exec(p.SQL)
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
	dsn := db.DSN(prof.DBType, prof.Host, prof.Port, prof.Username, prof.Password, prof.DatabaseName)
	conn, err := db.OpenConnectionWithPool(prof.DBType, dsn, cfg.MaxPoolSize)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	var query string
	switch prof.DBType {
	case "mysql", "mariadb":
		query = fmt.Sprintf("DESCRIBE `%s`", p.TableName)
	case "postgres":
		query = fmt.Sprintf("SELECT column_name, data_type, is_nullable FROM information_schema.columns WHERE table_name='%s'", p.TableName)
	case "sqlite":
		query = fmt.Sprintf("PRAGMA table_info('%s')", p.TableName)
	default:
		return nil, fmt.Errorf("unsupported db_type")
	}
	rows, err := conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var columns []struct {
		Name     string `json:"name"`
		Type     string `json:"type"`
		Nullable bool   `json:"nullable"`
		Key      string `json:"key,omitempty"`
	}
	switch prof.DBType {
	case "mysql", "mariadb":
		for rows.Next() {
			var field, typ, null, key, extra1, extra2 string
			if err := rows.Scan(&field, &typ, &null, &key, &extra1, &extra2); err != nil {
				return nil, err
			}
			columns = append(columns, struct {
				Name     string `json:"name"`
				Type     string `json:"type"`
				Nullable bool   `json:"nullable"`
				Key      string `json:"key,omitempty"`
			}{
				Name:     field,
				Type:     typ,
				Nullable: null == "YES",
				Key:      key,
			})
		}
	case "postgres":
		for rows.Next() {
			var name, typ, nullable string
			if err := rows.Scan(&name, &typ, &nullable); err != nil {
				return nil, err
			}
			columns = append(columns, struct {
				Name     string `json:"name"`
				Type     string `json:"type"`
				Nullable bool   `json:"nullable"`
				Key      string `json:"key,omitempty"`
			}{
				Name:     name,
				Type:     typ,
				Nullable: nullable == "YES",
			})
		}
	case "sqlite":
		for rows.Next() {
			var cid int
			var name, typ string
			var notnull, pk int
			var dflt interface{}
			if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
				return nil, err
			}
			columns = append(columns, struct {
				Name     string `json:"name"`
				Type     string `json:"type"`
				Nullable bool   `json:"nullable"`
				Key      string `json:"key,omitempty"`
			}{
				Name:     name,
				Type:     typ,
				Nullable: notnull == 0,
				Key:      "",
			})
		}
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
