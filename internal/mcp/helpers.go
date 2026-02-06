package mcp

import (
	"context"
	"database/sql"
	"fmt"

	"database-mcp-provider/internal/config"
	"database-mcp-provider/internal/db"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func errorResult(structErr *StructuredError) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: structErr.ToJSON()}},
	}
}

func (s *MCPServer) findProfile(profileName string) (*config.Config, *config.Profile, error) {
	cfg, err := config.LoadConfig(s.ConfigPath)
	if err != nil {
		return nil, nil, err
	}
	for i := range cfg.Profiles {
		if cfg.Profiles[i].ProfileName == profileName {
			return cfg, &cfg.Profiles[i], nil
		}
	}
	return cfg, nil, fmt.Errorf("profile not found")
}

func (s *MCPServer) openConnection(ctx context.Context, profileName, dbNameOverride string) (*sql.DB, *config.Profile, error) {
	cfg, prof, err := s.findProfile(profileName)
	if err != nil {
		return nil, nil, err
	}
	dbName := prof.DatabaseName
	if dbNameOverride != "" {
		dbName = dbNameOverride
	}
	dsn := db.DSN(prof.DBType, prof.Host, prof.Port, prof.Username, prof.Password, dbName, prof.SSLMode)
	conn, err := db.OpenConnectionWithPool(ctx, prof.DBType, dsn, cfg.MaxPoolSize)
	if err != nil {
		return nil, prof, err
	}
	return conn, prof, nil
}

func (s *MCPServer) switchMySQLDatabase(ctx context.Context, conn *sql.DB, prof *config.Profile, profileName, databaseName string) *mcp.CallToolResult {
	if prof == nil {
		structErr := NewStructuredError(
			ErrorCodeMissingParameter,
			"Missing profile",
			"profile is required for database switching",
		).WithContext("profile_name", profileName)
		return errorResult(structErr)
	}
	if prof.DBType != "mysql" && prof.DBType != "mariadb" {
		return nil
	}
	if databaseName == "" || databaseName == prof.DatabaseName {
		return nil
	}
	if conn == nil {
		structErr := NewStructuredError(
			ErrorCodeConnectionFailed,
			"Database connection missing",
			"connection is required to switch databases",
		).WithContextMap(map[string]interface{}{
			"profile_name": profileName,
			"database":     databaseName,
		})
		return errorResult(structErr)
	}
	if _, err := conn.ExecContext(ctx, "USE "+databaseName); err != nil { //nolint:gosec // NOSONAR - databaseName from internal config, not user SQL input
		structErr := s.errorAnalyzer.AnalyzeError(err, map[string]interface{}{
			"profile_name": profileName,
			"operation":    "switch_database",
			"database":     databaseName,
		})
		return errorResult(structErr)
	}
	return nil
}
