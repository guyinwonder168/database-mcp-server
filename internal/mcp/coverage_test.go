//go:build cgo

package mcp

import (
	"context"
	"strings"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

// TestNewMCPServer tests the NewMCPServer constructor
func TestNewMCPServer(t *testing.T) {
	server := NewMCPServer()
	if server == nil {
		t.Fatal("NewMCPServer() returned nil")
	}
	if server.ConfigPath != "config.yaml" {
		t.Errorf("Expected ConfigPath to be 'config.yaml', got %q", server.ConfigPath)
	}
	if server.server == nil {
		t.Error("Expected server.server to be non-nil")
	}
}

// TestMCPServer_Server tests the Server() method
func TestMCPServer_Server(t *testing.T) {
	server := NewMCPServer()
	if server.Server() == nil {
		t.Error("Server() returned nil")
	}
}

// TestHandleMCPInfo tests the handleMCPInfo handler
func TestHandleMCPInfo(t *testing.T) {
	server := NewMCPServer()
	result, _, err := server.handleMCPInfo(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("handleMCPInfo failed: %v", err)
	}
	if result == nil {
		t.Fatal("handleMCPInfo returned nil result")
	}
	if len(result.Content) == 0 {
		t.Error("handleMCPInfo returned empty content")
	}
}

// TestAnalyzeNamingRelationships tests the analyzeNamingRelationships function
func TestAnalyzeNamingRelationships(t *testing.T) {
	server := &MCPServer{}

	// Test case 1: tables with foreign key naming pattern
	tables := map[string]TableInfo{
		"users": {
			Columns: []SchemaColumnInfo{
				{ColumnName: "id"},
				{ColumnName: "name"},
			},
		},
		"orders": {
			Columns: []SchemaColumnInfo{
				{ColumnName: "id"},
				{ColumnName: "users_id"}, // Pattern: targetTableName + "_id"
			},
		},
	}

	rels := server.analyzeNamingRelationships("orders", tables["orders"], "users", tables["users"])
	if rels == nil {
		t.Error("Expected relationship to be detected for users_id column")
	} else {
		if rels.RelationshipType != "implicit_naming" {
			t.Errorf("Expected relationship type 'implicit_naming', got %q", rels.RelationshipType)
		}
		// The confidence score for suffix pattern is 0.7, but exact match is 0.8
		// Since users_id matches users table name, it's an exact match
		if rels.ConfidenceScore != 0.8 {
			t.Errorf("Expected confidence score 0.8, got %f", rels.ConfidenceScore)
		}
	}

	// Test case 2: shared column names
	tables2 := map[string]TableInfo{
		"users": {
			Columns: []SchemaColumnInfo{
				{ColumnName: "id"},
				{ColumnName: "status"},
			},
		},
		"orders": {
			Columns: []SchemaColumnInfo{
				{ColumnName: "id"},
				{ColumnName: "status"}, // Shared column (not id)
			},
		},
	}

	rels2 := server.analyzeNamingRelationships("orders", tables2["orders"], "users", tables2["users"])
	if rels2 == nil {
		t.Error("Expected relationship to be detected for shared 'status' column")
	} else {
		if rels2.RelationshipType != "shared_column" {
			t.Errorf("Expected relationship type 'shared_column', got %q", rels2.RelationshipType)
		}
	}

	// Test case 3: exact match pattern (orders_id in users table)
	tables3 := map[string]TableInfo{
		"orders": {
			Columns: []SchemaColumnInfo{
				{ColumnName: "id"},
				{ColumnName: "total"},
			},
		},
		"users": {
			Columns: []SchemaColumnInfo{
				{ColumnName: "id"},
				{ColumnName: "orders_id"}, // Pattern: exact match orders + "_id"
			},
		},
	}

	rels3 := server.analyzeNamingRelationships("users", tables3["users"], "orders", tables3["orders"])
	if rels3 == nil {
		t.Error("Expected relationship to be detected for exact match 'orders_id' column")
	} else {
		if rels3.RelationshipType != "implicit_naming" {
			t.Errorf("Expected relationship type 'implicit_naming', got %q", rels3.RelationshipType)
		}
		if rels3.ConfidenceScore != 0.8 {
			t.Errorf("Expected confidence score 0.8, got %f", rels3.ConfidenceScore)
		}
	}

	// Test case 4: no relationship
	tables4 := map[string]TableInfo{
		"users": {
			Columns: []SchemaColumnInfo{
				{ColumnName: "id"},
				{ColumnName: "name"},
			},
		},
		"products": {
			Columns: []SchemaColumnInfo{
				{ColumnName: "id"},
				{ColumnName: "title"},
			},
		},
	}

	rels4 := server.analyzeNamingRelationships("users", tables4["users"], "products", tables4["products"])
	if rels4 != nil {
		t.Error("Expected no relationship to be detected")
	}
}

// TestBuildRelationshipGraph tests the buildRelationshipGraph function
func TestBuildRelationshipGraph(t *testing.T) {
	server := &MCPServer{}

	relGraph := RelationshipGraph{
		ForeignKeys: []ForeignKeyRelationship{
			{
				FromTable:        "orders",
				FromColumn:       "user_id",
				ToTable:          "users",
				ToColumn:         "id",
				RelationshipType: "many_to_one",
				SuggestedJoin:    "SELECT * FROM orders JOIN users ON orders.user_id = users.id",
			},
		},
		SemanticRelationships: []SemanticRelationship{
			{
				Tables:           []string{"orders", "products"},
				RelationshipType: "join_suggestion",
				ConnectionBasis:  "discover-joins",
				ConfidenceScore:  0.8,
				SuggestedJoin:    "SELECT * FROM orders JOIN products ON orders.product_id = products.id",
			},
		},
		SuggestedJoins: []string{
			"SELECT * FROM orders JOIN users ON orders.user_id = users.id",
			"SELECT * FROM orders JOIN products ON orders.product_id = products.id",
		},
	}

	graph := server.buildRelationshipGraph(relGraph)
	if graph == nil {
		t.Fatal("buildRelationshipGraph returned nil")
	}

	// Check nodes
	nodes, ok := graph["nodes"].(map[string]map[string]interface{})
	if !ok {
		t.Fatal("Expected 'nodes' to be present in graph")
	}
	if _, exists := nodes["orders"]; !exists {
		t.Error("Expected 'orders' to be in nodes")
	}
	if _, exists := nodes["users"]; !exists {
		t.Error("Expected 'users' to be in nodes")
	}
	if _, exists := nodes["products"]; !exists {
		t.Error("Expected 'products' to be in nodes")
	}

	// Check edges
	edges, ok := graph["edges"].([]map[string]interface{})
	if !ok {
		t.Fatal("Expected 'edges' to be present in graph")
	}
	if len(edges) != 2 {
		t.Errorf("Expected 2 edges, got %d", len(edges))
	}

	// Check suggested_joins
	suggestedJoins, ok := graph["suggested_joins"].([]string)
	if !ok {
		t.Fatal("Expected 'suggested_joins' to be present in graph")
	}
	if len(suggestedJoins) != 2 {
		t.Errorf("Expected 2 suggested joins, got %d", len(suggestedJoins))
	}
}

// TestSmartBuilderNoTableMatchResult tests the smartBuilderNoTableMatchResult function
func TestSmartBuilderNoTableMatchResult(t *testing.T) {
	tables := []string{"users", "orders", "products"}
	result := smartBuilderNoTableMatchResult(tables)
	if result == nil {
		t.Fatal("smartBuilderNoTableMatchResult returned nil")
	}
	if len(result.Content) == 0 {
		t.Error("Expected non-empty content")
	}
}

// TestSanitizeItems tests the sanitizeItems function more thoroughly
func TestSanitizeItems(t *testing.T) {
	// Test case 1: items: true (should become object schema)
	n1 := map[string]any{"items": true}
	sanitizeItems(n1)
	items, ok := n1["items"].(map[string]any)
	if !ok {
		t.Error("Expected items to be converted to map")
	} else if items["type"] != "string" {
		t.Errorf("Expected items type to be 'string', got %v", items["type"])
	}

	// Test case 2: items: false (should be deleted)
	n2 := map[string]any{"items": false}
	sanitizeItems(n2)
	if _, exists := n2["items"]; exists {
		t.Error("Expected items to be deleted when false")
	}

	// Test case 3: items is already an object (should not change)
	n3 := map[string]any{"items": map[string]any{"type": "number"}}
	sanitizeItems(n3)
	items3, ok := n3["items"].(map[string]any)
	if !ok || items3["type"] != "number" {
		t.Error("Expected items to remain unchanged when already an object")
	}

	// Test case 4: no items key (should remain unchanged)
	n4 := map[string]any{"type": "string"}
	sanitizeItems(n4)
	if _, exists := n4["items"]; exists {
		t.Error("Expected no items key to be added")
	}
}

// TestStringInSlice tests the stringInSlice function
func TestStringInSlice(t *testing.T) {
	slice := []string{"a", "b", "c"}

	// Test case 1: string exists
	if !stringInSlice(slice, "b") {
		t.Error("Expected stringInSlice to return true for 'b'")
	}

	// Test case 2: string does not exist
	if stringInSlice(slice, "d") {
		t.Error("Expected stringInSlice to return false for 'd'")
	}

	// Test case 3: empty slice
	if stringInSlice([]string{}, "a") {
		t.Error("Expected stringInSlice to return false for empty slice")
	}

	// Test case 4: empty string in slice
	if !stringInSlice([]string{"", "a"}, "") {
		t.Error("Expected stringInSlice to return true for empty string")
	}
}

// TestTableListQuery tests the tableListQuery function
func TestTableListQuery(t *testing.T) {
	// Test MySQL
	query, err := tableListQuery("mysql")
	if err != nil {
		t.Fatalf("tableListQuery failed for mysql: %v", err)
	}
	if query == "" {
		t.Error("Expected non-empty query for mysql")
	}

	// Test PostgreSQL
	query, err = tableListQuery("postgres")
	if err != nil {
		t.Fatalf("tableListQuery failed for postgres: %v", err)
	}
	if query == "" {
		t.Error("Expected non-empty query for postgres")
	}

	// Test SQLite
	query, err = tableListQuery("sqlite")
	if err != nil {
		t.Fatalf("tableListQuery failed for sqlite: %v", err)
	}
	if query == "" {
		t.Error("Expected non-empty query for sqlite")
	}

	// Test unsupported
	_, err = tableListQuery("oracle")
	if err == nil {
		t.Error("Expected error for unsupported db type")
	}
}

// TestTableInfoListQuery tests the tableInfoListQuery function which returns
// 3-column-aligned queries suitable for scanTableInfo. This is a regression
// test for BUG-001/002 where scanTableInfo expected 3 columns but the query
// returned only 2 (mysql/mariadb SHOW FULL TABLES).
func TestTableInfoListQuery(t *testing.T) {
	tests := []struct {
		name       string
		dbType     string
		wantPrefix string // query should start with this
		wantErr    bool
	}{
		{name: "mysql", dbType: "mysql", wantPrefix: "SELECT TABLE_SCHEMA, TABLE_NAME, TABLE_TYPE", wantErr: false},
		{name: "mariadb", dbType: "mariadb", wantPrefix: "SELECT TABLE_SCHEMA, TABLE_NAME, TABLE_TYPE", wantErr: false},
		{name: "postgres", dbType: "postgres", wantPrefix: "SELECT table_schema, table_name, table_type", wantErr: false},
		{name: "sqlite", dbType: "sqlite", wantPrefix: "SELECT '' AS schema, name, type", wantErr: false},
		{name: "unsupported", dbType: "oracle", wantPrefix: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, err := tableInfoListQuery(tt.dbType)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error for unsupported db type")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.HasPrefix(query, tt.wantPrefix) {
				t.Errorf("query for %s should start with %q, got %q", tt.dbType, tt.wantPrefix, query)
			}
		})
	}
}

// TestSmartBuilderColumnQuery tests the smartBuilderColumnQuery function
func TestSmartBuilderColumnQuery(t *testing.T) {
	// Test MySQL
	query, err := smartBuilderColumnQuery("mysql", "users")
	if err != nil {
		t.Fatalf("smartBuilderColumnQuery failed for mysql: %v", err)
	}
	if query == "" {
		t.Error("Expected non-empty query for mysql")
	}

	// Test PostgreSQL
	query, err = smartBuilderColumnQuery("postgres", "users")
	if err != nil {
		t.Fatalf("smartBuilderColumnQuery failed for postgres: %v", err)
	}
	if query == "" {
		t.Error("Expected non-empty query for postgres")
	}

	// Test SQLite
	query, err = smartBuilderColumnQuery("sqlite", "users")
	if err != nil {
		t.Fatalf("smartBuilderColumnQuery failed for sqlite: %v", err)
	}
	if query == "" {
		t.Error("Expected non-empty query for sqlite")
	}

	// Test unsupported
	_, err = smartBuilderColumnQuery("oracle", "users")
	if err == nil {
		t.Error("Expected error for unsupported db type")
	}
}

// TestTableNameFromSimpleSelect tests the tableNameFromSimpleSelect function
func TestTableNameFromSimpleSelect(t *testing.T) {
	tests := []struct {
		sql      string
		expected string
	}{
		{"SELECT * FROM users", "users"},
		{"  select   *   from   orders  ", "orders"},
		{"SELECT id, name FROM products WHERE id = 1", "products"},
		{"INSERT INTO users VALUES (1)", ""}, // Not a SELECT
		{"SELECT * FROM", ""},                // Malformed
		{"", ""},
	}

	for _, tt := range tests {
		result := tableNameFromSimpleSelect(tt.sql)
		if result != tt.expected {
			t.Errorf("tableNameFromSimpleSelect(%q) = %q, expected %q", tt.sql, result, tt.expected)
		}
	}
}

// TestBuildIDSet tests the buildIDSet function
func TestBuildIDSet(t *testing.T) {
	rows := []map[string]interface{}{
		{"id": 1, "name": "alice"},
		{"id": 2, "name": "bob"},
		{"name": "charlie"}, // No id field
	}

	idSet := buildIDSet(rows)
	if len(idSet) != 2 {
		t.Errorf("Expected 2 IDs in set, got %d", len(idSet))
	}

	// Check that IDs are in the set
	if _, exists := idSet[1]; !exists {
		t.Error("Expected id 1 to be in set")
	}
	if _, exists := idSet[2]; !exists {
		t.Error("Expected id 2 to be in set")
	}
}

// TestReferenceColumnsForTarget tests the referenceColumnsForTarget function
func TestReferenceColumnsForTarget(t *testing.T) {
	columns := []SchemaColumnInfo{
		{ColumnName: "id"},
		{ColumnName: "user_id"},
		{ColumnName: "order_id"},
		{ColumnName: "name"},
	}

	refCols := referenceColumnsForTarget(columns, "users")
	if len(refCols) != 2 {
		t.Errorf("Expected 2 reference columns, got %d", len(refCols))
	}

	// Check that user_id is included (matches users_id pattern)
	found := false
	for _, col := range refCols {
		if col == "user_id" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected 'user_id' to be in reference columns")
	}
}

// TestCountReferenceMatches tests the countReferenceMatches function
func TestCountReferenceMatches(t *testing.T) {
	sourceRows := []map[string]interface{}{
		{"user_id": 1, "order_id": 100},
		{"user_id": 2, "order_id": 200},
		{"user_id": 3, "order_id": nil},
	}

	refCols := []string{"user_id", "order_id"}
	targetIDs := map[interface{}]struct{}{
		1: {},
		2: {},
	}

	total, matches := countReferenceMatches(sourceRows, refCols, targetIDs)
	// The function counts all values including nil
	if total != 6 { // 3 rows * 2 columns = 6 total values
		t.Errorf("Expected total 6, got %d", total)
	}
	if matches != 2 {
		t.Errorf("Expected 2 matches, got %d", matches)
	}
}

// TestCorrelateDataValues tests the correlateDataValues function
func TestCorrelateDataValues(t *testing.T) {
	server := &MCPServer{}

	sampleData := map[string][]map[string]interface{}{
		"users": {
			{"id": 1, "name": "alice"},
			{"id": 2, "name": "bob"},
		},
		"orders": {
			{"id": 100, "user_id": 1},
			{"id": 101, "user_id": 2},
		},
	}

	tables := map[string]TableInfo{
		"users": {
			Columns: []SchemaColumnInfo{
				{ColumnName: "id"},
				{ColumnName: "name"},
			},
		},
		"orders": {
			Columns: []SchemaColumnInfo{
				{ColumnName: "id"},
				{ColumnName: "user_id"},
			},
		},
	}

	correlation := server.correlateDataValues("orders", tables["orders"], "users", sampleData)
	if correlation != 1.0 {
		t.Errorf("Expected correlation 1.0 (all match), got %f", correlation)
	}

	// Test with no matching IDs
	sampleData2 := map[string][]map[string]interface{}{
		"users": {
			{"id": 10, "name": "alice"},
		},
		"orders": {
			{"id": 100, "user_id": 1}, // No matching user
		},
	}

	correlation2 := server.correlateDataValues("orders", tables["orders"], "users", sampleData2)
	if correlation2 != 0.0 {
		t.Errorf("Expected correlation 0.0 (no matches), got %f", correlation2)
	}
}

// TestDetectImplicitRelationships tests the detectImplicitRelationships function
func TestDetectImplicitRelationships(t *testing.T) {
	server := &MCPServer{}

	tables := map[string]TableInfo{
		"users": {
			Columns: []SchemaColumnInfo{
				{ColumnName: "id"},
				{ColumnName: "name"},
			},
		},
		"orders": {
			Columns: []SchemaColumnInfo{
				{ColumnName: "id"},
				{ColumnName: "users_id"}, // FK pattern
			},
		},
		"products": {
			Columns: []SchemaColumnInfo{
				{ColumnName: "id"},
				{ColumnName: "name"},
			},
		},
	}

	relationships := server.detectImplicitRelationships(tables)
	if len(relationships) == 0 {
		t.Error("Expected some implicit relationships to be detected")
	}

	// Should detect relationship between orders and users
	found := false
	for _, rel := range relationships {
		for _, table := range rel.Tables {
			if table == "orders" || table == "users" {
				found = true
				break
			}
		}
	}
	if !found {
		t.Error("Expected relationship between orders and users")
	}
}

// TestUpdateForeignKeyPatternCounts tests the updateForeignKeyPatternCounts function
func TestUpdateForeignKeyPatternCounts(t *testing.T) {
	// Test with _id suffix
	suffix, prefix, total := updateForeignKeyPatternCounts("user_id", 0, 0, 0)
	if suffix != 1 {
		t.Errorf("Expected suffix count 1, got %d", suffix)
	}
	if prefix != 0 {
		t.Errorf("Expected prefix count 0, got %d", prefix)
	}
	if total != 1 {
		t.Errorf("Expected total count 1, got %d", total)
	}

	// Test with id_ prefix
	suffix2, prefix2, total2 := updateForeignKeyPatternCounts("id_user", 0, 0, 0)
	if suffix2 != 0 {
		t.Errorf("Expected suffix count 0, got %d", suffix2)
	}
	if prefix2 != 1 {
		t.Errorf("Expected prefix count 1, got %d", prefix2)
	}
	if total2 != 1 {
		t.Errorf("Expected total count 1, got %d", total2)
	}

	// Test with no pattern
	suffix3, prefix3, total3 := updateForeignKeyPatternCounts("name", 5, 3, 10)
	if suffix3 != 5 {
		t.Errorf("Expected suffix count to remain 5, got %d", suffix3)
	}
	if prefix3 != 3 {
		t.Errorf("Expected prefix count to remain 3, got %d", prefix3)
	}
	if total3 != 10 {
		t.Errorf("Expected total count to remain 10, got %d", total3)
	}
}

// TestNamingValueStringSlice tests the namingValueStringSlice function
func TestNamingValueStringSlice(t *testing.T) {
	// Test with valid string slice
	data := map[string]interface{}{
		"items": []interface{}{"a", "b", "c"},
	}
	result := namingValueStringSlice(data, "items")
	if len(result) != 3 {
		t.Errorf("Expected 3 items, got %d", len(result))
	}

	// Test with non-string values (converted to string)
	data2 := map[string]interface{}{
		"items": []interface{}{"a", 123, "c"},
	}
	result2 := namingValueStringSlice(data2, "items")
	if len(result2) != 3 {
		t.Errorf("Expected 3 items (non-strings converted), got %d", len(result2))
	}
	// Check that 123 was converted to "123"
	if result2[1] != "123" {
		t.Errorf("Expected '123', got %q", result2[1])
	}

	// Test with missing key
	data3 := map[string]interface{}{}
	result3 := namingValueStringSlice(data3, "missing")
	if len(result3) != 0 {
		t.Errorf("Expected empty slice for missing key, got %d", len(result3))
	}
}

// TestGetDefaultSchema tests the GetDefaultSchema function
func TestGetDefaultSchema(t *testing.T) {
	// Create a mock database connection
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("failed to get connection: %v", err)
	}
	defer conn.Close()

	// Test case 1: current_schema() returns a valid schema
	mock.ExpectQuery("SELECT current_schema\\(\\)").WillReturnRows(sqlmock.NewRows([]string{"current_schema"}).AddRow("public"))

	schema, err := GetDefaultSchema(ctx, conn)
	if err != nil {
		t.Fatalf("GetDefaultSchema failed: %v", err)
	}
	if schema != "public" {
		t.Errorf("Expected schema 'public', got %q", schema)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// TestHandleGetToolHelp tests the handleGetToolHelp function
func TestHandleGetToolHelp(t *testing.T) {
	server := NewMCPServer()

	// Test case 1: empty tool name
	_, _, err := server.handleGetToolHelp(context.Background(), nil, GetToolHelpParams{})
	if err == nil {
		t.Error("Expected error for empty tool_name")
	}

	// Test case 2: invalid topic
	_, _, err = server.handleGetToolHelp(context.Background(), nil, GetToolHelpParams{ToolName: "list-tools", Topic: "invalid_topic"})
	if err == nil {
		t.Error("Expected error for invalid topic")
	}

	// Test case 3: tool not found
	result, _, err := server.handleGetToolHelp(context.Background(), nil, GetToolHelpParams{ToolName: "nonexistent-tool"})
	if err != nil {
		t.Fatalf("handleGetToolHelp failed: %v", err)
	}
	if result == nil {
		t.Fatal("handleGetToolHelp returned nil result")
	}

	// Test case 4: valid tool with all topic
	result, _, err = server.handleGetToolHelp(context.Background(), nil, GetToolHelpParams{ToolName: "list-tools", Topic: "all"})
	if err != nil {
		t.Fatalf("handleGetToolHelp failed: %v", err)
	}
	if result == nil {
		t.Fatal("handleGetToolHelp returned nil result")
	}

	// Test case 5: valid tool with summary topic
	result, _, err = server.handleGetToolHelp(context.Background(), nil, GetToolHelpParams{ToolName: "list-tools", Topic: "summary"})
	if err != nil {
		t.Fatalf("handleGetToolHelp failed: %v", err)
	}
	if result == nil {
		t.Fatal("handleGetToolHelp returned nil result")
	}

	// Test case 6: get-search-path tool (added in this PR)
	result, _, err = server.handleGetToolHelp(context.Background(), nil, GetToolHelpParams{ToolName: "get-search-path", Topic: "all"})
	if err != nil {
		t.Fatalf("handleGetToolHelp failed for get-search-path: %v", err)
	}
	if result == nil {
		t.Fatal("handleGetToolHelp returned nil result for get-search-path")
	}
}

// TestIsSupportedToolHelpTopic tests the isSupportedToolHelpTopic function
func TestIsSupportedToolHelpTopic(t *testing.T) {
	// Test empty topic (should return true - defaults to "all")
	if !isSupportedToolHelpTopic("") {
		t.Error("Expected empty topic to be supported")
	}

	// Test valid topics
	validTopics := []string{"summary", "minimal_example", "advanced_example", "errors", "all"}
	for _, topic := range validTopics {
		if !isSupportedToolHelpTopic(topic) {
			t.Errorf("Expected topic %q to be supported", topic)
		}
	}

	// Test invalid topic
	if isSupportedToolHelpTopic("invalid_topic") {
		t.Error("Expected invalid_topic to not be supported")
	}
}

// TestTopicFilteredToolHelpResult tests the topicFilteredToolHelpResult function
func TestTopicFilteredToolHelpResult(t *testing.T) {
	entry := toolHelpEntry{
		Summary:         "Test summary",
		MinimalExample:  map[string]any{"key": "value"},
		AdvancedExample: map[string]any{"key2": "value2"},
		CommonErrors:    []ToolHelpError{{Error: "err1", Cause: "cause1", Fix: "fix1"}},
		Notes:           []string{"note1", "note2"},
	}

	// Test empty topic (should default to all)
	result := topicFilteredToolHelpResult("test-tool", "", entry)
	if !result.Found {
		t.Error("Expected Found to be true")
	}
	if len(result.Topics) == 0 {
		t.Error("Expected Topics to be populated")
	}

	// Test summary topic
	result = topicFilteredToolHelpResult("test-tool", "summary", entry)
	if result.Summary != "Test summary" {
		t.Errorf("Expected summary, got %q", result.Summary)
	}

	// Test minimal_example topic
	result = topicFilteredToolHelpResult("test-tool", "minimal_example", entry)
	if result.MinimalExample == nil {
		t.Error("Expected MinimalExample to be set")
	}

	// Test advanced_example topic
	result = topicFilteredToolHelpResult("test-tool", "advanced_example", entry)
	if result.AdvancedExample == nil {
		t.Error("Expected AdvancedExample to be set")
	}

	// Test errors topic
	result = topicFilteredToolHelpResult("test-tool", "errors", entry)
	if len(result.CommonErrors) == 0 {
		t.Error("Expected CommonErrors to be set")
	}

	// Test all topic
	result = topicFilteredToolHelpResult("test-tool", "all", entry)
	if result.Summary != "Test summary" {
		t.Errorf("Expected all topics to include summary, got %q", result.Summary)
	}
	if result.MinimalExample == nil {
		t.Error("Expected all topics to include minimal_example")
	}
	if result.AdvancedExample == nil {
		t.Error("Expected all topics to include advanced_example")
	}
	if result.CommonErrors == nil {
		t.Error("Expected all topics to include common_errors")
	}
	if result.Notes == nil {
		t.Error("Expected all topics to include notes")
	}
}

// TestMarshalToolHelpResult tests the marshalToolHelpResult function
func TestMarshalToolHelpResult(t *testing.T) {
	result := GetToolHelpResult{
		ToolName: "test-tool",
		Found:    true,
		Summary:  "Test summary",
		Topics:   []string{"summary", "all"},
	}

	// Test successful marshaling
	mcpResult, output, err := marshalToolHelpResult(result)
	if err != nil {
		t.Fatalf("marshalToolHelpResult failed: %v", err)
	}
	if mcpResult == nil {
		t.Fatal("marshalToolHelpResult returned nil result")
	}
	if output != nil {
		t.Error("Expected nil output")
	}
	if len(mcpResult.Content) == 0 {
		t.Error("Expected non-empty content")
	}
}
