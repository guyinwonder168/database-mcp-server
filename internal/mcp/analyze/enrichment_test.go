package analyze

import (
	"testing"
)

func TestBuildClassificationSignals_Basic(t *testing.T) {
	tableColumns := map[string][]SchemaColumnInfo{
		"broadcast_messages": {{ColumnName: "id"}, {ColumnName: "content"}, {ColumnName: "channel_id"}},
		"broadcast_channels": {{ColumnName: "id"}, {ColumnName: "name"}},
		"call_sessions":      {{ColumnName: "id"}, {ColumnName: "caller"}, {ColumnName: "callee"}, {ColumnName: "duration"}},
		"call_legs":          {{ColumnName: "id"}, {ColumnName: "session_id"}, {ColumnName: "status"}},
		"adm_users":          {{ColumnName: "id"}, {ColumnName: "username"}},
		"adm_roles":          {{ColumnName: "id"}, {ColumnName: "role_name"}},
		"customers":          {{ColumnName: "id"}, {ColumnName: "email"}, {ColumnName: "phone"}},
	}

	fks := []ForeignKeyRelationship{
		{FromTable: "call_legs", FromColumn: "session_id", ToTable: "call_sessions", ToColumn: "id"},
		{FromTable: "broadcast_messages", FromColumn: "channel_id", ToTable: "broadcast_channels", ToColumn: "id"},
	}

	signals := BuildClassificationSignals(tableColumns, fks)

	if signals.TotalTables != 7 {
		t.Errorf("expected TotalTables=7, got %d", signals.TotalTables)
	}
	if signals.TotalColumns == 0 {
		t.Error("expected TotalColumns > 0")
	}
	if len(signals.TableNames) != 7 {
		t.Errorf("expected 7 table names, got %d", len(signals.TableNames))
	}
}

func TestBuildClassificationSignals_NamingPrefixes(t *testing.T) {
	tableColumns := map[string][]SchemaColumnInfo{
		"broadcast_messages": {{ColumnName: "id"}},
		"broadcast_channels": {{ColumnName: "id"}},
		"call_sessions":      {{ColumnName: "id"}},
		"call_legs":          {{ColumnName: "id"}},
		"adm_users":          {{ColumnName: "id"}},
		"adm_roles":          {{ColumnName: "id"}},
		"customers":          {{ColumnName: "id"}},
	}

	signals := BuildClassificationSignals(tableColumns, nil)

	if signals.NamingPrefixes["broadcast"] != 2 {
		t.Errorf("expected broadcast prefix count=2, got %d", signals.NamingPrefixes["broadcast"])
	}
	if signals.NamingPrefixes["call"] != 2 {
		t.Errorf("expected call prefix count=2, got %d", signals.NamingPrefixes["call"])
	}
	if signals.NamingPrefixes["adm"] != 2 {
		t.Errorf("expected adm prefix count=2, got %d", signals.NamingPrefixes["adm"])
	}
	// "customers" has no underscore prefix — should not appear
	if _, exists := signals.NamingPrefixes["customers"]; exists {
		t.Error("expected 'customers' to not be a prefix (no underscore)")
	}
}

func TestBuildClassificationSignals_NotableColumns(t *testing.T) {
	tableColumns := map[string][]SchemaColumnInfo{
		"call_sessions":  {{ColumnName: "id"}, {ColumnName: "caller"}, {ColumnName: "callee"}, {ColumnName: "duration"}, {ColumnName: "sip_uri"}},
		"sip_subscribers": {{ColumnName: "id"}, {ColumnName: "sip_uri"}, {ColumnName: "domain"}},
		"customers":       {{ColumnName: "id"}, {ColumnName: "email"}, {ColumnName: "phone"}},
	}

	signals := BuildClassificationSignals(tableColumns, nil)

	// Columns with domain-significant names should be captured
	notableSet := make(map[string]bool)
	for _, col := range signals.NotableColumns {
		notableSet[col] = true
	}

	// Domain-significant columns
	for _, expected := range []string{"caller", "callee", "duration", "sip_uri", "phone"} {
		if !notableSet[expected] {
			t.Errorf("expected notable column %q not found in signals", expected)
		}
	}

	// Generic columns should NOT be notable
	for _, unexpected := range []string{"id", "domain"} {
		if notableSet[unexpected] {
			t.Errorf("generic column %q should not be in notable columns", unexpected)
		}
	}
}

func TestBuildClassificationSignals_FKSummary(t *testing.T) {
	tableColumns := map[string][]SchemaColumnInfo{
		"users":    {{ColumnName: "id"}},
		"orders":   {{ColumnName: "id"}, {ColumnName: "user_id"}},
		"products": {{ColumnName: "id"}},
	}

	fks := []ForeignKeyRelationship{
		{FromTable: "orders", FromColumn: "user_id", ToTable: "users", ToColumn: "id"},
	}

	signals := BuildClassificationSignals(tableColumns, fks)

	if signals.FKSummary == "" {
		t.Error("expected non-empty FK summary")
	}
	// Should mention the FK relationship
	if len(fks) > 0 && signals.FKSummary == "none" {
		t.Error("FK summary should describe relationships, not say 'none'")
	}
}

func TestBuildClassificationSignals_NoFKs(t *testing.T) {
	tableColumns := map[string][]SchemaColumnInfo{
		"users": {{ColumnName: "id"}},
	}

	signals := BuildClassificationSignals(tableColumns, nil)

	if signals.FKSummary != "none" {
		t.Errorf("expected FK summary 'none' when no FKs, got %q", signals.FKSummary)
	}
}

func TestBuildClassificationSignals_EmptyInput(t *testing.T) {
	signals := BuildClassificationSignals(nil, nil)

	if signals.TotalTables != 0 {
		t.Errorf("expected TotalTables=0, got %d", signals.TotalTables)
	}
	if signals.TotalColumns != 0 {
		t.Errorf("expected TotalColumns=0, got %d", signals.TotalColumns)
	}
	if len(signals.TableNames) != 0 {
		t.Errorf("expected 0 table names, got %d", len(signals.TableNames))
	}
}
