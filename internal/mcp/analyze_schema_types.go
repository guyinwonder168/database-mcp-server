// Package mcp provides type aliases for the analyze-schema MCP tool.
// All types are defined in the analyze package; this file re-exports them
// for backward compatibility with existing server code and tests.
//
// Profiling types (EnhancedSchemaAnalysis, TableProfile, ColumnProfile,
// ColumnStatistics) are NOT aliased — they are defined in profiling_types.go
// with different fields used by the schema profiling engine.
package mcp

import (
	"database-mcp-provider/internal/mcp/analyze"
)

// Analysis level constants — re-exported from analyze package.
const (
	AnalysisLevelBasic         = analyze.AnalysisLevelBasic
	AnalysisLevelDetailed      = analyze.AnalysisLevelDetailed
	AnalysisLevelComprehensive = analyze.AnalysisLevelComprehensive
)

// Type aliases — all types live in the analyze package.
type AnalyzeSchemaParams = analyze.AnalyzeSchemaParams
type AnalysisMetadata = analyze.AnalysisMetadata
type DatabaseOverview = analyze.DatabaseOverview
type TableCatalog = analyze.TableCatalog
type TableEntity = analyze.TableEntity
type TableInfo = analyze.TableInfo
type KeyColumns = analyze.KeyColumns
type SchemaColumnInfo = analyze.SchemaColumnInfo
type ForeignKeyRef = analyze.ForeignKeyRef
type DataPattern = analyze.DataPattern
type ValueRange = analyze.ValueRange
type RelationshipGraph = analyze.RelationshipGraph
type ForeignKeyRelationship = analyze.ForeignKeyRelationship
type SemanticRelationship = analyze.SemanticRelationship
type BusinessContext = analyze.BusinessContext
type NamingConventions = analyze.NamingConventions
type EntityRelationships = analyze.EntityRelationships
type QualityMetrics = analyze.QualityMetrics
type AIQuerySuggestions = analyze.AIQuerySuggestions
type QuerySuggestion = analyze.QuerySuggestion
type SemanticInsights = analyze.SemanticInsights
type BusinessProcess = analyze.BusinessProcess
type KPI = analyze.KPI
type PerformanceOptimization = analyze.PerformanceOptimization
type RecommendedIndex = analyze.RecommendedIndex
type QueryPatterns = analyze.QueryPatterns
type ClassificationSignals = analyze.ClassificationSignals

// AnalyzeSchemaResult is the main response structure for schema analysis.
// Defined here (not aliased) because ColumnProfiling uses the profiling
// types from profiling_types.go, which differ from the analyze package's version.
type AnalyzeSchemaResult struct {
	AnalysisMetadata        AnalysisMetadata          `json:"analysis_metadata"`
	DatabaseOverview        DatabaseOverview          `json:"database_overview"`
	TableCatalog            TableCatalog              `json:"table_catalog,omitempty"`
	TableSchemas            map[string]TableInfo      `json:"table_schemas,omitempty"`
	RelationshipGraph       RelationshipGraph         `json:"relationship_graph,omitempty"`
	RelationshipGraphVisual map[string]interface{}    `json:"relationship_graph_visual,omitempty"`
	BusinessContext         BusinessContext           `json:"business_context,omitempty"`
	DataQualityMetrics      map[string]QualityMetrics `json:"data_quality_metrics,omitempty"`
	AIQuerySuggestions      AIQuerySuggestions        `json:"ai_query_suggestions,omitempty"`
	SemanticInsights        SemanticInsights          `json:"semantic_insights,omitempty"`
	PerformanceOptimization PerformanceOptimization   `json:"performance_optimization,omitempty"`
	QuickInsights           []string                  `json:"quick_insights,omitempty"`
	ColumnProfiling         *EnhancedSchemaAnalysis   `json:"column_profiling,omitempty"`
	ClassificationSignals   *ClassificationSignals    `json:"classification_signals,omitempty"`
	Warnings                []string                  `json:"warnings,omitempty"` // Non-fatal warnings (e.g., privilege issues)
}
