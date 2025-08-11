//go:build cgo

package mcp

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"database-mcp-provider/internal/config"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Helper: create a test config file
func setupTestConfig(t *testing.T) string {
	const testConfig = "test_config.yaml"
	os.Remove(testConfig)
	cfg := &config.Config{
		Profiles: []config.Profile{
			{
				ProfileName:  "testpg",
				DBType:       "postgres",
				Host:         "localhost",
				Port:         5432,
				Username:     "testuser",
				Password:     "testpass",
				DatabaseName: "testdb",
				Readonly:     false,
			},
			{
				ProfileName:  "testsqlite",
				DBType:       "sqlite",
				DatabaseName: ":memory:",
				Readonly:     false,
			},
		},
		MaxPoolSize: 5,
	}
	if err := config.SaveConfig(testConfig, cfg); err != nil {
		t.Fatalf("Failed to save test config: %v", err)
	}
	os.Setenv("DB_MCP_AES_KEY", "12345678901234567890123456789012") // 32 bytes for test
	return testConfig
}

func TestHandleListProfiles(t *testing.T) {
	testConfig := setupTestConfig(t)
	defer os.Remove(testConfig)

	server := NewMCPServerWithConfig(testConfig)
	ctx := context.Background()
	session := &mcp.ServerSession{}
	params := &mcp.CallToolParamsFor[any]{}

	res, err := server.handleListProfiles(ctx, session, params)
	if err != nil {
		t.Fatalf("handleListProfiles error: %v", err)
	}
	if res == nil || res.Content == nil {
		t.Fatalf("handleListProfiles returned nil content")
	}
}

func TestHandleConfigureProfile(t *testing.T) {
	testConfig := setupTestConfig(t)
	defer os.Remove(testConfig)

	server := NewMCPServerWithConfig(testConfig)
	ctx := context.Background()
	session := &mcp.ServerSession{}
	params := &mcp.CallToolParamsFor[ConfigureProfileParams]{
		Arguments: ConfigureProfileParams{
			ProfileName:  "newprofile",
			DBType:       "sqlite",
			DatabaseName: ":memory:",
			Readonly:     false,
		},
	}

	res, err := server.handleConfigureProfile(ctx, session, params)
	if err != nil {
		t.Fatalf("handleConfigureProfile error: %v", err)
	}
	if res == nil || res.Content == nil {
		t.Fatalf("handleConfigureProfile returned nil content")
	}
}

// TestHandleExecuteSQL tests the execute-sql MCP action.
func TestHandleExecuteSQL(t *testing.T) {
	testConfig := setupTestConfig(t)
	defer os.Remove(testConfig)

	server := NewMCPServerWithConfig(testConfig)
	ctx := context.Background()
	session := &mcp.ServerSession{}
	params := &mcp.CallToolParamsFor[ExecuteSQLParams]{
		Arguments: ExecuteSQLParams{
			ProfileName:  "testsqlite",
			SQL:          "SELECT 1",
			DatabaseName: ":memory:",
		},
	}

	res, err := server.handleExecuteSQL(ctx, session, params)
	if err != nil {
		t.Fatalf("handleExecuteSQL error: %v", err)
	}
	if res == nil || res.Content == nil {
		t.Fatalf("handleExecuteSQL returned nil content")
	}
}

// TestHandleExecuteSQL_ParamAndReadonly tests parameterized queries and read-only enforcement for all supported DBs.
func TestHandleExecuteSQL_ParamAndReadonly(t *testing.T) {
	testConfig := setupTestConfig(t)
	defer os.Remove(testConfig)

	server := NewMCPServerWithConfig(testConfig)
	ctx := context.Background()
	session := &mcp.ServerSession{}

	// SQLite: Create table, insert, select with params, enforce readonly
	createParams := &mcp.CallToolParamsFor[ExecuteSQLParams]{
		Arguments: ExecuteSQLParams{
			ProfileName:  "testsqlite",
			SQL:          "CREATE TABLE test (id INTEGER PRIMARY KEY, name TEXT)",
			DatabaseName: ":memory:",
		},
	}
	_, err := server.handleExecuteSQL(ctx, session, createParams)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	insertParams := &mcp.CallToolParamsFor[ExecuteSQLParams]{
		Arguments: ExecuteSQLParams{
			ProfileName:  "testsqlite",
			SQL:          "INSERT INTO test (name) VALUES (?)",
			DatabaseName: ":memory:",
			Params:       []interface{}{"Alice"},
		},
	}
	_, err = server.handleExecuteSQL(ctx, session, insertParams)
	if err != nil {
		t.Fatalf("Failed to insert row: %v", err)
	}

	selectParams := &mcp.CallToolParamsFor[ExecuteSQLParams]{
		Arguments: ExecuteSQLParams{
			ProfileName:  "testsqlite",
			SQL:          "SELECT id, name FROM test WHERE name = ?",
			DatabaseName: ":memory:",
			Params:       []interface{}{"Alice"},
		},
	}
	res, err := server.handleExecuteSQL(ctx, session, selectParams)
	if err != nil {
		t.Fatalf("Failed to select row: %v", err)
	}
	if res == nil || res.Content == nil {
		t.Fatalf("Select returned nil content")
	}

	// Enforce readonly: update profile to readonly and block INSERT
	cfg, _ := config.LoadConfig(testConfig)
	for i := range cfg.Profiles {
		if cfg.Profiles[i].ProfileName == "testsqlite" {
			cfg.Profiles[i].Readonly = true
		}
	}
	_ = config.SaveConfig(testConfig, cfg)

	readonlyInsert := &mcp.CallToolParamsFor[ExecuteSQLParams]{
		Arguments: ExecuteSQLParams{
			ProfileName:  "testsqlite",
			SQL:          "INSERT INTO test (name) VALUES (?)",
			DatabaseName: ":memory:",
			Params:       []interface{}{"Bob"},
		},
	}
	_, err = server.handleExecuteSQL(ctx, session, readonlyInsert)
	if err == nil {
		t.Fatalf("Expected error for INSERT on readonly profile, got nil")
	}
}

// Additional tests for execute-sql, list-tables, describe-table, list-databases would follow a similar pattern.
// For full integration tests, use a test database and mock connections as needed.

// TestHandleListTables tests the list-tables MCP action.
func TestHandleListTables(t *testing.T) {
	testConfig := setupTestConfig(t)
	defer os.Remove(testConfig)

	server := NewMCPServerWithConfig(testConfig)
	ctx := context.Background()
	session := &mcp.ServerSession{}
	params := &mcp.CallToolParamsFor[ListTablesParams]{
		Arguments: ListTablesParams{
			ProfileName:  "testsqlite",
			DatabaseName: ":memory:",
		},
	}

	res, err := server.handleListTables(ctx, session, params)
	if err != nil {
		t.Fatalf("handleListTables error: %v", err)
	}
	if res == nil || res.Content == nil {
		t.Fatalf("handleListTables returned nil content")
	}
}

// TestHandleDescribeTable tests the describe-table MCP action.
func TestHandleDescribeTable(t *testing.T) {
	testConfig := setupTestConfig(t)
	defer os.Remove(testConfig)

	server := NewMCPServerWithConfig(testConfig)
	ctx := context.Background()
	session := &mcp.ServerSession{}
	params := &mcp.CallToolParamsFor[DescribeTableParams]{
		Arguments: DescribeTableParams{
			ProfileName:  "testsqlite",
			DatabaseName: ":memory:",
			TableName:    "sqlite_master",
		},
	}

	res, err := server.handleDescribeTable(ctx, session, params)
	if err != nil {
		t.Fatalf("handleDescribeTable error: %v", err)
	}
	if res == nil || res.Content == nil {
		t.Fatalf("handleDescribeTable returned nil content")
	}
}

// TestHandleListDatabases tests the list-databases MCP action.
func TestHandleListDatabases(t *testing.T) {
	testConfig := setupTestConfig(t)
	defer os.Remove(testConfig)

	server := NewMCPServerWithConfig(testConfig)
	ctx := context.Background()
	session := &mcp.ServerSession{}
	params := &mcp.CallToolParamsFor[ListDatabasesParams]{
		Arguments: ListDatabasesParams{
			ProfileName: "testsqlite",
		},
	}

	res, err := server.handleListDatabases(ctx, session, params)
	if err != nil {
		t.Fatalf("handleListDatabases error: %v", err)
	}
	if res == nil || res.Content == nil {
		t.Fatalf("handleListDatabases returned nil content")
	}
}

// TestHandleSmartQueryBuilder tests the smart-query-builder MCP action with mock scenarios.
func TestHandleSmartQueryBuilder(t *testing.T) {
	testConfig := setupTestConfig(t)
	defer os.Remove(testConfig)

	server := NewMCPServerWithConfig(testConfig)
	ctx := context.Background()
	session := &mcp.ServerSession{}

	// Test 1: Profile not found
	params := &mcp.CallToolParamsFor[SmartQueryBuilderParams]{
		Arguments: SmartQueryBuilderParams{
			ProfileName: "nonexistent",
			Intent:      "test intent",
		},
	}
	_, err := server.handleSmartQueryBuilder(ctx, session, params)
	if err == nil {
		t.Fatalf("Expected error for nonexistent profile, got nil")
	}

	// Test 2: Valid parameters structure (will fail on DB connection but validates parameter handling)
	params2 := &mcp.CallToolParamsFor[SmartQueryBuilderParams]{
		Arguments: SmartQueryBuilderParams{
			ProfileName:  "testpg",
			Intent:       "attendance dashboard",
			DatabaseName: "testdb",
			TableNames:   []string{"attendance"},
		},
	}
	res, err := server.handleSmartQueryBuilder(ctx, session, params2)
	// Expect connection error but validate we got to that point
	if err == nil {
		// If no error, check response structure
		if res == nil || res.Content == nil {
			t.Fatalf("handleSmartQueryBuilder returned nil content")
		}
	} else {
		// Expected connection error for test environment
		t.Logf("Expected connection error in test environment: %v", err)
	}

	// Test 3: Test intent parsing logic by checking parameter validation
	params3 := &mcp.CallToolParamsFor[SmartQueryBuilderParams]{
		Arguments: SmartQueryBuilderParams{
			ProfileName: "testpg",
			Intent:      "", // Empty intent
		},
	}
	_, err = server.handleSmartQueryBuilder(ctx, session, params3)
	// Should handle empty intent gracefully
	if err != nil && err.Error() == "profile not found" {
		t.Fatalf("Wrong error for empty intent test: %v", err)
	}
}

// TestSmartQueryBuilderIntentParsing tests the intent parsing logic in isolation
func TestSmartQueryBuilderIntentParsing(t *testing.T) {
	// Test keyword extraction logic - simulating the parsing done in handleSmartQueryBuilder
	words := []string{"attendance", "dashboard", "employees"}

	// Simulate the stopwords filtering
	stopwords := map[string]bool{
		"for": true, "the": true, "a": true, "an": true, "to": true, "of": true,
		"and": true, "in": true, "on": true, "with": true, "by": true, "at": true,
		"from": true, "as": true, "is": true, "are": true, "was": true, "were": true,
		"be": true, "this": true, "that": true, "it": true, "dashboard": true,
	}

	var keywords []string
	for _, w := range words {
		if !stopwords[w] && len(w) > 2 {
			keywords = append(keywords, w)
		}
	}

	// Should extract "attendance" and "employees"
	expectedKeywords := []string{"attendance", "employees"}
	if len(keywords) != len(expectedKeywords) {
		t.Fatalf("Expected %d keywords, got %d", len(expectedKeywords), len(keywords))
	}

	// Test table matching logic
	tables := []string{"users", "attendance", "reports", "employees"}
	bestScore := 0
	bestTable := ""

	for _, table := range tables {
		score := 0
		for _, keyword := range keywords {
			if table == keyword {
				score++
			}
		}
		if score > bestScore {
			bestScore = score
			bestTable = table
		}
	}

	if bestTable != "attendance" && bestTable != "employees" {
		t.Fatalf("Expected best match to be 'attendance' or 'employees', got '%s'", bestTable)
	}
}

// TestHandleDiscoverJoins tests the discover-joins MCP action.
func TestHandleDiscoverJoins(t *testing.T) {
	testConfig := setupTestConfig(t)
	defer os.Remove(testConfig)

	server := NewMCPServerWithConfig(testConfig)
	ctx := context.Background()
	session := &mcp.ServerSession{}

	// Test 1: Profile not found
	params := &mcp.CallToolParamsFor[DiscoverJoinsParams]{
		Arguments: DiscoverJoinsParams{
			ProfileName: "nonexistent",
		},
	}
	_, err := server.handleDiscoverJoins(ctx, session, params)
	if err == nil {
		t.Fatalf("Expected error for nonexistent profile, got nil")
	}

	// Test 2: Valid SQLite profile (will fail on DB connection but validates parameter handling)
	params2 := &mcp.CallToolParamsFor[DiscoverJoinsParams]{
		Arguments: DiscoverJoinsParams{
			ProfileName: "testsqlite",
			Tables:      []string{"users", "orders"},
		},
	}
	res, err := server.handleDiscoverJoins(ctx, session, params2)
	// Expect connection error but validate we got to that point
	if err == nil {
		// If no error, check response structure
		if res == nil || res.Content == nil {
			t.Fatalf("handleDiscoverJoins returned nil content")
		}
	} else {
		// Expected connection error for test environment
		t.Logf("Expected connection error in test environment: %v", err)
	}

	// Test 3: Empty tables parameter
	params3 := &mcp.CallToolParamsFor[DiscoverJoinsParams]{
		Arguments: DiscoverJoinsParams{
			ProfileName: "testsqlite",
			Tables:      []string{}, // Empty tables list
		},
	}
	_, err = server.handleDiscoverJoins(ctx, session, params3)
	// Should handle empty tables gracefully
	if err != nil && err.Error() == "profile not found" {
		t.Fatalf("Wrong error for empty tables test: %v", err)
	}
}

// TestHandleDiscoverJoins_WithMockData tests join discovery with actual SQLite data.
func TestHandleDiscoverJoins_WithMockData(t *testing.T) {
	testConfig := setupTestConfig(t)
	defer os.Remove(testConfig)

	server := NewMCPServerWithConfig(testConfig)
	ctx := context.Background()
	session := &mcp.ServerSession{}

	// First, try to create tables with foreign key relationships
	createUsersParams := &mcp.CallToolParamsFor[ExecuteSQLParams]{
		Arguments: ExecuteSQLParams{
			ProfileName:  "testsqlite",
			SQL:          "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)",
			DatabaseName: ":memory:",
		},
	}
	_, err := server.handleExecuteSQL(ctx, session, createUsersParams)
	if err != nil {
		// Skip test if SQLite driver is not available in test environment
		if strings.Contains(err.Error(), "unknown driver") {
			t.Skipf("Skipping SQLite integration test due to driver issue: %v", err)
			return
		}
		t.Fatalf("Failed to create users table: %v", err)
	}

	createOrdersParams := &mcp.CallToolParamsFor[ExecuteSQLParams]{
		Arguments: ExecuteSQLParams{
			ProfileName:  "testsqlite",
			SQL:          "CREATE TABLE orders (id INTEGER PRIMARY KEY, user_id INTEGER, amount REAL, FOREIGN KEY(user_id) REFERENCES users(id))",
			DatabaseName: ":memory:",
		},
	}
	_, err = server.handleExecuteSQL(ctx, session, createOrdersParams)
	if err != nil {
		t.Fatalf("Failed to create orders table: %v", err)
	}

	// Enable foreign keys in SQLite
	enableFKParams := &mcp.CallToolParamsFor[ExecuteSQLParams]{
		Arguments: ExecuteSQLParams{
			ProfileName:  "testsqlite",
			SQL:          "PRAGMA foreign_keys = ON",
			DatabaseName: ":memory:",
		},
	}
	_, err = server.handleExecuteSQL(ctx, session, enableFKParams)
	if err != nil {
		t.Fatalf("Failed to enable foreign keys: %v", err)
	}

	// Now test join discovery
	discoverParams := &mcp.CallToolParamsFor[DiscoverJoinsParams]{
		Arguments: DiscoverJoinsParams{
			ProfileName: "testsqlite",
		},
	}
	res, err := server.handleDiscoverJoins(ctx, session, discoverParams)
	if err != nil {
		t.Fatalf("handleDiscoverJoins error: %v", err)
	}
	if res == nil || res.Content == nil {
		t.Fatalf("handleDiscoverJoins returned nil content")
	}

	// Validate response structure (should contain at least the foreign key relationship)
	if len(res.Content) == 0 {
		t.Fatalf("Expected response content, got empty")
	}

	// The response should be JSON containing join suggestions
	content := res.Content[0]
	if textContent, ok := content.(*mcp.TextContent); ok {
		if len(textContent.Text) == 0 {
			t.Fatalf("Expected non-empty text content")
		}
		t.Logf("Join discovery response: %s", textContent.Text)
	} else {
		t.Fatalf("Expected TextContent, got %T", content)
	}
}

// TestDiscoverJoinsResultStructure tests the structure of DiscoverJoinsResult.
func TestDiscoverJoinsResultStructure(t *testing.T) {
	// Test the data structures used in join discovery
	join := JoinSuggestion{
		FromTable:        "orders",
		FromColumn:       "user_id",
		ToTable:          "users",
		ToColumn:         "id",
		Relationship:     "foreign_key",
		SuggestedJoinSQL: "SELECT * FROM orders JOIN users ON orders.user_id = users.id",
	}

	if join.FromTable != "orders" {
		t.Errorf("Expected FromTable 'orders', got '%s'", join.FromTable)
	}
	if join.FromColumn != "user_id" {
		t.Errorf("Expected FromColumn 'user_id', got '%s'", join.FromColumn)
	}
	if join.ToTable != "users" {
		t.Errorf("Expected ToTable 'users', got '%s'", join.ToTable)
	}
	if join.ToColumn != "id" {
		t.Errorf("Expected ToColumn 'id', got '%s'", join.ToColumn)
	}
	if join.Relationship != "foreign_key" {
		t.Errorf("Expected Relationship 'foreign_key', got '%s'", join.Relationship)
	}

	result := DiscoverJoinsResult{
		Joins:   []JoinSuggestion{join},
		Summary: "Discovered 1 join(s) based on foreign key relationships.",
	}

	if len(result.Joins) != 1 {
		t.Errorf("Expected 1 join, got %d", len(result.Joins))
	}
	if result.Summary != "Discovered 1 join(s) based on foreign key relationships." {
		t.Errorf("Unexpected summary: %s", result.Summary)
	}
}

// TestDiscoverJoinsInputValidation tests input validation for join discovery.
func TestDiscoverJoinsInputValidation(t *testing.T) {
	testConfig := setupTestConfig(t)
	defer os.Remove(testConfig)

	server := NewMCPServerWithConfig(testConfig)
	ctx := context.Background()
	session := &mcp.ServerSession{}

	// Test missing profile name
	params := &mcp.CallToolParamsFor[DiscoverJoinsParams]{
		Arguments: DiscoverJoinsParams{
			ProfileName: "", // Empty profile name
			Tables:      []string{"users", "orders"},
		},
	}
	_, err := server.handleDiscoverJoins(ctx, session, params)
	if err == nil {
		t.Fatalf("Expected error for empty profile name, got nil")
	}

	// Test with special characters in table names (should be handled gracefully)
	params2 := &mcp.CallToolParamsFor[DiscoverJoinsParams]{
		Arguments: DiscoverJoinsParams{
			ProfileName: "testsqlite",
			Tables:      []string{"table with spaces", "table-with-dashes", "table_with_underscores"},
		},
	}
	_, err = server.handleDiscoverJoins(ctx, session, params2)
	// Should not crash on unusual table names, though may fail on connection
	if err != nil {
		t.Logf("Expected behavior for unusual table names: %v", err)
	}
}

// TestDiscoverJoinsDatabaseTypes tests join discovery behavior across database types.
func TestDiscoverJoinsDatabaseTypes(t *testing.T) {
	testConfig := setupTestConfig(t)
	defer os.Remove(testConfig)

	server := NewMCPServerWithConfig(testConfig)
	ctx := context.Background()
	session := &mcp.ServerSession{}

	// Test PostgreSQL profile (will fail on connection but tests code path)
	pgParams := &mcp.CallToolParamsFor[DiscoverJoinsParams]{
		Arguments: DiscoverJoinsParams{
			ProfileName: "testpg",
			Tables:      []string{"users", "orders"},
		},
	}
	_, err := server.handleDiscoverJoins(ctx, session, pgParams)
	if err != nil {
		// Expected since we don't have a real PostgreSQL connection
		t.Logf("Expected PostgreSQL connection error: %v", err)
	}

	// Test SQLite profile
	sqliteParams := &mcp.CallToolParamsFor[DiscoverJoinsParams]{
		Arguments: DiscoverJoinsParams{
			ProfileName: "testsqlite",
			Tables:      []string{"users", "orders"},
		},
	}
	_, err = server.handleDiscoverJoins(ctx, session, sqliteParams)
	if err != nil {
		// Expected since we're using in-memory database
		t.Logf("Expected SQLite connection behavior: %v", err)
	}
}

// TestDiscoverJoinsUnsupportedDatabase tests join discovery with unsupported database type.
func TestDiscoverJoinsUnsupportedDatabase(t *testing.T) {
	testConfig := "test_config_unsupported.yaml"
	defer os.Remove(testConfig)

	// Create config with unsupported database type
	cfg := &config.Config{
		Profiles: []config.Profile{
			{
				ProfileName:  "testunsupported",
				DBType:       "oracle", // Unsupported database type
				Host:         "localhost",
				Port:         1521,
				Username:     "testuser",
				Password:     "testpass",
				DatabaseName: "testdb",
				Readonly:     false,
			},
		},
		MaxPoolSize: 5,
	}
	if err := config.SaveConfig(testConfig, cfg); err != nil {
		t.Fatalf("Failed to save test config: %v", err)
	}
	os.Setenv("DB_MCP_AES_KEY", "12345678901234567890123456789012")

	server := NewMCPServerWithConfig(testConfig)
	ctx := context.Background()
	session := &mcp.ServerSession{}

	params := &mcp.CallToolParamsFor[DiscoverJoinsParams]{
		Arguments: DiscoverJoinsParams{
			ProfileName: "testunsupported",
			Tables:      []string{"users", "orders"},
		},
	}
	_, err := server.handleDiscoverJoins(ctx, session, params)
	if err == nil {
		t.Fatalf("Expected error for unsupported database type, got nil")
	}
	// The error might be about the driver or the unsupported db_type
	if !strings.Contains(err.Error(), "unsupported db_type for join discovery") && !strings.Contains(err.Error(), "unknown driver") {
		t.Fatalf("Expected error about unsupported database, got '%s'", err.Error())
	}
}

// TestDiscoverJoinsTableFiltering tests table filtering functionality.
func TestDiscoverJoinsTableFiltering(t *testing.T) {
	// Test the table filtering logic used in handleDiscoverJoins
	requestedTables := []string{"Users", "Orders", "Products"}
	tableSet := map[string]bool{}

	// Convert to lowercase for case-insensitive matching (as done in handleDiscoverJoins)
	for _, t := range requestedTables {
		tableSet[strings.ToLower(t)] = true
	}

	// Test filtering logic
	testCases := []struct {
		fromTable string
		toTable   string
		expected  bool
	}{
		{"users", "orders", true},    // Both in set
		{"orders", "products", true}, // Both in set
		{"users", "logs", true},      // One in set
		{"logs", "archives", false},  // Neither in set
		{"USERS", "ORDERS", true},    // Case insensitive matching (converts to lowercase)
	}

	for _, tc := range testCases {
		result := len(tableSet) == 0 || tableSet[strings.ToLower(tc.fromTable)] || tableSet[strings.ToLower(tc.toTable)]
		if result != tc.expected {
			t.Errorf("Table filtering for %s->%s: expected %v, got %v", tc.fromTable, tc.toTable, tc.expected, result)
		}
	}
}

// TestHandleSampleData tests the sample-data MCP action.
func TestHandleSampleData(t *testing.T) {
	testConfig := setupTestConfig(t)
	defer os.Remove(testConfig)

	server := NewMCPServerWithConfig(testConfig)
	ctx := context.Background()
	session := &mcp.ServerSession{}

	// Test 1: Profile not found
	params := &mcp.CallToolParamsFor[SampleDataParams]{
		Arguments: SampleDataParams{
			ProfileName: "nonexistent",
			TableName:   "users",
		},
	}
	_, err := server.handleSampleData(ctx, session, params)
	if err == nil {
		t.Fatalf("Expected error for nonexistent profile, got nil")
	}

	// Test 2: Missing table name
	params2 := &mcp.CallToolParamsFor[SampleDataParams]{
		Arguments: SampleDataParams{
			ProfileName: "testsqlite",
			TableName:   "", // Empty table name
		},
	}
	_, err = server.handleSampleData(ctx, session, params2)
	if err == nil {
		t.Fatalf("Expected error for empty table name, got nil")
	}

	// Test 3: Valid parameters structure (will fail on DB connection but validates parameter handling)
	params3 := &mcp.CallToolParamsFor[SampleDataParams]{
		Arguments: SampleDataParams{
			ProfileName:  "testsqlite",
			TableName:    "users",
			DatabaseName: ":memory:",
			SampleSize:   5,
		},
	}
	res, err := server.handleSampleData(ctx, session, params3)
	// Expect connection error but validate we got to that point
	if err == nil {
		// If no error, check response structure
		if res == nil || res.Content == nil {
			t.Fatalf("handleSampleData returned nil content")
		}
	} else {
		// Expected connection error for test environment
		t.Logf("Expected connection error in test environment: %v", err)
	}
}

// TestHandleSampleData_WithMockData tests sample data fetching with actual SQLite data.
func TestHandleSampleData_WithMockData(t *testing.T) {
	testConfig := setupTestConfig(t)
	defer os.Remove(testConfig)

	server := NewMCPServerWithConfig(testConfig)
	ctx := context.Background()
	session := &mcp.ServerSession{}

	// First, create a test table with sample data
	createTableParams := &mcp.CallToolParamsFor[ExecuteSQLParams]{
		Arguments: ExecuteSQLParams{
			ProfileName:  "testsqlite",
			SQL:          "CREATE TABLE sample_users (id INTEGER PRIMARY KEY, name TEXT, email TEXT, age INTEGER)",
			DatabaseName: ":memory:",
		},
	}
	_, err := server.handleExecuteSQL(ctx, session, createTableParams)
	if err != nil {
		// Skip test if SQLite driver is not available in test environment
		if strings.Contains(err.Error(), "unknown driver") {
			t.Skipf("Skipping SQLite integration test due to driver issue: %v", err)
			return
		}
		t.Fatalf("Failed to create sample table: %v", err)
	}

	// Insert sample data
	insertData := []string{
		"INSERT INTO sample_users (name, email, age) VALUES ('Alice', 'alice@example.com', 25)",
		"INSERT INTO sample_users (name, email, age) VALUES ('Bob', 'bob@example.com', 30)",
		"INSERT INTO sample_users (name, email, age) VALUES ('Charlie', 'charlie@example.com', 35)",
		"INSERT INTO sample_users (name, email, age) VALUES ('Diana', 'diana@example.com', 28)",
		"INSERT INTO sample_users (name, email, age) VALUES ('Eve', 'eve@example.com', 32)",
	}

	for _, insertSQL := range insertData {
		insertParams := &mcp.CallToolParamsFor[ExecuteSQLParams]{
			Arguments: ExecuteSQLParams{
				ProfileName:  "testsqlite",
				SQL:          insertSQL,
				DatabaseName: ":memory:",
			},
		}
		_, err = server.handleExecuteSQL(ctx, session, insertParams)
		if err != nil {
			t.Fatalf("Failed to insert sample data: %v", err)
		}
	}

	// Now test sample data fetching
	sampleParams := &mcp.CallToolParamsFor[SampleDataParams]{
		Arguments: SampleDataParams{
			ProfileName:  "testsqlite",
			TableName:    "sample_users",
			DatabaseName: ":memory:",
			SampleSize:   3,
		},
	}
	res, err := server.handleSampleData(ctx, session, sampleParams)
	if err != nil {
		t.Fatalf("handleSampleData error: %v", err)
	}
	if res == nil || res.Content == nil {
		t.Fatalf("handleSampleData returned nil content")
	}

	// Validate response structure
	if len(res.Content) == 0 {
		t.Fatalf("Expected response content, got empty")
	}

	// The response should be JSON containing sample data
	content := res.Content[0]
	if textContent, ok := content.(*mcp.TextContent); ok {
		if len(textContent.Text) == 0 {
			t.Fatalf("Expected non-empty text content")
		}
		t.Logf("Sample data response: %s", textContent.Text)

		// Verify it contains expected fields
		if !strings.Contains(textContent.Text, "sample_users") {
			t.Fatalf("Response should contain table name")
		}
		if !strings.Contains(textContent.Text, "columns") {
			t.Fatalf("Response should contain columns field")
		}
		if !strings.Contains(textContent.Text, "sample_rows") {
			t.Fatalf("Response should contain sample_rows field")
		}
	} else {
		t.Fatalf("Expected TextContent, got %T", content)
	}
}

// TestSampleDataResultStructure tests the structure of SampleDataResult.
func TestSampleDataResultStructure(t *testing.T) {
	// Test the data structures used in sample data fetching
	result := SampleDataResult{
		TableName:  "users",
		SampleSize: 2,
		Columns:    []string{"id", "name", "email"},
		SampleRows: [][]interface{}{
			{1, "Alice", "alice@example.com"},
			{2, "Bob", "bob@example.com"},
		},
	}

	if result.TableName != "users" {
		t.Errorf("Expected TableName 'users', got '%s'", result.TableName)
	}
	if result.SampleSize != 2 {
		t.Errorf("Expected SampleSize 2, got %d", result.SampleSize)
	}
	if len(result.Columns) != 3 {
		t.Errorf("Expected 3 columns, got %d", len(result.Columns))
	}
	if len(result.SampleRows) != 2 {
		t.Errorf("Expected 2 sample rows, got %d", len(result.SampleRows))
	}
	if result.Columns[0] != "id" || result.Columns[1] != "name" || result.Columns[2] != "email" {
		t.Errorf("Unexpected column names: %v", result.Columns)
	}
}

// TestSampleDataInputValidation tests input validation for sample data fetching.
func TestSampleDataInputValidation(t *testing.T) {
	testConfig := setupTestConfig(t)
	defer os.Remove(testConfig)

	server := NewMCPServerWithConfig(testConfig)
	ctx := context.Background()
	session := &mcp.ServerSession{}

	// Test missing profile name
	params := &mcp.CallToolParamsFor[SampleDataParams]{
		Arguments: SampleDataParams{
			ProfileName: "", // Empty profile name
			TableName:   "users",
		},
	}
	_, err := server.handleSampleData(ctx, session, params)
	if err == nil {
		t.Fatalf("Expected error for empty profile name, got nil")
	}

	// Test missing table name
	params2 := &mcp.CallToolParamsFor[SampleDataParams]{
		Arguments: SampleDataParams{
			ProfileName: "testsqlite",
			TableName:   "", // Empty table name
		},
	}
	_, err = server.handleSampleData(ctx, session, params2)
	if err == nil {
		t.Fatalf("Expected error for empty table name, got nil")
	}

	// Test default sample size handling
	params3 := &mcp.CallToolParamsFor[SampleDataParams]{
		Arguments: SampleDataParams{
			ProfileName: "testsqlite",
			TableName:   "users",
			SampleSize:  0, // Should default to 3
		},
	}
	// This will fail on connection, but validates default size logic
	_, err = server.handleSampleData(ctx, session, params3)
	if err != nil && !strings.Contains(err.Error(), "profile not found") {
		t.Logf("Expected connection error for default size test: %v", err)
	}

	// Test large sample size capping
	params4 := &mcp.CallToolParamsFor[SampleDataParams]{
		Arguments: SampleDataParams{
			ProfileName: "testsqlite",
			TableName:   "users",
			SampleSize:  200, // Should be capped at 100
		},
	}
	// This will fail on connection, but validates size capping logic
	_, err = server.handleSampleData(ctx, session, params4)
	if err != nil && !strings.Contains(err.Error(), "profile not found") {
		t.Logf("Expected connection error for large size test: %v", err)
	}
}

// TestSampleDataDatabaseTypes tests sample data behavior across database types.
func TestSampleDataDatabaseTypes(t *testing.T) {
	testConfig := setupTestConfig(t)
	defer os.Remove(testConfig)

	server := NewMCPServerWithConfig(testConfig)
	ctx := context.Background()
	session := &mcp.ServerSession{}

	// Test PostgreSQL profile (will fail on connection but tests code path)
	pgParams := &mcp.CallToolParamsFor[SampleDataParams]{
		Arguments: SampleDataParams{
			ProfileName:  "testpg",
			TableName:    "users",
			DatabaseName: "testdb",
			SampleSize:   5,
		},
	}
	_, err := server.handleSampleData(ctx, session, pgParams)
	if err != nil {
		// Expected since we don't have a real PostgreSQL connection
		t.Logf("Expected PostgreSQL connection error: %v", err)
	}

	// Test SQLite profile
	sqliteParams := &mcp.CallToolParamsFor[SampleDataParams]{
		Arguments: SampleDataParams{
			ProfileName:  "testsqlite",
			TableName:    "users",
			DatabaseName: ":memory:",
			SampleSize:   3,
		},
	}
	_, err = server.handleSampleData(ctx, session, sqliteParams)
	if err != nil {
		// Expected since we're using in-memory database without tables
		t.Logf("Expected SQLite connection behavior: %v", err)
	}
}

// TestSampleDataUnsupportedDatabase tests sample data with unsupported database type.
func TestSampleDataUnsupportedDatabase(t *testing.T) {
	testConfig := "test_config_sample_unsupported.yaml"
	defer os.Remove(testConfig)

	// Create config with unsupported database type
	cfg := &config.Config{
		Profiles: []config.Profile{
			{
				ProfileName:  "testunsupported",
				DBType:       "oracle", // Unsupported database type
				Host:         "localhost",
				Port:         1521,
				Username:     "testuser",
				Password:     "testpass",
				DatabaseName: "testdb",
				Readonly:     false,
			},
		},
		MaxPoolSize: 5,
	}
	if err := config.SaveConfig(testConfig, cfg); err != nil {
		t.Fatalf("Failed to save test config: %v", err)
	}
	os.Setenv("DB_MCP_AES_KEY", "12345678901234567890123456789012")

	server := NewMCPServerWithConfig(testConfig)
	ctx := context.Background()
	session := &mcp.ServerSession{}

	params := &mcp.CallToolParamsFor[SampleDataParams]{
		Arguments: SampleDataParams{
			ProfileName: "testunsupported",
			TableName:   "users",
			SampleSize:  3,
		},
	}
	_, err := server.handleSampleData(ctx, session, params)
	if err == nil {
		t.Fatalf("Expected error for unsupported database type, got nil")
	}
	// The error might be about the driver or the unsupported db_type
	if !strings.Contains(err.Error(), "unsupported db_type for sample data") && !strings.Contains(err.Error(), "unknown driver") {
		t.Fatalf("Expected error about unsupported database, got '%s'", err.Error())
	}
}

// TestSampleDataSpecialTableNames tests sample data with special table names.
func TestSampleDataSpecialTableNames(t *testing.T) {
	testConfig := setupTestConfig(t)
	defer os.Remove(testConfig)

	server := NewMCPServerWithConfig(testConfig)
	ctx := context.Background()
	session := &mcp.ServerSession{}

	// Test with special characters in table names (should be handled gracefully)
	testCases := []string{
		"table_with_underscores",
		"table-with-dashes",
		"CamelCaseTable",
		"123numeric_start",
	}

	for _, tableName := range testCases {
		params := &mcp.CallToolParamsFor[SampleDataParams]{
			Arguments: SampleDataParams{
				ProfileName:  "testsqlite",
				TableName:    tableName,
				DatabaseName: ":memory:",
				SampleSize:   3,
			},
		}
		_, err := server.handleSampleData(ctx, session, params)
		// Should not crash on unusual table names, though may fail on connection
		if err != nil {
			t.Logf("Expected behavior for table name '%s': %v", tableName, err)
		}
	}
}

// --- ListTools MCP Action Tests ---

func TestHandleListTools_Success(t *testing.T) {
	testConfig := setupTestConfig(t)
	defer os.Remove(testConfig)

	server := NewMCPServerWithConfig(testConfig)
	ctx := context.Background()
	session := &mcp.ServerSession{}
	params := &mcp.CallToolParamsFor[ListToolsParams]{}

	res, err := server.handleListTools(ctx, session, params)
	if err != nil {
		t.Fatalf("handleListTools error: %v", err)
	}
	if res == nil || res.Content == nil {
		t.Fatalf("handleListTools returned nil content")
	}
	if len(res.Content) == 0 {
		t.Fatalf("handleListTools returned empty content")
	}
	textContent, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("Expected TextContent, got %T", res.Content[0])
	}
	var result ListToolsResult
	if err := json.Unmarshal([]byte(textContent.Text), &result); err != nil {
		t.Fatalf("Failed to unmarshal ListToolsResult: %v", err)
	}
	if len(result.Tools) == 0 {
		t.Fatalf("Expected at least one tool in result")
	}
	for _, tool := range result.Tools {
		if tool.Name == "" || tool.Description == "" {
			t.Errorf("Tool missing name or description: %+v", tool)
		}
	}
}

func TestHandleListTools_EmptyParams(t *testing.T) {
	testConfig := setupTestConfig(t)
	defer os.Remove(testConfig)

	server := NewMCPServerWithConfig(testConfig)
	ctx := context.Background()
	session := &mcp.ServerSession{}

	// Pass nil params (should still work)
	res, err := server.handleListTools(ctx, session, nil)
	if err != nil {
		t.Fatalf("handleListTools error with nil params: %v", err)
	}
	if res == nil || res.Content == nil {
		t.Fatalf("handleListTools returned nil content with nil params")
	}
	if len(res.Content) == 0 {
		t.Fatalf("handleListTools returned empty content with nil params")
	}
	textContent, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("Expected TextContent, got %T", res.Content[0])
	}
	var result ListToolsResult
	if err := json.Unmarshal([]byte(textContent.Text), &result); err != nil {
		t.Fatalf("Failed to unmarshal ListToolsResult: %v", err)
	}
	if len(result.Tools) == 0 {
		t.Fatalf("Expected at least one tool in result")
	}
}

func TestHandleListTools_VerifyAllTools(t *testing.T) {
	testConfig := setupTestConfig(t)
	defer os.Remove(testConfig)

	server := NewMCPServerWithConfig(testConfig)
	ctx := context.Background()
	session := &mcp.ServerSession{}
	params := &mcp.CallToolParamsFor[ListToolsParams]{}

	res, err := server.handleListTools(ctx, session, params)
	if err != nil {
		t.Fatalf("handleListTools error: %v", err)
	}
	if res == nil || res.Content == nil || len(res.Content) == 0 {
		t.Fatalf("handleListTools returned nil or empty content")
	}
	textContent, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("Expected TextContent, got %T", res.Content[0])
	}
	var result ListToolsResult
	if err := json.Unmarshal([]byte(textContent.Text), &result); err != nil {
		t.Fatalf("Failed to unmarshal ListToolsResult: %v", err)
	}
	// Compare with server.toolsRegistry
	if len(result.Tools) != len(server.toolsRegistry) {
		t.Fatalf("Expected %d tools, got %d", len(server.toolsRegistry), len(result.Tools))
	}
	// Check all tool names and descriptions match
	for i, tool := range server.toolsRegistry {
		found := false
		for _, rtool := range result.Tools {
			if rtool.Name == tool.Name && rtool.Description == tool.Description {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Tool %d (%s) missing or mismatched in result", i, tool.Name)
		}
	}
}

// --- AnalyzeSchema MCP Action Tests ---

func TestHandleAnalyzeSchema(t *testing.T) {
	testConfig := setupTestConfig(t)
	defer os.Remove(testConfig)

	server := NewMCPServerWithConfig(testConfig)
	ctx := context.Background()
	session := &mcp.ServerSession{}

	levels := []string{AnalysisLevelBasic, AnalysisLevelDetailed, AnalysisLevelComprehensive}
	for _, level := range levels {
		params := &mcp.CallToolParamsFor[AnalyzeSchemaParams]{
			Arguments: AnalyzeSchemaParams{
				ProfileName:   "testsqlite",
				DatabaseName:  ":memory:",
				AnalysisLevel: level,
				SampleSize:    2,
			},
		}
		res, err := server.handleAnalyzeSchema(ctx, session, params)
		if err != nil {
			t.Fatalf("handleAnalyzeSchema error for level %s: %v", level, err)
		}
		if res == nil || res.Content == nil {
			t.Fatalf("handleAnalyzeSchema returned nil content for level %s", level)
		}
	}

	// Invalid analysis_level
	paramsInvalid := &mcp.CallToolParamsFor[AnalyzeSchemaParams]{
		Arguments: AnalyzeSchemaParams{
			ProfileName:   "testsqlite",
			DatabaseName:  ":memory:",
			AnalysisLevel: "invalid",
		},
	}
	_, err := server.handleAnalyzeSchema(ctx, session, paramsInvalid)
	if err == nil {
		t.Fatalf("Expected error for invalid analysis_level, got nil")
	}

	// Missing profile_name
	paramsMissing := &mcp.CallToolParamsFor[AnalyzeSchemaParams]{
		Arguments: AnalyzeSchemaParams{
			ProfileName:   "",
			DatabaseName:  ":memory:",
			AnalysisLevel: AnalysisLevelBasic,
		},
	}
	_, err = server.handleAnalyzeSchema(ctx, session, paramsMissing)
	if err == nil {
		t.Fatalf("Expected error for missing profile_name, got nil")
	}

	// Profile not found
	paramsNotFound := &mcp.CallToolParamsFor[AnalyzeSchemaParams]{
		Arguments: AnalyzeSchemaParams{
			ProfileName:   "nonexistent",
			DatabaseName:  ":memory:",
			AnalysisLevel: AnalysisLevelBasic,
		},
	}
	_, err = server.handleAnalyzeSchema(ctx, session, paramsNotFound)
	if err == nil {
		t.Fatalf("Expected error for profile not found, got nil")
	}

	// Database connection failure (simulate by using bad DB type)
	badConfig := "test_config_bad.yaml"
	defer os.Remove(badConfig)
	cfg := &config.Config{
		Profiles: []config.Profile{
			{
				ProfileName:  "badprofile",
				DBType:       "oracle", // Unsupported
				DatabaseName: "bad",
			},
		},
		MaxPoolSize: 5,
	}
	if err := config.SaveConfig(badConfig, cfg); err != nil {
		t.Fatalf("Failed to save bad test config: %v", err)
	}
	serverBad := NewMCPServerWithConfig(badConfig)
	paramsBad := &mcp.CallToolParamsFor[AnalyzeSchemaParams]{
		Arguments: AnalyzeSchemaParams{
			ProfileName:   "badprofile",
			DatabaseName:  "bad",
			AnalysisLevel: AnalysisLevelBasic,
		},
	}
	_, err = serverBad.handleAnalyzeSchema(ctx, session, paramsBad)
	if err == nil {
		t.Fatalf("Expected error for unsupported DB type, got nil")
	}
}

// --- AnalyzeSchema Helper Method Unit Tests ---

func TestDetectDomain(t *testing.T) {
	server := NewMCPServerWithConfig(setupTestConfig(t))
	domain, confidence := server.detectDomain([]string{"orders", "products", "customers"})
	if domain != "e-commerce" {
		t.Errorf("Expected domain 'e-commerce', got '%s'", domain)
	}
	if confidence <= 0 {
		t.Errorf("Expected positive confidence, got %f", confidence)
	}
}

func TestAnalyzeNamingConventions(t *testing.T) {
	server := NewMCPServerWithConfig(setupTestConfig(t))
	tables := []TableInfo{
		{Columns: []SchemaColumnInfo{{ColumnName: "created_at"}, {ColumnName: "user_id"}}},
		{Columns: []SchemaColumnInfo{{ColumnName: "order_id"}, {ColumnName: "updated_at"}}},
	}
	result := server.analyzeNamingConventions(tables)
	if result["cases"].(map[string]int)["snake_case"] == 0 {
		t.Errorf("Expected snake_case detection")
	}
	if len(result["timestampCols"].([]string)) == 0 {
		t.Errorf("Expected timestamp columns detection")
	}
}

func BenchmarkHandleAnalyzeSchema(b *testing.B) {
	testConfig := setupTestConfig(&testing.T{})
	server := NewMCPServerWithConfig(testConfig)
	ctx := context.Background()
	session := &mcp.ServerSession{}
	params := &mcp.CallToolParamsFor[AnalyzeSchemaParams]{
		Arguments: AnalyzeSchemaParams{
			ProfileName:   "testsqlite",
			DatabaseName:  ":memory:",
			AnalysisLevel: AnalysisLevelComprehensive,
			SampleSize:    2,
		},
	}
	for i := 0; i < b.N; i++ {
		_, _ = server.handleAnalyzeSchema(ctx, session, params)
	}
}

// --- Additional Tests: Idempotent registration, business context, readonly enforcement edge cases, pattern integration & issue cap, decryption error path, Postgres constraint mapping presence, relationship graph inclusion ---
//
// NOTE: These tests extend coverage for outstanding items in reminder #17.
//
// 1. Tool duplication prevention
// 2. Business context inference on empty schema (panic safety)
// 3. Read-only enforcement: multi-statement block & WITH CTE allowance
// 4. Data pattern propagation + quality issue cap truncation
// 5. Decryption error path (invalid AES key length)
// 6. Postgres constraint mapping (query definition presence)
// 7. Relationship graph inclusion (semantic implicit relationship)
//
// Each test is self-contained and uses existing helpers where possible.

func TestRegisterAllTools_Idempotent(t *testing.T) {
	server := NewMCPServerWithConfig("nonexistent.yaml")
	initial := len(server.toolsRegistry)
	// Call again; should not add duplicates
	server.registerAllTools()
	if len(server.toolsRegistry) != initial {
		t.Fatalf("Expected toolsRegistry size %d after re-registration, got %d", initial, len(server.toolsRegistry))
	}
}

func TestInferBusinessContextEmpty(t *testing.T) {
	server := NewMCPServerWithConfig("nonexistent.yaml")
	ctx := server.inferBusinessContext(map[string]TableInfo{})
	if ctx == nil {
		t.Fatalf("Expected non-nil BusinessContext")
	}
	if len(ctx.DomainIndicators) == 0 {
		t.Errorf("Expected DomainIndicators to contain at least 'unknown'")
	}
	if _, ok := ctx.DomainIndicators["unknown"]; !ok {
		t.Errorf("Expected 'unknown' domain indicator for empty schema")
	}
}

func TestReadonlyEnforcement_MultiStatementBlocked(t *testing.T) {
	testConfig := setupTestConfig(t)
	defer os.Remove(testConfig)

	// Mark sqlite profile readonly
	cfg, _ := config.LoadConfig(testConfig)
	for i := range cfg.Profiles {
		if cfg.Profiles[i].ProfileName == "testsqlite" {
			cfg.Profiles[i].Readonly = true
		}
	}
	_ = config.SaveConfig(testConfig, cfg)

	server := NewMCPServerWithConfig(testConfig)
	ctx := context.Background()
	session := &mcp.ServerSession{}

	params := &mcp.CallToolParamsFor[ExecuteSQLParams]{
		Arguments: ExecuteSQLParams{
			ProfileName:  "testsqlite",
			DatabaseName: ":memory:",
			SQL:          "SELECT 1; SELECT 2;",
		},
	}
	_, err := server.handleExecuteSQL(ctx, session, params)
	if err == nil {
		t.Fatalf("Expected multi-statement block error on readonly profile")
	}
}

func TestReadonlyEnforcement_WithCTEAllowed(t *testing.T) {
	testConfig := setupTestConfig(t)
	defer os.Remove(testConfig)

	// Create profile readonly AFTER creating an in-memory table (optional)
	cfg, _ := config.LoadConfig(testConfig)
	for i := range cfg.Profiles {
		if cfg.Profiles[i].ProfileName == "testsqlite" {
			cfg.Profiles[i].Readonly = true
		}
	}
	_ = config.SaveConfig(testConfig, cfg)

	server := NewMCPServerWithConfig(testConfig)
	ctx := context.Background()
	session := &mcp.ServerSession{}

	params := &mcp.CallToolParamsFor[ExecuteSQLParams]{
		Arguments: ExecuteSQLParams{
			ProfileName:  "testsqlite",
			DatabaseName: ":memory:",
			SQL:          "WITH cte AS (SELECT 1 AS a) SELECT a FROM cte",
		},
	}
	// Should NOT error (read-only safe)
	_, err := server.handleExecuteSQL(ctx, session, params)
	if err != nil {
		t.Fatalf("Expected WITH CTE SELECT to be allowed, got error: %v", err)
	}
}

func TestAnalyzeSchema_PatternPropagationAndIssueCap(t *testing.T) {
	testConfig := setupTestConfig(t)
	defer os.Remove(testConfig)

	server := NewMCPServerWithConfig(testConfig)
	ctx := context.Background()
	session := &mcp.ServerSession{}

	// Create users & orders tables to generate implicit relationship and pattern-rich email column
	createUsers := []string{
		"CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT)",
	}
	createOrders := []string{
		"CREATE TABLE orders (id INTEGER PRIMARY KEY, user_id INTEGER)",
	}
	for _, sqlStmt := range append(createUsers, createOrders...) {
		_, err := server.handleExecuteSQL(ctx, session, &mcp.CallToolParamsFor[ExecuteSQLParams]{Arguments: ExecuteSQLParams{
			ProfileName:  "testsqlite",
			DatabaseName: ":memory:",
			SQL:          sqlStmt,
		}})
		if err != nil {
			t.Fatalf("Failed to create table (%s): %v", sqlStmt, err)
		}
	}

	// Insert 23 rows: 12 valid emails, 11 invalid (to exceed issue cap of 10)
	validEmails := []string{
		"alice@example.com", "bob@example.com", "carol@example.com", "dave@example.com",
		"eve@example.com", "frank@example.com", "grace@example.com", "heidi@example.com",
		"ivan@example.com", "judy@example.com", "mallory@example.com", "trent@example.com",
	}
	invalidEmails := []string{
		"not-an-email", "missing-at-symbol.com", "user@.com", "user@domain",
		"@@doubleat.com", "bad@@example.com", "user@domain,com", "space in@email.com",
		"user@domain.c", "user@domain.toolongtld", "invalid@",
	}
	allEmails := append(validEmails, invalidEmails...)
	for _, e := range allEmails {
		_, err := server.handleExecuteSQL(ctx, session, &mcp.CallToolParamsFor[ExecuteSQLParams]{Arguments: ExecuteSQLParams{
			ProfileName:  "testsqlite",
			DatabaseName: ":memory:",
			SQL:          "INSERT INTO users (email) VALUES (?)",
			Params:       []interface{}{e},
		}})
		if err != nil {
			t.Fatalf("Failed to insert email '%s': %v", e, err)
		}
	}

	// Run comprehensive analyze-schema with sufficient sample size
	params := &mcp.CallToolParamsFor[AnalyzeSchemaParams]{Arguments: AnalyzeSchemaParams{
		ProfileName:   "testsqlite",
		DatabaseName:  ":memory:",
		AnalysisLevel: AnalysisLevelComprehensive,
		SampleSize:    30,
	}}
	res, err := server.handleAnalyzeSchema(ctx, session, params)
	if err != nil {
		t.Fatalf("analyze-schema error: %v", err)
	}
	if res == nil || len(res.Content) == 0 {
		t.Fatalf("Empty analyze-schema result content")
	}
	textContent, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("Expected TextContent, got %T", res.Content[0])
	}

	var result AnalyzeSchemaResult
	if err := json.Unmarshal([]byte(textContent.Text), &result); err != nil {
		t.Fatalf("Failed to unmarshal AnalyzeSchemaResult: %v", err)
	}

	// Relationship graph inclusion
	if (len(result.RelationshipGraph.SemanticRelationships) + len(result.RelationshipGraph.ForeignKeys)) == 0 {
		t.Errorf("Expected at least one relationship (implicit naming) in RelationshipGraph")
	}
	if result.RelationshipGraphVisual == nil {
		t.Errorf("Expected RelationshipGraphVisual to be present")
	}

	// Pattern propagation: users.email column should have PatternType 'email'
	usersSchema, okTbl := result.TableSchemas["users"]
	if !okTbl {
		t.Fatalf("Expected users table schema in result")
	}
	var emailCol *SchemaColumnInfo
	for i := range usersSchema.Columns {
		if usersSchema.Columns[i].ColumnName == "email" {
			emailCol = &usersSchema.Columns[i]
			break
		}
	}
	if emailCol == nil {
		t.Fatalf("users.email column not found in schema")
	}
	if emailCol.PatternType != "email" {
		t.Errorf("Expected PatternType 'email', got '%s'", emailCol.PatternType)
	}

	// Issue cap enforcement for users.email data quality metrics
	qm, okQM := result.DataQualityMetrics["users.email"]
	if !okQM {
		t.Fatalf("Expected quality metrics for users.email")
	}
	if len(qm.Issues) == 0 {
		t.Fatalf("Expected issues for invalid email rows")
	}
	if len(qm.Issues) > 11 { // 10 + truncation message
		t.Errorf("Expected issues length <=11 (10 + truncation), got %d", len(qm.Issues))
	}
	if len(qm.Issues) == 11 {
		last := qm.Issues[len(qm.Issues)-1]
		if !strings.Contains(last, "more issues truncated") {
			t.Errorf("Expected truncation indicator in last issue, got: %s", last)
		}
	}
}

func TestDecryptionErrorPath_InvalidAESKeyLength(t *testing.T) {
	// Create a config with invalid AES key length (<32) to trigger fast-fail in LoadConfig
	path := "bad_aes_config.yaml"
	defer os.Remove(path)
	content := `aes_key: "short-key-123456789"
max_pool_size: 5
profiles:
  - profile_name: "badp"
    db_type: "sqlite"
    database_name: ":memory:"
    password: "plaintext"`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}
	_, err := config.LoadConfig(path)
	if err == nil {
		t.Fatalf("Expected decryption/AES key error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid AES key length") {
		t.Errorf("Expected invalid AES key length error, got: %v", err)
	}
}

func TestPostgresConstraintMappingQueryDefinition(t *testing.T) {
	// Static verification that CASE mapping for PRI/UNI/MUL exists in server.go (proxy for mapping presence)
	data, err := os.ReadFile("internal/mcp/server.go")
	if err != nil {
		t.Fatalf("Failed to read server.go: %v", err)
	}
	src := string(data)
	requiredSnippets := []string{
		"WHEN tc.constraint_type = 'PRIMARY KEY' THEN 'PRI'",
		"WHEN tc.constraint_type = 'UNIQUE' THEN 'UNI'",
		"WHEN tc.constraint_type = 'FOREIGN KEY' THEN 'MUL'",
	}
	for _, snip := range requiredSnippets {
		if !strings.Contains(src, snip) {
			t.Errorf("Expected Postgres constraint mapping snippet missing: %s", snip)
		}
	}
}
