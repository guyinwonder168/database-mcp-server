package mcp

import (
	"context"
	"testing"
)

func TestEnhanceSchemaAnalysis(t *testing.T) {
	ctx := context.Background()
	tableSchemas := map[string]TableInfo{
		"users": {
			Columns: []SchemaColumnInfo{
				{ColumnName: "id", DataType: "integer"},
				{ColumnName: "email", DataType: "text"},
			},
		},
	}
	sampleData := map[string][]map[string]interface{}{
		"users": {
			{"id": 1, "email": "alice@example.com"},
			{"id": 2, "email": "bob@example.com"},
		},
	}

	enhanced := enhanceSchemaAnalysis(ctx, tableSchemas, sampleData, 2)
	if enhanced == nil {
		t.Fatalf("expected non-nil enhanced analysis")
	}
	if !enhanced.Enabled {
		t.Fatalf("expected enhanced analysis to be enabled")
	}
	if len(enhanced.TableProfiles) != 1 {
		t.Fatalf("unexpected table profile count: got %d, want 1", len(enhanced.TableProfiles))
	}
	if enhanced.TableProfiles[0].TableName != "users" {
		t.Fatalf("unexpected table profile name: %s", enhanced.TableProfiles[0].TableName)
	}
}

func TestEnhanceSchemaAnalysis_DefaultWorkerCount(t *testing.T) {
	ctx := context.Background()
	tableSchemas := map[string]TableInfo{
		"users": {
			Columns: []SchemaColumnInfo{
				{ColumnName: "id", DataType: "integer"},
			},
		},
	}
	sampleData := map[string][]map[string]interface{}{
		"users": {{"id": 1}},
	}

	enhanced := enhanceSchemaAnalysis(ctx, tableSchemas, sampleData, 0)
	if enhanced == nil {
		t.Fatalf("expected non-nil enhanced analysis")
	}
	if enhanced.MaxWorkers != defaultProfilingWorkers {
		t.Fatalf("unexpected default max_workers: got %d, want %d", enhanced.MaxWorkers, defaultProfilingWorkers)
	}
}

func TestProfileTablesConcurrently_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	tableSchemas := map[string]TableInfo{
		"users": {
			Columns: []SchemaColumnInfo{{ColumnName: "id", DataType: "integer"}},
		},
	}
	sampleData := map[string][]map[string]interface{}{
		"users": {{"id": 1}},
	}

	profiles := profileTablesConcurrently(ctx, tableSchemas, sampleData, 1)
	if len(profiles) != 0 {
		t.Fatalf("expected no profiles after cancellation, got %d", len(profiles))
	}
}

func TestMergeWithExistingSchema(t *testing.T) {
	result := &AnalyzeSchemaResult{}
	enhanced := &EnhancedSchemaAnalysis{
		Enabled: true,
		TableProfiles: []TableProfile{
			{TableName: "users"},
		},
	}

	mergeWithExistingSchema(result, enhanced)
	if result.ColumnProfiling == nil {
		t.Fatalf("expected ColumnProfiling to be set")
	}
	if len(result.ColumnProfiling.TableProfiles) != 1 {
		t.Fatalf("unexpected merged table profiles: %d", len(result.ColumnProfiling.TableProfiles))
	}
}

func TestMergeWithExistingSchema_NilGuards(t *testing.T) {
	mergeWithExistingSchema(nil, &EnhancedSchemaAnalysis{Enabled: true})

	result := &AnalyzeSchemaResult{}
	mergeWithExistingSchema(result, nil)
	if result.ColumnProfiling != nil {
		t.Fatalf("expected ColumnProfiling to remain nil when enhanced payload is nil")
	}
}
