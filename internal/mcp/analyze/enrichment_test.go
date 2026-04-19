package analyze

import (
	"fmt"
	"strings"
	"testing"
)

// --- computeDomainSignals Tests ---

func TestComputeDomainSignals_ECommerce(t *testing.T) {
	tables := []string{"products", "orders", "cart_items", "customers", "payments", "inventory"}
	signals := ComputeDomainSignals(tables)
	// Should produce prefix frequencies for underscore-delimited segments
	if len(signals) == 0 {
		t.Fatal("expected non-empty signals")
	}
	// Single-word tables should appear as-is
	if signals["products"] != 1 {
		t.Errorf("expected products: 1, got %v", signals["products"])
	}
	if signals["orders"] != 1 {
		t.Errorf("expected orders: 1, got %v", signals["orders"])
	}
	// Compound table: "cart_items" → prefix "cart"
	if signals["cart"] != 1 {
		t.Errorf("expected cart: 1, got %v", signals["cart"])
	}
}

func TestComputeDomainSignals_CaseInsensitive(t *testing.T) {
	tables := []string{"PRODUCTS", "Orders", "CART_ITEMS"}
	signals := ComputeDomainSignals(tables)
	// All keys should be lowercase
	for key := range signals {
		if key != strings.ToLower(key) {
			t.Errorf("expected lowercase key, got %q", key)
		}
	}
	if signals["products"] != 1 {
		t.Errorf("expected products: 1, got %v", signals["products"])
	}
	if signals["cart"] != 1 {
		t.Errorf("expected cart: 1, got %v", signals["cart"])
	}
}

func TestComputeDomainSignals_Empty(t *testing.T) {
	signals := ComputeDomainSignals([]string{})
	if len(signals) == 0 {
		t.Fatal("expected non-empty signals even for empty input")
	}
	if _, ok := signals["unknown"]; !ok {
		t.Error("expected 'unknown' signal for empty input")
	}
}

func TestComputeDomainSignals_Grouping(t *testing.T) {
	// Tables with same prefix should be counted together
	tables := []string{"order_items", "order_status", "order_history"}
	signals := ComputeDomainSignals(tables)
	if signals["order"] != 3 {
		t.Errorf("expected order: 3, got %v", signals["order"])
	}
}

func TestComputeDomainSignals_SingleWord(t *testing.T) {
	tables := []string{"users", "products", "orders"}
	signals := ComputeDomainSignals(tables)
	if len(signals) != 3 {
		t.Errorf("expected 3 signals, got %d", len(signals))
	}
	if signals["users"] != 1 {
		t.Errorf("expected users: 1, got %v", signals["users"])
	}
}

// --- IdentifyEntityTypes Tests ---

func TestIdentifyEntityTypes_LogTables(t *testing.T) {
	types := IdentifyEntityTypes([]string{"audit_log", "system_logs"})
	if len(types) != 2 {
		t.Fatalf("expected 2 types, got %d", len(types))
	}
	for _, et := range types {
		if et != "log" {
			t.Errorf("expected 'log', got %q", et)
		}
	}
}

func TestIdentifyEntityTypes_LookupTables(t *testing.T) {
	types := IdentifyEntityTypes([]string{"status_lookup", "order_type", "user_status"})
	if len(types) != 3 {
		t.Fatalf("expected 3 types, got %d", len(types))
	}
	for _, et := range types {
		if et != "lookup" {
			t.Errorf("expected 'lookup', got %q", et)
		}
	}
}

func TestIdentifyEntityTypes_TransactionalTables(t *testing.T) {
	types := IdentifyEntityTypes([]string{"transactions", "orders", "invoices"})
	if len(types) != 3 {
		t.Fatalf("expected 3 types, got %d", len(types))
	}
	for _, et := range types {
		if et != "transactional" {
			t.Errorf("expected 'transactional', got %q", et)
		}
	}
}

func TestIdentifyEntityTypes_MasterDataTables(t *testing.T) {
	types := IdentifyEntityTypes([]string{"users", "products", "customers"})
	if len(types) != 3 {
		t.Fatalf("expected 3 types, got %d", len(types))
	}
	for _, et := range types {
		if et != "master_data" {
			t.Errorf("expected 'master_data', got %q", et)
		}
	}
}

func TestIdentifyEntityTypes_OtherTables(t *testing.T) {
	types := IdentifyEntityTypes([]string{"settings", "config"})
	if len(types) != 2 {
		t.Fatalf("expected 2 types, got %d", len(types))
	}
	for _, et := range types {
		if et != "other" {
			t.Errorf("expected 'other', got %q", et)
		}
	}
}

func TestIdentifyEntityTypes_Mixed(t *testing.T) {
	types := IdentifyEntityTypes([]string{"audit_log", "users", "transactions", "settings"})
	expected := []string{"log", "master_data", "transactional", "other"}
	if len(types) != len(expected) {
		t.Fatalf("expected %d types, got %d", len(expected), len(types))
	}
	for i, et := range expected {
		if types[i] != et {
			t.Errorf("index %d: expected %q, got %q", i, et, types[i])
		}
	}
}

func TestIdentifyEntityTypes_Empty(t *testing.T) {
	types := IdentifyEntityTypes([]string{})
	if len(types) != 0 {
		t.Errorf("expected 0 types for empty input, got %d", len(types))
	}
}

// --- Naming Accumulator Helper Tests ---

func TestRecordCaseType_SnakeCase(t *testing.T) {
	counts := map[string]int{"snake_case": 0, "camelCase": 0, "PascalCase": 0}
	recordCaseType(counts, "user_name")
	recordCaseType(counts, "order_id")
	if counts["snake_case"] != 2 {
		t.Errorf("expected snake_case=2, got %d", counts["snake_case"])
	}
}

func TestRecordCaseType_CamelCase(t *testing.T) {
	counts := map[string]int{"snake_case": 0, "camelCase": 0, "PascalCase": 0}
	recordCaseType(counts, "userName")
	recordCaseType(counts, "orderId")
	if counts["camelCase"] != 2 {
		t.Errorf("expected camelCase=2, got %d", counts["camelCase"])
	}
}

func TestRecordCaseType_PascalCase(t *testing.T) {
	counts := map[string]int{"snake_case": 0, "camelCase": 0, "PascalCase": 0}
	recordCaseType(counts, "UserName")
	recordCaseType(counts, "OrderId")
	if counts["PascalCase"] != 2 {
		t.Errorf("expected PascalCase=2, got %d", counts["PascalCase"])
	}
}

func TestDominantCase_SnakeCase(t *testing.T) {
	counts := map[string]int{"snake_case": 10, "camelCase": 2, "PascalCase": 1}
	mainCase, consistency := dominantCase(counts)
	if mainCase != "snake_case" {
		t.Errorf("expected 'snake_case', got %q", mainCase)
	}
	expectedConsistency := 10.0 / 13.0
	if consistency < expectedConsistency-0.01 || consistency > expectedConsistency+0.01 {
		t.Errorf("expected consistency ~%.4f, got %f", expectedConsistency, consistency)
	}
}

func TestDominantCase_Empty(t *testing.T) {
	counts := map[string]int{"snake_case": 0, "camelCase": 0, "PascalCase": 0}
	mainCase, consistency := dominantCase(counts)
	if mainCase != "unknown" {
		t.Errorf("expected 'unknown', got %q", mainCase)
	}
	if consistency != 0.0 {
		t.Errorf("expected 0.0, got %f", consistency)
	}
}

func TestClassifyForeignKeyPattern_Suffix(t *testing.T) {
	result := ClassifyForeignKeyPattern(5, 0, 5)
	if result != "suffix" {
		t.Errorf("expected 'suffix', got %q", result)
	}
}

func TestClassifyForeignKeyPattern_Prefix(t *testing.T) {
	result := ClassifyForeignKeyPattern(0, 3, 3)
	if result != "prefix" {
		t.Errorf("expected 'prefix', got %q", result)
	}
}

func TestClassifyForeignKeyPattern_Mixed(t *testing.T) {
	result := ClassifyForeignKeyPattern(2, 3, 5)
	if result != "mixed" {
		t.Errorf("expected 'mixed', got %q", result)
	}
}

func TestClassifyForeignKeyPattern_None(t *testing.T) {
	result := ClassifyForeignKeyPattern(0, 0, 0)
	if result != "none" {
		t.Errorf("expected 'none', got %q", result)
	}
}

func TestIsTimestampColumn(t *testing.T) {
	tests := []struct {
		name     string
		expected bool
	}{
		{"created_at", true},
		{"updated_at", true},
		{"event_timestamp", true},
		{"date_created", true},
		{"last_modified", true},
		{"username", false},
		{"email", false},
		{"created", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isTimestampColumn(tt.name)
			if result != tt.expected {
				t.Errorf("isTimestampColumn(%q) = %v, want %v", tt.name, result, tt.expected)
			}
		})
	}
}

func TestUpdateForeignKeyPatternCounts_Suffix(t *testing.T) {
	suffix, prefix, total := UpdateForeignKeyPatternCounts("user_id", 0, 0, 0)
	if suffix != 1 || prefix != 0 || total != 1 {
		t.Errorf("expected (1,0,1), got (%d,%d,%d)", suffix, prefix, total)
	}
}

func TestUpdateForeignKeyPatternCounts_Prefix(t *testing.T) {
	suffix, prefix, total := UpdateForeignKeyPatternCounts("id_user", 0, 0, 0)
	if suffix != 0 || prefix != 1 || total != 1 {
		t.Errorf("expected (0,1,1), got (%d,%d,%d)", suffix, prefix, total)
	}
}

func TestUpdateForeignKeyPatternCounts_NoMatch(t *testing.T) {
	suffix, prefix, total := UpdateForeignKeyPatternCounts("username", 2, 3, 5)
	if suffix != 2 || prefix != 3 || total != 5 {
		t.Errorf("expected (2,3,5), got (%d,%d,%d)", suffix, prefix, total)
	}
}

func TestRecordPrefixAndSuffix(t *testing.T) {
	prefixes := make(map[string]int)
	suffixes := make(map[string]int)
	RecordPrefixAndSuffix(prefixes, suffixes, "user_name")
	if prefixes["user"] != 1 {
		t.Errorf("expected prefix 'user'=1, got %d", prefixes["user"])
	}
	if suffixes["name"] != 1 {
		t.Errorf("expected suffix 'name'=1, got %d", suffixes["name"])
	}
}

func TestRecordPrefixAndSuffix_NoUnderscore(t *testing.T) {
	prefixes := make(map[string]int)
	suffixes := make(map[string]int)
	RecordPrefixAndSuffix(prefixes, suffixes, "username")
	if len(prefixes) != 0 || len(suffixes) != 0 {
		t.Error("expected no prefixes/suffixes for single-word name")
	}
}

// --- AnalyzeNamingConventions Tests ---

func TestAnalyzeNamingConventions_SnakeCase(t *testing.T) {
	tables := []TableInfo{
		{Columns: []SchemaColumnInfo{
			{ColumnName: "user_id"},
			{ColumnName: "user_name"},
			{ColumnName: "created_at"},
		}},
		{Columns: []SchemaColumnInfo{
			{ColumnName: "order_id"},
			{ColumnName: "user_id"},
			{ColumnName: "total_amount"},
		}},
	}
	result := AnalyzeNamingConventions(tables)
	mainCase := result["main_case"].(string)
	if mainCase != "snake_case" {
		t.Errorf("expected main_case 'snake_case', got %q", mainCase)
	}
	consistency := result["consistency"].(float64)
	if consistency <= 0 {
		t.Errorf("expected consistency > 0, got %f", consistency)
	}
	fkPattern := result["fk_pattern"].(string)
	if fkPattern != "suffix" {
		t.Errorf("expected fk_pattern 'suffix', got %q", fkPattern)
	}
}

func TestAnalyzeNamingConventions_Empty(t *testing.T) {
	result := AnalyzeNamingConventions([]TableInfo{})
	mainCase := result["main_case"].(string)
	if mainCase != "unknown" {
		t.Errorf("expected main_case 'unknown', got %q", mainCase)
	}
	fkPattern := result["fk_pattern"].(string)
	if fkPattern != "none" {
		t.Errorf("expected fk_pattern 'none', got %q", fkPattern)
	}
}

func TestAnalyzeNamingConventions_TimestampCols(t *testing.T) {
	tables := []TableInfo{
		{Columns: []SchemaColumnInfo{
			{ColumnName: "id"},
			{ColumnName: "created_at"},
			{ColumnName: "updated_at"},
		}},
	}
	result := AnalyzeNamingConventions(tables)
	timestampCols := result["timestampCols"].([]string)
	if len(timestampCols) != 2 {
		t.Errorf("expected 2 timestamp columns, got %d", len(timestampCols))
	}
}

// --- GenerateBusinessDescription Tests ---

func TestGenerateBusinessDescription_WithSignals(t *testing.T) {
	signals := map[string]float64{
		"order":    3,
		"product":  2,
		"customer": 1,
	}
	desc := GenerateBusinessDescription("order", 3, []string{"master_data", "transactional"}, signals)
	if desc == "" {
		t.Fatal("expected non-empty description")
	}
	// Should mention the top signal
	if !containsStr(desc, "order") {
		t.Errorf("expected description to mention 'order', got %q", desc)
	}
	// Should mention entity breakdown
	if !containsStr(desc, "master_data") {
		t.Errorf("expected description to mention entity type, got %q", desc)
	}
}

func TestGenerateBusinessDescription_EmptySignals(t *testing.T) {
	desc := GenerateBusinessDescription("", 0, []string{"other"}, nil)
	if desc == "" {
		t.Fatal("expected non-empty description")
	}
	// Should still describe entities
	if !containsStr(desc, "other") {
		t.Errorf("expected description to mention 'other' entity, got %q", desc)
	}
}

func TestGenerateBusinessDescription_MultipleSignals(t *testing.T) {
	signals := map[string]float64{
		"call":      5,
		"broadcast": 3,
		"sip":       2,
	}
	desc := GenerateBusinessDescription("call", 5, []string{"master_data", "log"}, signals)
	if desc == "" {
		t.Fatal("expected non-empty description")
	}
	// Should include multiple signals
	if !containsStr(desc, "call") {
		t.Errorf("expected description to mention 'call', got %q", desc)
	}
	if !containsStr(desc, "broadcast") {
		t.Errorf("expected description to mention 'broadcast', got %q", desc)
	}
}

// --- InferBusinessContext Integration Tests ---

func TestInferBusinessContext_ECommerce(t *testing.T) {
	tableSchemas := map[string]TableInfo{
		"products": {
			Columns: []SchemaColumnInfo{
				{ColumnName: "id"},
				{ColumnName: "product_name"},
				{ColumnName: "price"},
				{ColumnName: "created_at"},
			},
		},
		"orders": {
			Columns: []SchemaColumnInfo{
				{ColumnName: "id"},
				{ColumnName: "product_id"},
				{ColumnName: "customer_id"},
				{ColumnName: "total_amount"},
				{ColumnName: "created_at"},
			},
		},
		"customers": {
			Columns: []SchemaColumnInfo{
				{ColumnName: "id"},
				{ColumnName: "customer_name"},
				{ColumnName: "email"},
			},
		},
	}

	ctx := InferBusinessContext(tableSchemas)

	if ctx == nil {
		t.Fatal("expected non-nil BusinessContext")
	}

	// DomainIndicators should now contain naming prefix frequencies
	// Single-word tables: "products", "orders", "customers" → each count 1
	if ctx.DomainIndicators["products"] != 1 {
		t.Errorf("expected products: 1, got %v", ctx.DomainIndicators["products"])
	}
	if ctx.DomainIndicators["orders"] != 1 {
		t.Errorf("expected orders: 1, got %v", ctx.DomainIndicators["orders"])
	}
	if ctx.DomainIndicators["customers"] != 1 {
		t.Errorf("expected customers: 1, got %v", ctx.DomainIndicators["customers"])
	}

	// Naming should be snake_case
	if ctx.NamingConventions.Pattern != "snake_case" {
		t.Errorf("expected pattern 'snake_case', got %q", ctx.NamingConventions.Pattern)
	}

	// Should have central entities
	if len(ctx.EntityRelationships.CentralEntities) == 0 {
		t.Error("expected non-empty central entities")
	}

	// Should have audit columns
	if len(ctx.NamingConventions.AuditColumns) == 0 {
		t.Error("expected audit columns (created_at)")
	}
}

func TestInferBusinessContext_Empty(t *testing.T) {
	ctx := InferBusinessContext(map[string]TableInfo{})
	if ctx == nil {
		t.Fatal("expected non-nil BusinessContext")
	}
	// Empty input should produce "unknown" signal
	if _, ok := ctx.DomainIndicators["unknown"]; !ok {
		t.Error("expected 'unknown' signal for empty input")
	}
}

func TestInferBusinessContext_NilInput(t *testing.T) {
	ctx := InferBusinessContext(nil)
	if ctx == nil {
		t.Fatal("expected non-nil BusinessContext for nil input")
	}
}

// Helper function for string containment checks.
func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStrImpl(s, substr))
}

func containsStrImpl(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// --- Data Pattern Recognition Tests (moved from server.go, converted to pure functions) ---

// --- SemanticTypeRegexes Tests ---

func TestSemanticTypeRegexes_ReturnsExpectedPatterns(t *testing.T) {
	regexes := SemanticTypeRegexes()
	expectedKeys := []string{"email", "phone", "url", "id", "uuid", "date", "currency"}
	for _, key := range expectedKeys {
		if _, ok := regexes[key]; !ok {
			t.Errorf("expected key %q in regexes", key)
		}
	}
}

// --- DetectDataType Tests ---

func TestDetectDataType_Email(t *testing.T) {
	values := []interface{}{"alice@example.com", "bob@test.org", "user@domain.net"}
	result := DetectDataType(values)
	if result != "email" {
		t.Errorf("expected 'email', got %q", result)
	}
}

func TestDetectDataType_Phone(t *testing.T) {
	values := []interface{}{"+1234567890", "+9876543210", "+1112223333"}
	result := DetectDataType(values)
	if result != "phone" {
		t.Errorf("expected 'phone', got %q", result)
	}
}

func TestDetectDataType_UUID(t *testing.T) {
	values := []interface{}{
		"550e8400-e29b-41d4-a716-446655440000",
		"6ba7b810-9dad-11d1-80b4-00c04fd430c8",
	}
	result := DetectDataType(values)
	if result != "uuid" {
		t.Errorf("expected 'uuid', got %q", result)
	}
}

func TestDetectDataType_Date(t *testing.T) {
	values := []interface{}{"2025-01-15", "2025-02-20", "2024-12-31"}
	result := DetectDataType(values)
	if result != "date" {
		t.Errorf("expected 'date', got %q", result)
	}
}

func TestDetectDataType_Unknown(t *testing.T) {
	values := []interface{}{"foo", "bar", "baz"}
	result := DetectDataType(values)
	if result != "unknown" {
		t.Errorf("expected 'unknown', got %q", result)
	}
}

func TestDetectDataType_Empty(t *testing.T) {
	result := DetectDataType([]interface{}{})
	if result != "unknown" {
		t.Errorf("expected 'unknown' for empty input, got %q", result)
	}
}

func TestDetectDataType_NilValues(t *testing.T) {
	values := []interface{}{nil, nil, nil}
	result := DetectDataType(values)
	if result != "unknown" {
		t.Errorf("expected 'unknown' for all-nil input, got %q", result)
	}
}

func TestDetectDataType_MixedBelowThreshold(t *testing.T) {
	// Mix of emails and random strings — neither should exceed 50%
	values := []interface{}{"a@b.com", "foo", "bar", "baz"}
	result := DetectDataType(values)
	// Only 1/4 = 25% match email, below 50% threshold
	if result != "unknown" {
		t.Errorf("expected 'unknown' (below threshold), got %q", result)
	}
}

// --- AnalyzeValueDistribution Tests ---

func TestAnalyzeValueDistribution_Constant(t *testing.T) {
	values := []interface{}{"same", "same", "same"}
	stats := AnalyzeValueDistribution(values)
	if stats["distribution"] != "constant" {
		t.Errorf("expected 'constant', got %q", stats["distribution"])
	}
	if stats["unique_count"] != 1 {
		t.Errorf("expected unique_count=1, got %v", stats["unique_count"])
	}
}

func TestAnalyzeValueDistribution_Categorical(t *testing.T) {
	// 10 values, 3 unique → 3 < 10/2 = 5 → categorical
	values := []interface{}{"a", "a", "a", "b", "b", "c", "c", "a", "b", "c"}
	stats := AnalyzeValueDistribution(values)
	if stats["distribution"] != "categorical" {
		t.Errorf("expected 'categorical', got %q", stats["distribution"])
	}
}

func TestAnalyzeValueDistribution_Variable(t *testing.T) {
	// All unique → variable
	values := []interface{}{"a", "b", "c", "d", "e"}
	stats := AnalyzeValueDistribution(values)
	if stats["distribution"] != "variable" {
		t.Errorf("expected 'variable', got %q", stats["distribution"])
	}
}

func TestAnalyzeValueDistribution_NullCount(t *testing.T) {
	values := []interface{}{"a", nil, "b", nil, nil}
	stats := AnalyzeValueDistribution(values)
	if stats["null_count"] != 3 {
		t.Errorf("expected null_count=3, got %v", stats["null_count"])
	}
}

func TestAnalyzeValueDistribution_MostCommon(t *testing.T) {
	values := []interface{}{"a", "a", "a", "b", "b", "c"}
	stats := AnalyzeValueDistribution(values)
	common, ok := stats["most_common"].([]string)
	if !ok {
		t.Fatalf("expected most_common to be []string")
	}
	if len(common) == 0 || common[0] != "a" {
		t.Errorf("expected 'a' as most common, got %v", common)
	}
}

func TestAnalyzeValueDistribution_Empty(t *testing.T) {
	stats := AnalyzeValueDistribution([]interface{}{})
	if stats["distribution"] != "variable" {
		// 0 unique, 0 values → 0 < 0/2 is false → variable
		t.Errorf("expected 'variable' for empty, got %q", stats["distribution"])
	}
}

// --- ratio Tests ---

func TestRatio_Normal(t *testing.T) {
	result := ratio(3.0, 10.0)
	if result != 0.3 {
		t.Errorf("expected 0.3, got %f", result)
	}
}

func TestRatio_ZeroDenominator(t *testing.T) {
	result := ratio(5.0, 0.0)
	if result != 0 {
		t.Errorf("expected 0 for zero denominator, got %f", result)
	}
}

func TestRatio_One(t *testing.T) {
	result := ratio(7.0, 7.0)
	if result != 1.0 {
		t.Errorf("expected 1.0, got %f", result)
	}
}

// --- toFloat64 Tests ---

func TestToFloat64_Int(t *testing.T) {
	v, err := toFloat64(42)
	if err != nil || v != 42.0 {
		t.Errorf("expected 42.0, got %v, err=%v", v, err)
	}
}

func TestToFloat64_Int32(t *testing.T) {
	v, err := toFloat64(int32(100))
	if err != nil || v != 100.0 {
		t.Errorf("expected 100.0, got %v, err=%v", v, err)
	}
}

func TestToFloat64_Int64(t *testing.T) {
	v, err := toFloat64(int64(999))
	if err != nil || v != 999.0 {
		t.Errorf("expected 999.0, got %v, err=%v", v, err)
	}
}

func TestToFloat64_Float32(t *testing.T) {
	v, err := toFloat64(float32(3.14))
	if err != nil || v < 3.13 || v > 3.15 {
		t.Errorf("expected ~3.14, got %v, err=%v", v, err)
	}
}

func TestToFloat64_Float64(t *testing.T) {
	v, err := toFloat64(2.718)
	if err != nil || v != 2.718 {
		t.Errorf("expected 2.718, got %v, err=%v", v, err)
	}
}

func TestToFloat64_String(t *testing.T) {
	v, err := toFloat64("123.45")
	if err != nil || v != 123.45 {
		t.Errorf("expected 123.45, got %v, err=%v", v, err)
	}
}

func TestToFloat64_InvalidString(t *testing.T) {
	_, err := toFloat64("not-a-number")
	if err == nil {
		t.Error("expected error for invalid string")
	}
}

func TestToFloat64_UnsupportedType(t *testing.T) {
	_, err := toFloat64([]byte("test"))
	if err == nil {
		t.Error("expected error for unsupported type")
	}
}

// --- countDecimalPlaces Tests ---

func TestCountDecimalPlaces_WithDecimals(t *testing.T) {
	if countDecimalPlaces("1.23") != 2 {
		t.Errorf("expected 2, got %d", countDecimalPlaces("1.23"))
	}
}

func TestCountDecimalPlaces_NoDecimal(t *testing.T) {
	if countDecimalPlaces("100") != 0 {
		t.Errorf("expected 0, got %d", countDecimalPlaces("100"))
	}
}

func TestCountDecimalPlaces_ManyDecimals(t *testing.T) {
	if countDecimalPlaces("3.14159") != 5 {
		t.Errorf("expected 5, got %d", countDecimalPlaces("3.14159"))
	}
}

func TestCountDecimalPlaces_ZeroDecimals(t *testing.T) {
	if countDecimalPlaces("1.0") != 1 {
		t.Errorf("expected 1, got %d", countDecimalPlaces("1.0"))
	}
}

// --- matchRegex Tests ---

func TestMatchRegex_ValidEmail(t *testing.T) {
	if !matchRegex(`^[\w\.\-]+@[\w\.\-]+\.\w+$`, "test@example.com") {
		t.Error("expected email to match")
	}
}

func TestMatchRegex_InvalidEmail(t *testing.T) {
	if matchRegex(`^[\w\.\-]+@[\w\.\-]+\.\w+$`, "not-an-email") {
		t.Error("expected 'not-an-email' to not match email regex")
	}
}

func TestMatchRegex_InvalidPattern(t *testing.T) {
	if matchRegex(`[invalid`, "anything") {
		t.Error("expected false for invalid regex pattern")
	}
}

func TestMatchRegex_DatePattern(t *testing.T) {
	if !matchRegex(`^\d{4}-\d{2}-\d{2}`, "2025-01-15") {
		t.Error("expected date to match")
	}
}

// --- enumValuesFromSet Tests ---

func TestEnumValuesFromSet_Small(t *testing.T) {
	set := map[string]struct{}{"a": {}, "b": {}, "c": {}}
	result := enumValuesFromSet(set)
	if len(result) != 3 {
		t.Errorf("expected 3 values, got %d", len(result))
	}
}

func TestEnumValuesFromSet_Empty(t *testing.T) {
	set := map[string]struct{}{}
	result := enumValuesFromSet(set)
	if result != nil {
		t.Errorf("expected nil for empty set, got %v", result)
	}
}

func TestEnumValuesFromSet_TooLarge(t *testing.T) {
	set := make(map[string]struct{})
	for i := 0; i < 15; i++ {
		set[fmt.Sprintf("val%d", i)] = struct{}{}
	}
	result := enumValuesFromSet(set)
	if result != nil {
		t.Errorf("expected nil for set > 10, got %v", result)
	}
}

// --- distributionType Tests ---

func TestDistributionType_Constant(t *testing.T) {
	dist := map[string]interface{}{"distribution": "constant"}
	if distributionType(dist) != "constant" {
		t.Errorf("expected 'constant'")
	}
}

func TestDistributionType_Unknown(t *testing.T) {
	dist := map[string]interface{}{}
	if distributionType(dist) != "unknown" {
		t.Errorf("expected 'unknown' for missing key")
	}
}

func TestDistributionType_WrongType(t *testing.T) {
	dist := map[string]interface{}{"distribution": 42}
	if distributionType(dist) != "unknown" {
		t.Errorf("expected 'unknown' for non-string value")
	}
}

// --- columnPatternAccumulator Tests ---

func TestColumnPatternAccumulator_ConsumeNil(t *testing.T) {
	acc := newColumnPatternAccumulator("unknown")
	acc.consume(nil)
	if acc.nullCount != 1 {
		t.Errorf("expected nullCount=1, got %d", acc.nullCount)
	}
}

func TestColumnPatternAccumulator_ConsumeUnique(t *testing.T) {
	acc := newColumnPatternAccumulator("unknown")
	acc.consume("a")
	acc.consume("b")
	acc.consume("a")
	if len(acc.uniqueSet) != 2 {
		t.Errorf("expected 2 unique values, got %d", len(acc.uniqueSet))
	}
}

func TestColumnPatternAccumulator_ConsumeNumeric(t *testing.T) {
	acc := newColumnPatternAccumulator("unknown")
	acc.consume(10)
	acc.consume(20.5)
	acc.consume(5)
	if !acc.minSet || acc.min != 5.0 {
		t.Errorf("expected min=5.0, got %v (set=%v)", acc.min, acc.minSet)
	}
	if !acc.maxSet || acc.max != 20.5 {
		t.Errorf("expected max=20.5, got %v (set=%v)", acc.max, acc.maxSet)
	}
}

func TestColumnPatternAccumulator_ConsumeSemanticPattern(t *testing.T) {
	acc := newColumnPatternAccumulator("unknown")
	acc.consume("test@example.com")
	if acc.patternType != "email" {
		t.Errorf("expected patternType='email', got %q", acc.patternType)
	}
}

func TestColumnPatternAccumulator_BuildPattern(t *testing.T) {
	acc := newColumnPatternAccumulator("unknown")
	acc.consume(1.5)
	acc.consume(2.5)
	acc.consume(3.5)
	dist := map[string]interface{}{"distribution": "variable"}
	pattern := acc.buildPattern([]interface{}{1.5, 2.5, 3.5}, dist)
	if pattern == nil {
		t.Fatal("expected non-nil pattern")
	}
	if pattern.Range == nil {
		t.Fatal("expected non-nil range")
	}
	if pattern.DecimalPlaces != 1 {
		t.Errorf("expected decimalPlaces=1, got %d", pattern.DecimalPlaces)
	}
	if pattern.NullPercentage != 0.0 {
		t.Errorf("expected nullPercentage=0, got %f", pattern.NullPercentage)
	}
}

func TestColumnPatternAccumulator_ValueRange_NotSet(t *testing.T) {
	acc := newColumnPatternAccumulator("unknown")
	acc.consume("text") // non-numeric, min/max not set
	rng := acc.valueRange()
	if rng != nil {
		t.Errorf("expected nil range for non-numeric values")
	}
}

// --- DetectColumnPattern Tests ---

func TestDetectColumnPattern_Email(t *testing.T) {
	values := []interface{}{"alice@example.com", "bob@test.org", "carol@domain.net"}
	pattern := DetectColumnPattern("users", "email", values)
	if pattern == nil {
		t.Fatal("expected non-nil pattern")
	}
	if pattern.PatternType != "email" {
		t.Errorf("expected patternType='email', got %q", pattern.PatternType)
	}
	if pattern.NullPercentage != 0.0 {
		t.Errorf("expected nullPercentage=0, got %f", pattern.NullPercentage)
	}
}

func TestDetectColumnPattern_Numeric(t *testing.T) {
	values := []interface{}{10, 20, 30, 40, 50}
	pattern := DetectColumnPattern("scores", "value", values)
	if pattern == nil {
		t.Fatal("expected non-nil pattern")
	}
	if pattern.Range == nil {
		t.Fatal("expected non-nil range for numeric values")
	}
	if pattern.Uniqueness != 1.0 {
		t.Errorf("expected uniqueness=1.0, got %f", pattern.Uniqueness)
	}
}

func TestDetectColumnPattern_WithNulls(t *testing.T) {
	values := []interface{}{"a", nil, "b", nil}
	pattern := DetectColumnPattern("test", "col", values)
	if pattern == nil {
		t.Fatal("expected non-nil pattern")
	}
	if pattern.NullPercentage != 0.5 {
		t.Errorf("expected nullPercentage=0.5, got %f", pattern.NullPercentage)
	}
}

func TestDetectColumnPattern_Empty(t *testing.T) {
	pattern := DetectColumnPattern("test", "col", []interface{}{})
	if pattern == nil {
		t.Fatal("expected non-nil pattern")
	}
}

// --- AnalyzeDataPatterns Tests ---

func TestAnalyzeDataPatterns_EmailAndNumeric(t *testing.T) {
	sampleData := []map[string]interface{}{
		{"email": "alice@example.com", "score": 1.5},
		{"email": "bob@test.org", "score": 2.5},
		{"email": "carol@domain.net", "score": 3.5},
	}
	columns := []SchemaColumnInfo{
		{ColumnName: "email"},
		{ColumnName: "score"},
	}
	patterns := AnalyzeDataPatterns("users", sampleData, columns)
	if len(patterns) != 2 {
		t.Fatalf("expected 2 patterns, got %d", len(patterns))
	}
	if patterns[0].PatternType != "email" {
		t.Errorf("expected email pattern, got %q", patterns[0].PatternType)
	}
	if patterns[1].Range == nil {
		t.Error("expected numeric range for score")
	}
}

func TestAnalyzeDataPatterns_WithNulls(t *testing.T) {
	sampleData := []map[string]interface{}{
		{"name": "Alice"},
		{"name": nil},
		{"name": "Bob"},
	}
	columns := []SchemaColumnInfo{{ColumnName: "name"}}
	patterns := AnalyzeDataPatterns("users", sampleData, columns)
	if len(patterns) != 1 {
		t.Fatalf("expected 1 pattern, got %d", len(patterns))
	}
	if patterns[0].NullPercentage != 1.0/3.0 {
		t.Errorf("expected nullPercentage=%.4f, got %f", 1.0/3.0, patterns[0].NullPercentage)
	}
}

func TestAnalyzeDataPatterns_EmptyColumns(t *testing.T) {
	sampleData := []map[string]interface{}{{"a": 1}}
	patterns := AnalyzeDataPatterns("test", sampleData, []SchemaColumnInfo{})
	if len(patterns) != 0 {
		t.Errorf("expected 0 patterns for empty columns, got %d", len(patterns))
	}
}

func TestAnalyzeDataPatterns_MissingColumnInData(t *testing.T) {
	sampleData := []map[string]interface{}{
		{"other": "value"},
	}
	columns := []SchemaColumnInfo{{ColumnName: "missing_col"}}
	patterns := AnalyzeDataPatterns("test", sampleData, columns)
	if len(patterns) != 1 {
		t.Fatalf("expected 1 pattern, got %d", len(patterns))
	}
	// Missing column (no values) defaults to "unknown" pattern type
	if patterns[0].PatternType != "unknown" {
		t.Errorf("expected 'unknown' patternType for missing column, got %q", patterns[0].PatternType)
	}
}

func TestAnalyzeDataPatterns_DatePattern(t *testing.T) {
	sampleData := []map[string]interface{}{
		{"event_date": "2025-01-15T10:30:00"},
		{"event_date": "2025-02-20T14:45:00"},
		{"event_date": "2025-03-10T09:15:00"},
	}
	columns := []SchemaColumnInfo{{ColumnName: "event_date"}}
	patterns := AnalyzeDataPatterns("events", sampleData, columns)
	if len(patterns) != 1 {
		t.Fatalf("expected 1 pattern, got %d", len(patterns))
	}
	if patterns[0].PatternType != "date" {
		t.Errorf("expected date pattern, got %q", patterns[0].PatternType)
	}
}

// --- Quality Metrics Tests ---

func TestCollectColumnSampleValues(t *testing.T) {
	sampleData := []map[string]interface{}{
		{"name": "Alice", "age": 30},
		{"name": "Bob", "age": 25},
		{"name": nil, "age": 35},
	}
	values := CollectColumnSampleValues(sampleData, "name")
	if len(values) != 3 {
		t.Fatalf("expected 3 values, got %d", len(values))
	}
	if values[0] != "Alice" {
		t.Errorf("expected 'Alice', got %v", values[0])
	}
	if values[2] != nil {
		t.Errorf("expected nil, got %v", values[2])
	}
}

func TestCollectColumnSampleValues_MissingColumn(t *testing.T) {
	sampleData := []map[string]interface{}{{"name": "Alice"}}
	values := CollectColumnSampleValues(sampleData, "missing")
	if len(values) != 0 {
		t.Errorf("expected 0 values for missing column, got %d", len(values))
	}
}

func TestComputeColumnQualityMetrics_NoData(t *testing.T) {
	col := SchemaColumnInfo{ColumnName: "test"}
	metrics := ComputeColumnQualityMetrics(col, []interface{}{})
	if metrics.OverallScore != 0 {
		t.Errorf("expected overall score 0, got %f", metrics.OverallScore)
	}
	if len(metrics.Issues) != 1 || metrics.Issues[0] != "No sample data available" {
		t.Errorf("expected 'No sample data available' issue, got %v", metrics.Issues)
	}
}

func TestComputeColumnQualityMetrics_AllNonNull(t *testing.T) {
	col := SchemaColumnInfo{ColumnName: "name"}
	metrics := ComputeColumnQualityMetrics(col, []interface{}{"Alice", "Bob", "Charlie"})
	if metrics.Completeness != 1.0 {
		t.Errorf("expected completeness=1.0, got %f", metrics.Completeness)
	}
	if metrics.Uniqueness != 1.0 {
		t.Errorf("expected uniqueness=1.0, got %f", metrics.Uniqueness)
	}
	if metrics.Validity != 1.0 {
		t.Errorf("expected validity=1.0, got %f", metrics.Validity)
	}
}

func TestComputeColumnQualityMetrics_WithNulls(t *testing.T) {
	col := SchemaColumnInfo{ColumnName: "email"}
	metrics := ComputeColumnQualityMetrics(col, []interface{}{"a@b.com", nil, "c@d.com", nil})
	if metrics.Completeness != 0.5 {
		t.Errorf("expected completeness=0.5, got %f", metrics.Completeness)
	}
	if len(metrics.Issues) != 2 {
		t.Errorf("expected 2 null issues, got %d", len(metrics.Issues))
	}
}

func TestComputeColumnQualityMetrics_WithValidationRegex(t *testing.T) {
	col := SchemaColumnInfo{
		ColumnName:      "email",
		PatternType:     "email",
		ValidationRegex: `^[\w\.\-]+@[\w\.\-]+\.\w+$`,
	}
	metrics := ComputeColumnQualityMetrics(col, []interface{}{"valid@example.com", "invalid-email", "good@test.org"})
	if metrics.Validity != 2.0/3.0 {
		t.Errorf("expected validity=%.4f, got %f", 2.0/3.0, metrics.Validity)
	}
}

func TestComputeColumnQualityMetrics_TemporalConsistent(t *testing.T) {
	col := SchemaColumnInfo{ColumnName: "created_at", DataType: "date"}
	metrics := ComputeColumnQualityMetrics(col, []interface{}{"2025-01-01", "2025-01-02", "2025-01-03"})
	if metrics.TemporalConsistency != 1.0 {
		t.Errorf("expected temporal consistency=1.0, got %f", metrics.TemporalConsistency)
	}
}

func TestComputeColumnQualityMetrics_TemporalInconsistent(t *testing.T) {
	col := SchemaColumnInfo{ColumnName: "created_at", DataType: "date"}
	metrics := ComputeColumnQualityMetrics(col, []interface{}{"2025-01-03", "2025-01-01", "2025-01-02"})
	if metrics.TemporalConsistency != 0.0 {
		t.Errorf("expected temporal consistency=0.0, got %f", metrics.TemporalConsistency)
	}
}

func TestTruncateQualityIssues_UnderLimit(t *testing.T) {
	issues := []string{"issue1", "issue2"}
	result := TruncateQualityIssues(issues)
	if len(result) != 2 {
		t.Errorf("expected 2 issues, got %d", len(result))
	}
}

func TestTruncateQualityIssues_OverLimit(t *testing.T) {
	issues := make([]string, 15)
	for i := range issues {
		issues[i] = fmt.Sprintf("issue%d", i)
	}
	result := TruncateQualityIssues(issues)
	if len(result) != maxQualityIssuesPerColumn+1 {
		t.Errorf("expected %d items, got %d", maxQualityIssuesPerColumn+1, len(result))
	}
	last := result[len(result)-1]
	if !strings.Contains(last, "truncated") {
		t.Errorf("expected truncation message, got %q", last)
	}
}

func TestGenerateDataQualityMetrics(t *testing.T) {
	sampleData := []map[string]interface{}{
		{"name": "Alice", "email": "alice@example.com"},
		{"name": "Bob", "email": "bob@test.org"},
	}
	columns := []SchemaColumnInfo{
		{ColumnName: "name"},
		{ColumnName: "email"},
	}
	metrics := GenerateDataQualityMetrics(sampleData, columns)
	if len(metrics) != 2 {
		t.Fatalf("expected 2 metrics, got %d", len(metrics))
	}
	if _, ok := metrics["name"]; !ok {
		t.Error("expected 'name' metric")
	}
	if _, ok := metrics["email"]; !ok {
		t.Error("expected 'email' metric")
	}
}

// --- Aggregate Metric Tests ---

func TestAddTableAggregateMetric(t *testing.T) {
	metrics := map[string]QualityMetrics{
		"col1": {OverallScore: 0.8},
		"col2": {OverallScore: 0.6},
	}
	AddTableAggregateMetric(metrics)
	tableMetric, ok := metrics["__table__"]
	if !ok {
		t.Fatal("expected __table__ metric")
	}
	if tableMetric.OverallScore != 0.7 {
		t.Errorf("expected 0.7, got %f", tableMetric.OverallScore)
	}
}

func TestAddTableAggregateMetric_Empty(t *testing.T) {
	metrics := map[string]QualityMetrics{}
	AddTableAggregateMetric(metrics)
	if _, ok := metrics["__table__"]; ok {
		t.Error("expected no __table__ metric for empty input")
	}
}

func TestFlattenQualityMetrics(t *testing.T) {
	metrics := make(map[string]QualityMetrics)
	columnMetrics := map[string]QualityMetrics{
		"col1":      {OverallScore: 0.8},
		"__table__": {OverallScore: 0.7},
	}
	FlattenQualityMetrics(metrics, "users", columnMetrics)
	if _, ok := metrics["users.col1"]; !ok {
		t.Error("expected 'users.col1' key")
	}
	if _, ok := metrics["users"]; !ok {
		t.Error("expected 'users' key for __table__")
	}
}

func TestAddDatabaseAggregateMetric(t *testing.T) {
	metrics := map[string]QualityMetrics{
		"users.__table__":  {OverallScore: 0.8},
		"orders.__table__": {OverallScore: 0.6},
		"users.name":       {OverallScore: 0.9},
		"orders.total":     {OverallScore: 0.5},
	}
	AddDatabaseAggregateMetric(metrics)
	dbMetric, ok := metrics["__database__"]
	if !ok {
		t.Fatal("expected __database__ metric")
	}
	// Average of 0.9 + 0.5 = 0.7
	if dbMetric.OverallScore != 0.7 {
		t.Errorf("expected 0.7, got %f", dbMetric.OverallScore)
	}
}

func TestAddDatabaseAggregateMetric_Empty(t *testing.T) {
	metrics := map[string]QualityMetrics{}
	AddDatabaseAggregateMetric(metrics)
	if _, ok := metrics["__database__"]; ok {
		t.Error("expected no __database__ metric for empty input")
	}
}

// --- CategorizeTables Tests ---

func TestCategorizeTables_CoreEntities(t *testing.T) {
	schemas := map[string]TableInfo{
		"users":     {ColumnCount: 5, KeyColumns: KeyColumns{PrimaryKey: "id"}},
		"products":  {ColumnCount: 8, KeyColumns: KeyColumns{PrimaryKey: "id"}},
		"customers": {ColumnCount: 6, KeyColumns: KeyColumns{PrimaryKey: "id"}},
	}
	tableNames := []string{"users", "products", "customers"}
	catalog := CategorizeTables(tableNames, schemas, nil)
	if len(catalog.CoreEntities) != 3 {
		t.Errorf("expected 3 core entities, got %d", len(catalog.CoreEntities))
	}
	if len(catalog.LookupTables) != 0 || len(catalog.JunctionTables) != 0 || len(catalog.AuditTables) != 0 {
		t.Error("expected no lookup/junction/audit tables")
	}
}

func TestCategorizeTables_LookupTables(t *testing.T) {
	schemas := map[string]TableInfo{
		"status_lookup": {},
		"order_type":    {},
		"user_status":   {},
	}
	tableNames := []string{"status_lookup", "order_type", "user_status"}
	catalog := CategorizeTables(tableNames, schemas, nil)
	if len(catalog.LookupTables) != 3 {
		t.Errorf("expected 3 lookup tables, got %d", len(catalog.LookupTables))
	}
}

func TestCategorizeTables_JunctionTables(t *testing.T) {
	schemas := map[string]TableInfo{
		"user_role_junction": {},
		"order_join":         {},
	}
	tableNames := []string{"user_role_junction", "order_join"}
	catalog := CategorizeTables(tableNames, schemas, nil)
	if len(catalog.JunctionTables) != 2 {
		t.Errorf("expected 2 junction tables, got %d", len(catalog.JunctionTables))
	}
}

func TestCategorizeTables_AuditTables(t *testing.T) {
	schemas := map[string]TableInfo{
		"audit_log":      {},
		"system_logs":    {},
		"activity_audit": {},
	}
	tableNames := []string{"audit_log", "system_logs", "activity_audit"}
	catalog := CategorizeTables(tableNames, schemas, nil)
	if len(catalog.AuditTables) != 3 {
		t.Errorf("expected 3 audit tables, got %d", len(catalog.AuditTables))
	}
}

func TestCategorizeTables_Mixed(t *testing.T) {
	schemas := map[string]TableInfo{
		"users":              {ColumnCount: 5},
		"status_lookup":      {},
		"user_role_junction": {},
		"audit_log":          {},
	}
	tableNames := []string{"users", "status_lookup", "user_role_junction", "audit_log"}
	catalog := CategorizeTables(tableNames, schemas, nil)
	if len(catalog.CoreEntities) != 1 {
		t.Errorf("expected 1 core entity, got %d", len(catalog.CoreEntities))
	}
	if len(catalog.LookupTables) != 1 {
		t.Errorf("expected 1 lookup table, got %d", len(catalog.LookupTables))
	}
	if len(catalog.JunctionTables) != 1 {
		t.Errorf("expected 1 junction table, got %d", len(catalog.JunctionTables))
	}
	if len(catalog.AuditTables) != 1 {
		t.Errorf("expected 1 audit table, got %d", len(catalog.AuditTables))
	}
}

func TestCategorizeTables_Empty(t *testing.T) {
	catalog := CategorizeTables([]string{}, map[string]TableInfo{}, nil)
	if len(catalog.CoreEntities) != 0 || len(catalog.LookupTables) != 0 ||
		len(catalog.JunctionTables) != 0 || len(catalog.AuditTables) != 0 {
		t.Error("expected empty catalog for empty input")
	}
}

func TestCategorizeTables_EntityMetadata(t *testing.T) {
	schemas := map[string]TableInfo{
		"users": {ColumnCount: 5, KeyColumns: KeyColumns{PrimaryKey: "id"}},
	}
	tableNames := []string{"users"}
	catalog := CategorizeTables(tableNames, schemas, nil)
	if len(catalog.CoreEntities) != 1 {
		t.Fatal("expected 1 core entity")
	}
	entity := catalog.CoreEntities[0]
	if entity.TableName != "users" {
		t.Errorf("expected table name 'users', got %q", entity.TableName)
	}
	if entity.ColumnCount != 5 {
		t.Errorf("expected column count 5, got %d", entity.ColumnCount)
	}
	if entity.PrimaryKey != "id" {
		t.Errorf("expected primary key 'id', got %q", entity.PrimaryKey)
	}
	if entity.BusinessRole != "core" {
		t.Errorf("expected business role 'core', got %q", entity.BusinessRole)
	}
}

func TestCategorizeTables_JunctionByFKs(t *testing.T) {
	// A table with 2+ outgoing FKs and few non-FK columns should be classified as junction
	tableNames := []string{"order_items", "products", "users"}
	schemas := map[string]TableInfo{
		"order_items": {ColumnCount: 4, KeyColumns: KeyColumns{PrimaryKey: "id"}},
		"products":    {ColumnCount: 5, KeyColumns: KeyColumns{PrimaryKey: "id"}},
		"users":       {ColumnCount: 5, KeyColumns: KeyColumns{PrimaryKey: "id"}},
	}
	fks := []ForeignKeyRelationship{
		{FromTable: "order_items", FromColumn: "product_id", ToTable: "products", ToColumn: "id"},
		{FromTable: "order_items", FromColumn: "user_id", ToTable: "users", ToColumn: "id"},
	}
	catalog := CategorizeTables(tableNames, schemas, fks)
	// order_items has 2 outgoing FKs and only 4 columns → junction
	if len(catalog.JunctionTables) != 1 || catalog.JunctionTables[0].TableName != "order_items" {
		t.Errorf("expected order_items as junction, got junction=%v, core=%v", catalog.JunctionTables, catalog.CoreEntities)
	}
}

func TestCategorizeTables_FKSignals(t *testing.T) {
	tableNames := []string{"orders", "users"}
	schemas := map[string]TableInfo{
		"orders": {ColumnCount: 5, KeyColumns: KeyColumns{PrimaryKey: "id"}},
		"users":  {ColumnCount: 3, KeyColumns: KeyColumns{PrimaryKey: "id"}},
	}
	fks := []ForeignKeyRelationship{
		{FromTable: "orders", FromColumn: "user_id", ToTable: "users", ToColumn: "id"},
	}
	catalog := CategorizeTables(tableNames, schemas, fks)
	// Check FK signal fields
	var ordersEntity, usersEntity *TableEntity
	for i := range catalog.CoreEntities {
		if catalog.CoreEntities[i].TableName == "orders" {
			ordersEntity = &catalog.CoreEntities[i]
		}
		if catalog.CoreEntities[i].TableName == "users" {
			usersEntity = &catalog.CoreEntities[i]
		}
	}
	if ordersEntity == nil {
		t.Fatal("expected orders in core entities")
	}
	if usersEntity == nil {
		t.Fatal("expected users in core entities")
	}
	if ordersEntity.OutgoingFKs != 1 {
		t.Errorf("expected orders OutgoingFKs=1, got %d", ordersEntity.OutgoingFKs)
	}
	if usersEntity.IncomingFKs != 1 {
		t.Errorf("expected users IncomingFKs=1, got %d", usersEntity.IncomingFKs)
	}
}

// --- BuildQualityMetrics Tests ---

func TestBuildQualityMetrics_BasicLevel(t *testing.T) {
	schemas := map[string]TableInfo{
		"users": {Columns: []SchemaColumnInfo{{ColumnName: "id"}}},
	}
	sampleData := map[string][]map[string]interface{}{
		"users": {{"id": "1"}, {"id": "2"}},
	}
	metrics := BuildQualityMetrics(AnalysisLevelBasic, schemas, sampleData)
	if len(metrics) != 0 {
		t.Errorf("expected empty metrics for basic level, got %d entries", len(metrics))
	}
}

func TestBuildQualityMetrics_DetailedLevel(t *testing.T) {
	schemas := map[string]TableInfo{
		"users": {Columns: []SchemaColumnInfo{
			{ColumnName: "id", DataType: "int"},
			{ColumnName: "name", DataType: "varchar"},
		}},
	}
	sampleData := map[string][]map[string]interface{}{
		"users": {
			{"id": "1", "name": "Alice"},
			{"id": "2", "name": "Bob"},
			{"id": "3", "name": nil},
		},
	}
	metrics := BuildQualityMetrics(AnalysisLevelDetailed, schemas, sampleData)
	// Should have: users.id, users.name, users (table aggregate), __database__
	if len(metrics) < 3 {
		t.Errorf("expected at least 3 metric entries, got %d", len(metrics))
	}
	// Check table aggregate (key is just tableName, not tableName.__table__)
	if _, ok := metrics["users"]; !ok {
		t.Error("expected users table aggregate metric")
	}
	// Check __database__ aggregate exists
	if _, ok := metrics["__database__"]; !ok {
		t.Error("expected __database__ aggregate metric")
	}
}

func TestBuildQualityMetrics_ComprehensiveLevel(t *testing.T) {
	schemas := map[string]TableInfo{
		"users": {Columns: []SchemaColumnInfo{{ColumnName: "id", DataType: "int"}}},
	}
	sampleData := map[string][]map[string]interface{}{
		"users": {{"id": "1"}, {"id": "2"}, {"id": "3"}},
	}
	metrics := BuildQualityMetrics(AnalysisLevelComprehensive, schemas, sampleData)
	if len(metrics) < 2 {
		t.Errorf("expected at least 2 metric entries, got %d", len(metrics))
	}
}

func TestBuildQualityMetrics_EmptyTables(t *testing.T) {
	schemas := map[string]TableInfo{}
	sampleData := map[string][]map[string]interface{}{}
	metrics := BuildQualityMetrics(AnalysisLevelDetailed, schemas, sampleData)
	if len(metrics) != 0 {
		t.Errorf("expected empty metrics for empty input, got %d entries", len(metrics))
	}
}

func TestBuildQualityMetrics_MultipleTables(t *testing.T) {
	schemas := map[string]TableInfo{
		"users":  {Columns: []SchemaColumnInfo{{ColumnName: "id", DataType: "int"}}},
		"orders": {Columns: []SchemaColumnInfo{{ColumnName: "id", DataType: "int"}}},
	}
	sampleData := map[string][]map[string]interface{}{
		"users":  {{"id": "1"}},
		"orders": {{"id": "100"}},
	}
	metrics := BuildQualityMetrics(AnalysisLevelDetailed, schemas, sampleData)
	// Should have: users.id, orders.id, users (table agg), orders (table agg), __database__
	expectedKeys := []string{"users.id", "orders.id", "users", "orders", "__database__"}
	for _, key := range expectedKeys {
		if _, ok := metrics[key]; !ok {
			t.Errorf("expected metric key %q, not found", key)
		}
	}
}
