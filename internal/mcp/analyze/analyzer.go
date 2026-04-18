package analyze

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"
)

// analyzer.go provides the main Run() function that orchestrates schema analysis.
// This is called from the thin handleAnalyzeSchema wrapper in server.go.

// Run orchestrates a full schema analysis using only pure functions and database connections.
// It is database-agnostic and has no server dependencies.
func Run(ctx context.Context, db *sql.DB, dbType, schema string, params AnalyzeSchemaParams, tableNames []string) (*AnalyzeSchemaResult, error) {
	startTime := time.Now()

	sampleSize := NormalizeSampleSize(params.SampleSize)

	// 1. Fetch column metadata
	tableColumns, err := fetchAllColumns(ctx, db, dbType, schema, tableNames)
	if err != nil {
		return nil, fmt.Errorf("fetch columns: %w", err)
	}

	// 2. Build TableInfo map from column data
	tableSchemas := buildTableSchemas(tableColumns)

	// 3. Fetch row counts
	rowCounts, _ := FetchRowCounts(ctx, db, dbType, schema, tableNames)
	for tableName, info := range tableSchemas {
		if rc, ok := rowCounts[tableName]; ok {
			info.RowCount = rc
			tableSchemas[tableName] = info
		}
	}

	// 4. Fetch indexes
	indexes, _ := FetchIndexes(ctx, db, dbType, schema, tableNames)
	applyIndexesToSchemas(tableSchemas, indexes)

	// 5. Discover real foreign keys
	fks, _ := DiscoverForeignKeys(ctx, db, dbType, schema, tableNames)

	// 6. Detect implicit relationships
	implicitRels := DetectImplicitRelationships(tableColumns)

	// 7. Build relationship graph
	relGraph := buildRelationshipGraph(fks, implicitRels)

	// 8. Categorize tables
	tableCatalog := CategorizeTables(tableNames, tableSchemas)

	// 9. Compute row count estimates for table catalog
	applyRowCountToCatalog(&tableCatalog, rowCounts)

	// 10. Sample data + enrichment (detailed/comprehensive only)
	var sampleDataMap map[string][]map[string]interface{}
	var businessCtx *BusinessContext
	var domain string
	var confidence float64
	var businessDesc string

	if params.AnalysisLevel == AnalysisLevelDetailed || params.AnalysisLevel == AnalysisLevelComprehensive {
		sampleDataMap = fetchAllSampleRows(ctx, db, dbType, tableNames, sampleSize)

		// Apply data patterns to schemas
		tableSchemas = applyDataPatterns(tableSchemas, sampleDataMap)

		// Build quality metrics
		qualityMetrics := BuildQualityMetrics(params.AnalysisLevel, tableSchemas, sampleDataMap)

		// Performance optimization
		perfOpt := BuildPerformanceOptimization(tableColumns, fks, indexes)

		if params.AnalysisLevel == AnalysisLevelComprehensive {
			// Business context inference
			businessCtx = InferBusinessContext(tableSchemas)
			domain, confidence, businessDesc = summarizeBusinessContext(businessCtx)

			// Classification signals for LLM
			classificationSignals := buildClassificationSignals(tableNames, tableColumns, fks)

			result := &AnalyzeSchemaResult{
				AnalysisMetadata:        buildAnalysisMetadata(startTime, params, dbType, tableNames, tableSchemas),
				DatabaseOverview:        buildDatabaseOverview(tableNames, tableSchemas, relGraph, domain, confidence, businessDesc),
				TableCatalog:            tableCatalog,
				TableSchemas:            tableSchemas,
				RelationshipGraph:       relGraph,
				BusinessContext:         *businessCtx,
				DataQualityMetrics:      qualityMetrics,
				PerformanceOptimization: perfOpt,
				ClassificationSignals:   classificationSignals,
			}
			return result, nil
		}

		// Detailed level — no business context
		result := &AnalyzeSchemaResult{
			AnalysisMetadata:        buildAnalysisMetadata(startTime, params, dbType, tableNames, tableSchemas),
			DatabaseOverview:        buildDatabaseOverview(tableNames, tableSchemas, relGraph, "", 0, ""),
			TableCatalog:            tableCatalog,
			TableSchemas:            tableSchemas,
			RelationshipGraph:       relGraph,
			DataQualityMetrics:      qualityMetrics,
			PerformanceOptimization: perfOpt,
		}
		return result, nil
	}

	// Basic level — minimal output
	result := &AnalyzeSchemaResult{
		AnalysisMetadata:  buildAnalysisMetadata(startTime, params, dbType, tableNames, tableSchemas),
		DatabaseOverview:  buildDatabaseOverview(tableNames, tableSchemas, relGraph, "", 0, ""),
		TableCatalog:      tableCatalog,
		TableSchemas:      tableSchemas,
		RelationshipGraph: relGraph,
	}
	return result, nil
}

// fetchAllColumns gets column metadata for all tables.
// Uses bulk queries for MySQL/PostgreSQL, per-table for SQLite.
func fetchAllColumns(ctx context.Context, db *sql.DB, dbType, schema string, tableNames []string) (map[string][]SchemaColumnInfo, error) {
	switch dbType {
	case "mysql", "mariadb", "postgres", "postgresql":
		result, err := FetchColumnsBulk(ctx, db, dbType, schema)
		if err != nil {
			// Fallback to per-table
			return FetchColumnsPerTable(ctx, db, dbType, tableNames)
		}
		return result, nil
	case "sqlite":
		return FetchColumnsPerTable(ctx, db, dbType, tableNames)
	default:
		return nil, fmt.Errorf("unsupported db type: %s", dbType)
	}
}

// buildTableSchemas creates TableInfo entries from column metadata.
func buildTableSchemas(tableColumns map[string][]SchemaColumnInfo) map[string]TableInfo {
	schemas := make(map[string]TableInfo)
	for tableName, cols := range tableColumns {
		info := TableInfo{
			ColumnCount:  len(cols),
			Columns:      cols,
			DataPatterns: make(map[string]DataPattern),
		}
		// Extract key columns
		for _, col := range cols {
			if col.IsPrimaryKey {
				info.KeyColumns.PrimaryKey = col.ColumnName
			}
			if col.IsForeignKey {
				info.KeyColumns.ForeignKeys = append(info.KeyColumns.ForeignKeys, col.ColumnName)
			}
			if col.Unique && !col.IsPrimaryKey {
				info.KeyColumns.UniqueColumns = append(info.KeyColumns.UniqueColumns, col.ColumnName)
			}
			if col.Indexed && !col.IsPrimaryKey {
				info.KeyColumns.IndexedColumns = append(info.KeyColumns.IndexedColumns, col.ColumnName)
			}
		}
		schemas[tableName] = info
	}
	return schemas
}

// applyIndexesToSchemas adds index information to table schemas.
func applyIndexesToSchemas(schemas map[string]TableInfo, indexes []IndexInfo) {
	for _, idx := range indexes {
		info, ok := schemas[idx.TableName]
		if !ok {
			continue
		}
		if idx.IsPrimary {
			// Primary key already captured from column metadata
			continue
		}
		// Add indexed columns that aren't already captured
		for _, col := range idx.Columns {
			found := false
			for _, existing := range info.KeyColumns.IndexedColumns {
				if existing == col {
					found = true
					break
				}
			}
			if !found {
				info.KeyColumns.IndexedColumns = append(info.KeyColumns.IndexedColumns, col)
			}
		}
		schemas[idx.TableName] = info
	}
}

// buildRelationshipGraph combines FK and implicit relationships.
func buildRelationshipGraph(fks []ForeignKeyRelationship, implicitRels []SemanticRelationship) RelationshipGraph {
	// Build suggested joins from FKs
	for i := range fks {
		fk := &fks[i]
		join := fmt.Sprintf("SELECT * FROM %s JOIN %s ON %s.%s = %s.%s",
			fk.FromTable, fk.ToTable,
			fk.FromTable, fk.FromColumn,
			fk.ToTable, fk.ToColumn)
		fk.SuggestedJoin = join
	}
	// Build suggested joins from semantic relationships
	for i := range implicitRels {
		rel := &implicitRels[i]
		if len(rel.Tables) >= 2 && rel.FromColumn != "" && rel.ToColumn != "" {
			join := fmt.Sprintf("SELECT * FROM %s JOIN %s ON %s.%s = %s.%s",
				rel.Tables[0], rel.Tables[1],
				rel.Tables[0], rel.FromColumn,
				rel.Tables[1], rel.ToColumn)
			rel.SuggestedJoin = join
		}
	}
	return RelationshipGraph{
		ForeignKeys:           fks,
		SemanticRelationships: implicitRels,
	}
}

// applyRowCountToCatalog adds estimated row counts to table catalog entries.
func applyRowCountToCatalog(catalog *TableCatalog, rowCounts map[string]int64) {
	applyToEntities := func(entities []TableEntity) {
		for i, e := range entities {
			if rc, ok := rowCounts[e.TableName]; ok {
				entities[i].EstimatedRows = formatRowCount(rc)
			}
		}
	}
	applyToEntities(catalog.CoreEntities)
	applyToEntities(catalog.LookupTables)
	applyToEntities(catalog.JunctionTables)
	applyToEntities(catalog.AuditTables)
}

// fetchAllSampleRows fetches sample data for all tables.
func fetchAllSampleRows(ctx context.Context, db *sql.DB, dbType string, tableNames []string, sampleSize int) map[string][]map[string]interface{} {
	sampleDataMap := make(map[string][]map[string]interface{})
	for _, table := range tableNames {
		sampleDataMap[table] = FetchSampleRows(ctx, db, table, dbType, sampleSize)
	}
	return sampleDataMap
}

// applyDataPatterns runs pattern detection on sample data and updates schemas.
func applyDataPatterns(tableSchemas map[string]TableInfo, sampleDataMap map[string][]map[string]interface{}) map[string]TableInfo {
	for tableName, schema := range tableSchemas {
		sampleData := sampleDataMap[tableName]
		if len(sampleData) == 0 {
			continue
		}
		patterns := AnalyzeDataPatterns(tableName, sampleData, schema.Columns)
		updated := schema
		updated.DataPatterns = make(map[string]DataPattern)
		for idx, col := range schema.Columns {
			if idx >= len(patterns) {
				continue
			}
			pattern := patterns[idx]
			updated.DataPatterns[col.ColumnName] = pattern
			// Update column with pattern info
			cols := make([]SchemaColumnInfo, len(schema.Columns))
			copy(cols, schema.Columns)
			cols[idx].PatternType = pattern.PatternType
			cols[idx].ValidationRegex = pattern.ValidationRegex
			cols[idx].Uniqueness = pattern.Uniqueness
			cols[idx].NullPercentage = pattern.NullPercentage
			cols[idx].Distribution = pattern.Distribution
			updated.Columns = cols
		}
		tableSchemas[tableName] = updated
	}
	return tableSchemas
}

// summarizeBusinessContext extracts the dominant domain and description.
func summarizeBusinessContext(businessCtx *BusinessContext) (string, float64, string) {
	if businessCtx == nil {
		return "", 0, ""
	}
	domain := ""
	confidence := 0.0
	for candidateDomain, score := range businessCtx.DomainIndicators {
		if score > confidence || domain == "" {
			domain = candidateDomain
			confidence = score
		}
	}
	if domain == "" {
		return "", 0, ""
	}
	description := GenerateBusinessDescription(domain, businessCtx.EntityRelationships.CentralEntities, confidence)
	return domain, confidence, description
}

// buildClassificationSignals creates raw signals for LLM-based inference.
func buildClassificationSignals(tableNames []string, tableColumns map[string][]SchemaColumnInfo, fks []ForeignKeyRelationship) *ClassificationSignals {
	prefixes := make(map[string]int)
	var notableCols []string
	notableSet := make(map[string]bool)

	for _, cols := range tableColumns {
		for _, col := range cols {
			name := strings.ToLower(col.ColumnName)
			// Extract prefix (everything before first underscore)
			if idx := strings.Index(name, "_"); idx > 0 {
				prefix := name[:idx]
				if _, ok := commonColumns[prefix]; !ok {
					prefixes[prefix]++
				}
			}
			// Collect notable columns
			if notableColumnKeywords[name] && !notableSet[name] {
				notableCols = append(notableCols, name)
				notableSet[name] = true
			}
		}
	}

	// Build FK summary
	var fkParts []string
	for _, fk := range fks {
		fkParts = append(fkParts, fmt.Sprintf("%s.%s → %s.%s", fk.FromTable, fk.FromColumn, fk.ToTable, fk.ToColumn))
	}
	sort.Strings(fkParts)

	totalCols := 0
	for _, cols := range tableColumns {
		totalCols += len(cols)
	}

	return &ClassificationSignals{
		TableNames:     tableNames,
		NamingPrefixes: prefixes,
		NotableColumns: notableCols,
		FKSummary:      strings.Join(fkParts, "\n"),
		TotalTables:    len(tableNames),
		TotalColumns:   totalCols,
	}
}

// buildAnalysisMetadata creates metadata for the analysis result.
func buildAnalysisMetadata(startTime time.Time, params AnalyzeSchemaParams, dbType string, _ []string, _ map[string]TableInfo) AnalysisMetadata {
	return AnalysisMetadata{
		AnalysisLevel:      params.AnalysisLevel,
		DatabaseType:       dbType,
		AnalysisTimestamp:  startTime,
		ToolsUsed:          []string{"list-tables", "describe-table", "sample-data", "discover-joins"},
		AnalysisDurationMs: int(time.Since(startTime).Milliseconds()),
	}
}

// buildDatabaseOverview creates a high-level database summary.
func buildDatabaseOverview(tableNames []string, tableSchemas map[string]TableInfo, relGraph RelationshipGraph, _ string, confidence float64, businessDesc string) DatabaseOverview {
	totalCols := 0
	for _, info := range tableSchemas {
		totalCols += info.ColumnCount
	}
	totalRels := len(relGraph.ForeignKeys) + len(relGraph.SemanticRelationships)
	insights := []string{}
	if businessDesc != "" {
		insights = append(insights, businessDesc)
	}
	return DatabaseOverview{
		DatabaseCount:         1,
		TotalTables:           len(tableNames),
		TotalColumns:          totalCols,
		TotalRelationships:    totalRels,
		BusinessModelInsights: insights,
		Summary:               fmt.Sprintf("Analyzed %d tables and %d columns.", len(tableNames), totalCols),
	}
}

// formatRowCount formats a row count for display.
func formatRowCount(count int64) string {
	if count >= 1_000_000 {
		return fmt.Sprintf("~%dM", count/1_000_000)
	}
	if count >= 1_000 {
		return fmt.Sprintf("~%dk", count/1_000)
	}
	return fmt.Sprintf("%d", count)
}
