// Package analyze provides types for the analyze-schema MCP tool.
package analyze

import (
	"fmt"
	"time"
)

// Analysis levels for schema analysis.
//
// AnalysisLevel values:
//   - BASIC: Quick overview for initial exploration (minimal metadata, table list, key columns)
//   - DETAILED: Comprehensive schema for query construction (full table/column metadata, relationships, sample data)
//   - COMPREHENSIVE: Deep business context with AI insights (business domain inference, semantic relationships, KPIs, AI-generated documentation)
//
// AI agents MUST specify which analysis level they want. This parameter is REQUIRED and must be one of the above values.
const (
	AnalysisLevelBasic         = "basic"
	AnalysisLevelDetailed      = "detailed"
	AnalysisLevelComprehensive = "comprehensive"
)

/*
AnalyzeSchemaParams defines input parameters for the analyze-schema tool.

AnalysisLevel is REQUIRED and must be one of:
  - "basic"
  - "detailed"
  - "comprehensive"

If AnalysisLevel is empty or invalid, validation will fail.
*/
type AnalyzeSchemaParams struct {
	ProfileName    string   `json:"profile_name"`            // Required: Database profile to analyze
	DatabaseName   string   `json:"database_name,omitempty"` // Optional: Specific database (uses profile default if empty)
	Schema         string   `json:"schema,omitempty"`        // Optional: Specific schema (auto-detected if empty)
	AnalysisLevel  string   `json:"analysis_level"`          // REQUIRED: "basic", "detailed", "comprehensive"
	IncludeTables  []string `json:"include_tables,omitempty"`
	ExcludeTables  []string `json:"exclude_tables,omitempty"`
	SampleSize     int      `json:"sample_size,omitempty"`
	IncludeQueries bool     `json:"include_queries,omitempty"` // Optional: Generate query suggestions (default: true)
	Profiling      bool     `json:"profiling,omitempty"`       // Optional: Enable advanced profiling output
}

// Validate checks that required fields are present and valid.
func (p *AnalyzeSchemaParams) Validate() error {
	if p.ProfileName == "" {
		return fmt.Errorf("profile_name is required")
	}
	if p.AnalysisLevel == "" {
		return fmt.Errorf("analysis_level is required and must be one of: basic, detailed, comprehensive")
	}
	switch p.AnalysisLevel {
	case AnalysisLevelBasic, AnalysisLevelDetailed, AnalysisLevelComprehensive:
		// valid
	default:
		return fmt.Errorf("analysis_level must be one of: basic, detailed, comprehensive")
	}
	return nil
}

// AnalyzeSchemaResult is the main response structure for schema analysis.
type AnalyzeSchemaResult struct {
	AnalysisMetadata        AnalysisMetadata          `json:"analysis_metadata"`                   // Metadata about the analysis run
	DatabaseOverview        DatabaseOverview          `json:"database_overview"`                   // High-level database summary
	TableCatalog            TableCatalog              `json:"table_catalog,omitempty"`             // Categorized tables (basic/detailed)
	TableSchemas            map[string]TableInfo      `json:"table_schemas,omitempty"`             // Detailed table schemas (detailed/comprehensive)
	RelationshipGraph       RelationshipGraph         `json:"relationship_graph,omitempty"`        // Foreign key and semantic relationships
	RelationshipGraphVisual map[string]interface{}    `json:"relationship_graph_visual,omitempty"` // Visual/graph form for UI/AI consumption
	BusinessContext         BusinessContext           `json:"business_context,omitempty"`          // Inferred business context
	DataQualityMetrics      map[string]QualityMetrics `json:"data_quality_metrics,omitempty"`      // Data quality metrics per column/table
	AIQuerySuggestions      AIQuerySuggestions        `json:"ai_query_suggestions,omitempty"`      // AI-optimized query suggestions
	SemanticInsights        SemanticInsights          `json:"semantic_insights,omitempty"`         // Business processes, KPIs, etc.
	PerformanceOptimization PerformanceOptimization   `json:"performance_optimization,omitempty"`  // Index and query hints
	QuickInsights           []string                  `json:"quick_insights,omitempty"`            // Human-readable summary points
	ColumnProfiling         *EnhancedSchemaAnalysis   `json:"column_profiling,omitempty"`          // Optional advanced table/column profiling
	ClassificationSignals   *ClassificationSignals    `json:"classification_signals,omitempty"`    // Raw signals for LLM domain inference (replaces hardcoded domain/entity)
}

// AnalysisMetadata provides metadata about the analysis run.
type AnalysisMetadata struct {
	AnalysisLevel      string    `json:"analysis_level"`       // Level of analysis performed
	DatabaseType       string    `json:"database_type"`        // Type of database (e.g., mysql, postgres)
	AnalysisTimestamp  time.Time `json:"analysis_timestamp"`   // When analysis was performed
	ToolsUsed          []string  `json:"tools_used"`           // MCP tools used for analysis
	AnalysisDurationMs int       `json:"analysis_duration_ms"` // Duration in milliseconds
}

// DatabaseOverview summarizes the database at a high level.
type DatabaseOverview struct {
	DatabaseCount         int      `json:"database_count,omitempty"`            // Number of databases
	TotalTables           int      `json:"total_tables"`                        // Total tables analyzed
	TotalColumns          int      `json:"total_columns"`                       // Total columns analyzed
	TotalRelationships    int      `json:"total_relationships,omitempty"`       // Total relationships found
	BusinessModelInsights []string `json:"business_model_insights,omitempty"`   // Key business model findings
	Summary               string   `json:"summary,omitempty"`                   // Human-readable summary
}

// TableCatalog categorizes tables by business role.
type TableCatalog struct {
	CoreEntities   []TableEntity `json:"core_entities,omitempty"`   // Main business entities
	LookupTables   []TableEntity `json:"lookup_tables,omitempty"`   // Lookup/reference tables
	JunctionTables []TableEntity `json:"junction_tables,omitempty"` // Many-to-many join tables
	AuditTables    []TableEntity `json:"audit_tables,omitempty"`    // Audit/history tables
}

// TableEntity describes a table's business role.
type TableEntity struct {
	TableName     string `json:"table_name"`               // Table name
	ColumnCount   int    `json:"column_count,omitempty"`   // Number of columns
	PrimaryKey    string `json:"primary_key,omitempty"`    // Primary key column
	EstimatedRows string `json:"estimated_rows,omitempty"` // Estimated row count (e.g., "~50k")
}

// TableInfo provides detailed schema info for a table.
type TableInfo struct {
	ColumnCount  int                    `json:"column_count"`            // Number of columns
	KeyColumns   KeyColumns             `json:"key_columns"`             // Primary, foreign, unique, indexed columns
	DataPatterns map[string]DataPattern `json:"data_patterns,omitempty"` // Patterns detected in columns
	Columns      []SchemaColumnInfo     `json:"columns,omitempty"`       // Detailed column info
	RowCount     int64                  `json:"row_count,omitempty"`     // Estimated or actual row count
}

// KeyColumns lists key columns for a table.
type KeyColumns struct {
	PrimaryKey     string   `json:"primary_key,omitempty"`     // Primary key column
	ForeignKeys    []string `json:"foreign_keys,omitempty"`    // Foreign key columns
	UniqueColumns  []string `json:"unique_columns,omitempty"`  // Unique columns
	IndexedColumns []string `json:"indexed_columns,omitempty"` // Indexed columns
}

// SchemaColumnInfo describes a single column for schema analysis.
type SchemaColumnInfo struct {
	ColumnName      string         `json:"column_name"`                // Column name
	DataType        string         `json:"data_type"`                  // SQL data type
	IsNullable      bool           `json:"is_nullable"`                // Nullable flag
	IsPrimaryKey    bool           `json:"is_primary_key,omitempty"`   // Is primary key
	IsForeignKey    bool           `json:"is_foreign_key,omitempty"`   // Is foreign key
	ForeignKeyRef   *ForeignKeyRef `json:"foreign_key_ref,omitempty"`  // Foreign key reference
	Unique          bool           `json:"unique,omitempty"`           // Is unique
	Indexed         bool           `json:"indexed,omitempty"`          // Is indexed
	DefaultValue    interface{}    `json:"default_value,omitempty"`    // Default value
	MaxLength       int            `json:"max_length,omitempty"`       // Max length for strings
	EnumValues      []string       `json:"enum_values,omitempty"`      // Enum values if applicable
	NullPercentage  float64        `json:"null_percentage,omitempty"`  // Percentage of nulls
	Uniqueness      float64        `json:"uniqueness,omitempty"`       // Uniqueness ratio
	Distribution    string         `json:"distribution,omitempty"`     // Value distribution type
	PatternType     string         `json:"pattern_type,omitempty"`     // Detected pattern type (e.g., email, uuid)
	ValidationRegex string         `json:"validation_regex,omitempty"` // Regex for validation
	Description     string         `json:"description,omitempty"`      // Natural language description
}

// ForeignKeyRef describes a foreign key reference.
type ForeignKeyRef struct {
	RefTable  string `json:"ref_table"`  // Referenced table
	RefColumn string `json:"ref_column"` // Referenced column
}

// DataPattern describes detected data patterns in a column.
type DataPattern struct {
	PatternType     string      `json:"pattern_type"`               // Type of pattern (e.g., email, currency)
	ValidationRegex string      `json:"validation_regex,omitempty"` // Regex for validation
	Uniqueness      float64     `json:"uniqueness,omitempty"`       // Uniqueness ratio
	NullPercentage  float64     `json:"null_percentage,omitempty"`  // Percentage of nulls
	DecimalPlaces   int         `json:"decimal_places,omitempty"`   // Decimal places for numeric types
	Range           *ValueRange `json:"range,omitempty"`            // Min/max values
	Distribution    string      `json:"distribution,omitempty"`     // Distribution type
	Values          []string    `json:"values,omitempty"`           // Enum or distinct values
}

// ValueRange represents a min/max range.
type ValueRange struct {
	Min interface{} `json:"min"`
	Max interface{} `json:"max"`
}

// RelationshipGraph describes relationships between tables.
type RelationshipGraph struct {
	ForeignKeys           []ForeignKeyRelationship `json:"foreign_keys,omitempty"`           // Foreign key relationships
	SemanticRelationships []SemanticRelationship   `json:"semantic_relationships,omitempty"` // Inferred semantic relationships
	SuggestedJoins        []string                 `json:"suggested_joins,omitempty"`        // Suggested join SQL snippets
}

// ForeignKeyRelationship describes a foreign key relationship.
type ForeignKeyRelationship struct {
	FromTable        string `json:"from_table"`               // Source table
	FromColumn       string `json:"from_column"`              // Source column
	ToTable          string `json:"to_table"`                 // Target table
	ToColumn         string `json:"to_column"`                // Target column
	RelationshipType string `json:"relationship_type"`        // e.g., "many_to_one"
	SuggestedJoin    string `json:"suggested_join,omitempty"` // SQL join suggestion
}

// SemanticRelationship describes an inferred relationship.
type SemanticRelationship struct {
	Tables           []string `json:"tables"`                     // Tables involved
	RelationshipType string   `json:"relationship_type"`          // e.g., "one_to_many"
	ConnectionBasis  string   `json:"connection_basis"`           // Basis for inference (e.g., naming_convention)
	ConfidenceScore  float64  `json:"confidence_score"`           // Confidence in relationship
	BusinessMeaning  string   `json:"business_meaning,omitempty"` // Natural language explanation
	SuggestedJoin    string   `json:"suggested_join,omitempty"`   // SQL join suggestion
	FromColumn       string   `json:"from_column,omitempty"`      // Source column (for implicit matches)
	ToColumn         string   `json:"to_column,omitempty"`        // Target column (for implicit matches)
}

// BusinessContext provides inferred business context.
type BusinessContext struct {
	DomainIndicators    map[string]float64     `json:"domain_indicators,omitempty"`    // Domain scores
	NamingConventions   NamingConventions      `json:"naming_conventions,omitempty"`   // Naming pattern analysis
	EntityRelationships EntityRelationships    `json:"entity_relationships,omitempty"` // Central entities and relationship density
	DataPatterns        map[string]DataPattern `json:"data_patterns,omitempty"`        // Global data patterns
}

// NamingConventions describes naming patterns.
type NamingConventions struct {
	Pattern           string   `json:"pattern,omitempty"`             // e.g., "snake_case"
	ConsistencyScore  float64  `json:"consistency_score,omitempty"`   // Consistency of naming
	ForeignKeyPattern string   `json:"foreign_key_pattern,omitempty"` // e.g., "*_id"
	AuditColumns      []string `json:"audit_columns,omitempty"`       // Audit columns present
}

// EntityRelationships describes central entities and density.
type EntityRelationships struct {
	CentralEntities      []string `json:"central_entities,omitempty"`       // Main entities
	RelationshipDensity  float64  `json:"relationship_density,omitempty"`   // Density of relationships
	MaxRelationshipDepth int      `json:"max_relationship_depth,omitempty"` // Max depth of relationships
}

// QualityMetrics describes data quality for a column/table.
type QualityMetrics struct {
	Completeness           float64  `json:"completeness,omitempty"`             // Ratio of non-null values
	Uniqueness             float64  `json:"uniqueness,omitempty"`               // Ratio of unique values
	Validity               float64  `json:"validity,omitempty"`                 // Ratio of valid values
	Consistency            float64  `json:"consistency,omitempty"`              // Consistency score
	TemporalConsistency    float64  `json:"temporal_consistency,omitempty"`     // For date/time columns
	BusinessRuleCompliance float64  `json:"business_rule_compliance,omitempty"` // Compliance with business rules
	OverallScore           float64  `json:"overall_score,omitempty"`            // Aggregate quality score
	Issues                 []string `json:"issues,omitempty"`                   // List of detected issues
	QualityIssues          []string `json:"quality_issues,omitempty"`           // Deprecated: use Issues
}

// AIQuerySuggestions provides AI-optimized query suggestions.
type AIQuerySuggestions struct {
	DataExploration      []QuerySuggestion `json:"data_exploration,omitempty"`      // Data exploration queries
	BusinessIntelligence []QuerySuggestion `json:"business_intelligence,omitempty"` // BI queries
	OperationalQueries   []QuerySuggestion `json:"operational_queries,omitempty"`   // Operational queries
	ComplianceQueries    []QuerySuggestion `json:"compliance_queries,omitempty"`    // Compliance queries
	PerformanceOptimized []QuerySuggestion `json:"performance_optimized,omitempty"` // Performance-optimized queries
	BusinessQuestions    []QuerySuggestion `json:"business_questions,omitempty"`    // Business question queries
	CommonPatterns       []string          `json:"common_patterns,omitempty"`       // Common query patterns
}

// QuerySuggestion describes a suggested query.
type QuerySuggestion struct {
	Category          string `json:"category,omitempty"`           // Query category
	Question          string `json:"question"`                     // Natural language question
	SQL               string `json:"sql"`                          // SQL query
	Complexity        string `json:"complexity,omitempty"`         // Query complexity (e.g., "easy", "medium")
	EstimatedRows     int    `json:"estimated_rows,omitempty"`     // Estimated rows returned
	VisualizationHint string `json:"visualization_hint,omitempty"` // Suggested visualization
	BusinessValue     string `json:"business_value,omitempty"`     // Business value of query
	Urgency           string `json:"urgency,omitempty"`            // Urgency for compliance queries
	PerformanceHint   string `json:"performance_hint,omitempty"`   // Performance optimization hint
}

// SemanticInsights provides business processes and KPIs.
type SemanticInsights struct {
	BusinessProcesses []BusinessProcess `json:"business_processes,omitempty"` // Business process insights
	KPISuggestions    []KPI             `json:"kpi_suggestions,omitempty"`    // KPI suggestions
}

// BusinessProcess describes a business process.
type BusinessProcess struct {
	Process                   string   `json:"process"`                              // Process name
	TablesInvolved            []string `json:"tables_involved"`                      // Tables involved in process
	TypicalWorkflow           string   `json:"typical_workflow,omitempty"`           // Typical workflow steps
	AutomationOpportunities   []string `json:"automation_opportunities,omitempty"`   // Automation opportunities
	OptimizationOpportunities []string `json:"optimization_opportunities,omitempty"` // Optimization opportunities
}

// KPI describes a key performance indicator.
type KPI struct {
	KPI         string `json:"kpi"`                 // KPI name
	Calculation string `json:"calculation"`         // Calculation SQL or formula
	Benchmark   string `json:"benchmark,omitempty"` // Benchmark value/range
}

// PerformanceOptimization provides index and query hints.
type PerformanceOptimization struct {
	RecommendedIndexes []RecommendedIndex `json:"recommended_indexes,omitempty"` // Index recommendations
	QueryPatterns      QueryPatterns      `json:"query_patterns,omitempty"`      // Query pattern hints
}

// RecommendedIndex describes an index recommendation.
type RecommendedIndex struct {
	Table   string   `json:"table"`            // Table name
	Columns []string `json:"columns"`          // Columns to index
	Reason  string   `json:"reason,omitempty"` // Reason for recommendation
}

// QueryPatterns provides query pattern hints.
type QueryPatterns struct {
	Avoid  []string `json:"avoid,omitempty"`  // Patterns to avoid
	Prefer []string `json:"prefer,omitempty"` // Patterns to prefer
}

// ClassificationSignals provides raw signals for LLM-based domain/entity inference.
// Replaces hardcoded domain dictionary (BUG-007) and entity taxonomy (BUG-008).
type ClassificationSignals struct {
	TableNames     []string       `json:"table_names"`      // All table names
	NamingPrefixes map[string]int `json:"naming_prefixes"`  // Prefix frequency (e.g., "call_": 3, "broadcast_": 5)
	NotableColumns []string       `json:"notable_columns"`  // Domain-significant column names
	FKSummary      string         `json:"fk_summary"`       // Summary of FK relationships
	TotalTables    int            `json:"total_tables"`     // Total number of tables
	TotalColumns   int            `json:"total_columns"`    // Total number of columns
}

// EnhancedSchemaAnalysis represents the output of advanced column profiling.
// Defined in profiling_types.go but referenced here.
type EnhancedSchemaAnalysis struct {
	TableProfiles map[string]*TableProfile `json:"table_profiles,omitempty"`
}

// TableProfile represents per-table profiling output.
type TableProfile struct {
	TableName string          `json:"table_name"`
	Columns   []ColumnProfile `json:"columns,omitempty"`
}

// ColumnProfile represents per-column profiling output.
type ColumnProfile struct {
	ColumnName string           `json:"column_name"`
	Statistics ColumnStatistics `json:"statistics,omitempty"`
}

// ColumnStatistics represents statistical profiling for a column.
type ColumnStatistics struct {
	NonNullCount int     `json:"non_null_count,omitempty"`
	NullCount    int     `json:"null_count,omitempty"`
	DistinctCount int    `json:"distinct_count,omitempty"`
	MinValue     interface{} `json:"min_value,omitempty"`
	MaxValue     interface{} `json:"max_value,omitempty"`
	AvgValue     float64 `json:"avg_value,omitempty"`
}
