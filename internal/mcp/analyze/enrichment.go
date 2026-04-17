package analyze

import (
	"fmt"
	"strings"
)

// enrichment.go handles business context, query suggestions, quality metrics, and data patterns.
// BUG-007/BUG-008 fix: replaces hardcoded domain/entity classification with ClassificationSignals.

// notableColumnKeywords identifies column names that suggest a specific business domain.
// Generic columns (id, name, status, etc.) are excluded — only domain-significant ones are kept.
var notableColumnKeywords = map[string]bool{
	// Telephony/VoIP
	"caller": true, "callee": true, "duration": true, "sip_uri": true,
	"phone": true, "extension": true, "dial": true, "ring": true,
	"voicemail": true, "conference": true, "recording": true,
	// E-commerce
	"sku": true, "cart": true, "invoice_number": true, "shipment": true,
	"tracking_number": true, "payment_method": true, "discount": true,
	// Healthcare
	"diagnosis": true, "prescription": true, "medical_record": true,
	"patient_id": true, "appointment_date": true, "provider_id": true,
	// Finance
	"account_number": true, "routing_number": true, "ledger": true,
	"balance": true, "interest_rate": true, "transaction_type": true,
	// General domain-significant
	"email": true, "latitude": true, "longitude": true,
	"api_key": true, "token": true, "secret": true,
	"price": true, "quantity": true, "amount": true,
	"url": true, "hostname": true, "port": true, "protocol": true,
}

// BuildClassificationSignals extracts raw signals from table schemas and FK relationships
// for LLM-based domain and entity classification. Replaces hardcoded detectDomain() and
// identifyEntityTypes() which produced inaccurate results (BUG-007, BUG-008).
func BuildClassificationSignals(tableColumns map[string][]SchemaColumnInfo, fks []ForeignKeyRelationship) ClassificationSignals {
	if len(tableColumns) == 0 {
		return ClassificationSignals{
			NamingPrefixes: map[string]int{},
			FKSummary:     "none",
		}
	}

	var tableNames []string
	totalCols := 0
	for name := range tableColumns {
		tableNames = append(tableNames, name)
		totalCols += len(tableColumns[name])
	}

	return ClassificationSignals{
		TableNames:     tableNames,
		NamingPrefixes: extractNamingPrefixes(tableColumns),
		NotableColumns: extractNotableColumns(tableColumns),
		FKSummary:      buildFKSummary(fks),
		TotalTables:    len(tableColumns),
		TotalColumns:   totalCols,
	}
}

// extractNamingPrefixes finds underscore-delimited prefixes in table names
// and counts their frequency. E.g., "broadcast_messages" → prefix "broadcast".
func extractNamingPrefixes(tableColumns map[string][]SchemaColumnInfo) map[string]int {
	prefixes := make(map[string]int)
	for name := range tableColumns {
		idx := strings.Index(name, "_")
		if idx <= 0 {
			continue // no underscore or starts with underscore
		}
		prefix := name[:idx]
		prefixes[prefix]++
	}
	return prefixes
}

// extractNotableColumns collects domain-significant column names across all tables.
// Generic columns (id, name, status, etc.) are excluded.
func extractNotableColumns(tableColumns map[string][]SchemaColumnInfo) []string {
	seen := make(map[string]bool)
	var notable []string

	for _, columns := range tableColumns {
		for _, col := range columns {
			name := col.ColumnName
			if notableColumnKeywords[name] && !seen[name] {
				notable = append(notable, name)
				seen[name] = true
			}
		}
	}
	return notable
}

// buildFKSummary creates a human-readable summary of FK relationships.
func buildFKSummary(fks []ForeignKeyRelationship) string {
	if len(fks) == 0 {
		return "none"
	}
	var parts []string
	for _, fk := range fks {
		parts = append(parts, fmt.Sprintf("%s.%s → %s.%s", fk.FromTable, fk.FromColumn, fk.ToTable, fk.ToColumn))
	}
	return strings.Join(parts, "; ")
}
