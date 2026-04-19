package analyze

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
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
			FKSummary:      "none",
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

// --- Business Context Inference (moved from server.go, converted to pure functions) ---

// flattenTableSchemas converts a map of table schemas into parallel slices.
func flattenTableSchemas(tableSchemas map[string]TableInfo) ([]string, []TableInfo) {
	tableNames := make([]string, 0, len(tableSchemas))
	tables := make([]TableInfo, 0, len(tableSchemas))
	for name, table := range tableSchemas {
		tableNames = append(tableNames, name)
		tables = append(tables, table)
	}
	return tableNames, tables
}

// InferBusinessContext analyzes table schemas to produce raw domain signals,
// naming conventions, and entity type hints. Pure function — no server dependencies.
// DomainIndicators contains naming prefix frequencies (not authoritative classifications).
// The calling LLM should interpret these signals using its own world knowledge.
func InferBusinessContext(tableSchemas map[string]TableInfo) *BusinessContext {
	tableNames, tables := flattenTableSchemas(tableSchemas)
	domainSignals := ComputeDomainSignals(tableNames)
	naming := AnalyzeNamingConventions(tables)
	entities := IdentifyEntityTypes(tableNames)

	pattern := NamingValueString(naming, "main_case", "unknown")
	consistencyScore := NamingValueFloat(naming, "consistency", 0.0)
	fkPattern := NamingValueString(naming, "fk_pattern", "unknown")
	auditCols := NamingValueStringSlice(naming, "timestampCols")

	return &BusinessContext{
		DomainIndicators: domainSignals,
		NamingConventions: NamingConventions{
			Pattern:           pattern,
			ConsistencyScore:  consistencyScore,
			ForeignKeyPattern: fkPattern,
			AuditColumns:      auditCols,
		},
		EntityRelationships: EntityRelationships{
			CentralEntities:      entities,
			RelationshipDensity:  0.0,
			MaxRelationshipDepth: 0,
		},
		DataPatterns: map[string]DataPattern{},
	}
}

// ComputeDomainSignals extracts naming prefix frequencies from table names.
// Returns a map of prefix→count where prefix is the first underscore-delimited
// segment of each table name (e.g., "order_items" → "order": 1).
// This provides raw signals for the calling LLM to interpret the domain,
// rather than hardcoding domain classifications.
func ComputeDomainSignals(tableNames []string) map[string]float64 {
	signals := make(map[string]float64)
	for _, name := range tableNames {
		lower := strings.ToLower(name)
		// Extract the first segment as the primary naming prefix
		if idx := strings.Index(lower, "_"); idx > 0 {
			prefix := lower[:idx]
			signals[prefix]++
		} else {
			// Single-word table name — use the whole name
			signals[lower]++
		}
	}
	if len(signals) == 0 {
		signals["unknown"] = 0
	}
	return signals
}

// AnalyzeNamingConventions examines column names to determine naming patterns,
// FK conventions, and audit column presence.
func AnalyzeNamingConventions(tables []TableInfo) map[string]interface{} {
	accumulator := newNamingAccumulator()
	for _, table := range tables {
		for _, column := range table.Columns {
			accumulator.consume(column.ColumnName)
		}
	}
	mainCase, consistency := dominantCase(accumulator.cases)
	fkPattern := ClassifyForeignKeyPattern(accumulator.fkSuffix, accumulator.fkPrefix, accumulator.fkTotal)
	return map[string]interface{}{
		"cases":         accumulator.cases,
		"prefixes":      accumulator.prefixes,
		"suffixes":      accumulator.suffixes,
		"timestampCols": accumulator.timestampCols,
		"main_case":     mainCase,
		"consistency":   consistency,
		"fk_pattern":    fkPattern,
	}
}

// namingAccumulator collects naming statistics from column names.
type namingAccumulator struct {
	cases         map[string]int
	prefixes      map[string]int
	suffixes      map[string]int
	timestampCols []string
	fkSuffix      int
	fkPrefix      int
	fkTotal       int
}

func newNamingAccumulator() *namingAccumulator {
	return &namingAccumulator{
		cases:    map[string]int{"snake_case": 0, "camelCase": 0, "PascalCase": 0},
		prefixes: map[string]int{},
		suffixes: map[string]int{},
	}
}

func (a *namingAccumulator) consume(name string) {
	recordCaseType(a.cases, name)
	RecordPrefixAndSuffix(a.prefixes, a.suffixes, name)
	if isTimestampColumn(name) {
		a.timestampCols = append(a.timestampCols, name)
	}
	a.fkSuffix, a.fkPrefix, a.fkTotal = UpdateForeignKeyPatternCounts(name, a.fkSuffix, a.fkPrefix, a.fkTotal)
}

func recordCaseType(caseCounts map[string]int, name string) {
	if strings.Contains(name, "_") {
		caseCounts["snake_case"]++
		return
	}
	if len(name) > 1 && unicode.IsUpper(rune(name[0])) {
		caseCounts["PascalCase"]++
		return
	}
	if len(name) > 1 && unicode.IsLower(rune(name[0])) && strings.IndexFunc(name, unicode.IsUpper) > 0 {
		caseCounts["camelCase"]++
	}
}

func RecordPrefixAndSuffix(prefixes, suffixes map[string]int, name string) {
	parts := strings.Split(name, "_")
	if len(parts) <= 1 {
		return
	}
	prefixes[parts[0]]++
	suffixes[parts[len(parts)-1]]++
}

func isTimestampColumn(name string) bool {
	return strings.HasSuffix(name, "created_at") ||
		strings.HasSuffix(name, "updated_at") ||
		strings.HasSuffix(name, "timestamp") ||
		strings.HasSuffix(name, "created") ||
		strings.HasSuffix(name, "modified")
}

func UpdateForeignKeyPatternCounts(name string, fkSuffix, fkPrefix, fkTotal int) (int, int, int) {
	if strings.HasSuffix(name, "_id") && len(name) > 3 {
		return fkSuffix + 1, fkPrefix, fkTotal + 1
	}
	if strings.HasPrefix(name, "id_") && len(name) > 3 {
		return fkSuffix, fkPrefix + 1, fkTotal + 1
	}
	return fkSuffix, fkPrefix, fkTotal
}

func dominantCase(caseCounts map[string]int) (string, float64) {
	totalCaseCount := caseCounts["snake_case"] + caseCounts["camelCase"] + caseCounts["PascalCase"]
	if totalCaseCount == 0 {
		return "unknown", 0.0
	}
	mainCase := "unknown"
	maxCount := 0
	for key, count := range caseCounts {
		if count > maxCount {
			mainCase = key
			maxCount = count
		}
	}
	return mainCase, float64(maxCount) / float64(totalCaseCount)
}

func ClassifyForeignKeyPattern(fkSuffix, fkPrefix, fkTotal int) string {
	if fkTotal == 0 {
		return "none"
	}
	switch {
	case fkSuffix > 0 && fkPrefix > 0:
		return "mixed"
	case fkSuffix > 0:
		return "suffix"
	case fkPrefix > 0:
		return "prefix"
	default:
		return "none"
	}
}

// IdentifyEntityTypes classifies each table name into a business entity category.
func IdentifyEntityTypes(tableNames []string) []string {
	types := make([]string, 0, len(tableNames))
	for _, name := range tableNames {
		lower := strings.ToLower(name)
		switch {
		case strings.Contains(lower, "log") || strings.Contains(lower, "audit"):
			types = append(types, "log")
		case strings.Contains(lower, "lookup") || strings.HasSuffix(lower, "_type") || strings.HasSuffix(lower, "_status"):
			types = append(types, "lookup")
		case strings.Contains(lower, "transaction") || strings.Contains(lower, "order") || strings.Contains(lower, "invoice"):
			types = append(types, "transactional")
		case strings.Contains(lower, "user") || strings.Contains(lower, "product") || strings.Contains(lower, "customer"):
			types = append(types, "master_data")
		default:
			types = append(types, "other")
		}
	}
	return types
}

// GenerateBusinessDescription produces a human-readable summary of the detected
// naming signals and entity composition. The description describes raw signals
// rather than claiming a definitive domain.
func GenerateBusinessDescription(_ string, _ float64, entities []string, allSignals map[string]float64) string {
	entitySummary := map[string]int{}
	for _, e := range entities {
		entitySummary[e]++
	}
	entityDesc := make([]string, 0, len(entitySummary))
	for k, v := range entitySummary {
		entityDesc = append(entityDesc, fmt.Sprintf("%d %s tables", v, k))
	}

	// Build top signals description (up to 5 most common)
	type sig struct {
		prefix string
		count  float64
	}
	var top []sig
	for p, c := range allSignals {
		top = append(top, sig{p, c})
	}
	sort.Slice(top, func(i, j int) bool { return top[i].count > top[j].count })
	if len(top) > 5 {
		top = top[:5]
	}
	sigParts := make([]string, 0, len(top))
	for _, s := range top {
		sigParts = append(sigParts, fmt.Sprintf("%s: %.0f", s.prefix, s.count))
	}

	if len(sigParts) > 0 {
		return fmt.Sprintf("Top naming signals: %s. Entity breakdown: %s. Interpret domain using these signals.",
			strings.Join(sigParts, ", "), strings.Join(entityDesc, ", "))
	}
	return fmt.Sprintf("Entity breakdown: %s. No strong naming signals detected.",
		strings.Join(entityDesc, ", "))
}

// --- Helper functions for naming value extraction ---

func NamingValueString(values map[string]interface{}, key, fallback string) string {
	if value, ok := values[key]; ok {
		if asString, ok := value.(string); ok {
			return asString
		}
	}
	return fallback
}

func NamingValueFloat(values map[string]interface{}, key string, fallback float64) float64 {
	value, ok := values[key]
	if !ok {
		return fallback
	}
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	default:
		return fallback
	}
}

func NamingValueStringSlice(values map[string]interface{}, key string) []string {
	value, ok := values[key]
	if !ok {
		return []string{}
	}
	switch typed := value.(type) {
	case []string:
		return typed
	case []interface{}:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			out = append(out, fmt.Sprintf("%v", item))
		}
		return out
	default:
		return []string{}
	}
}

// --- Data Pattern Recognition (moved from server.go, converted to pure functions) ---

// AnalyzeDataPatterns analyzes sample data for each column to detect patterns.
func AnalyzeDataPatterns(tableName string, sampleData []map[string]interface{}, columns []SchemaColumnInfo) []DataPattern {
	patterns := make([]DataPattern, len(columns))
	for i, col := range columns {
		values := collectValuesForColumn(sampleData, col.ColumnName)
		pattern := DetectColumnPattern(tableName, col.ColumnName, values)
		if pattern != nil {
			patterns[i] = *pattern
		}
	}
	return patterns
}

// collectValuesForColumn extracts values for a single column from sample rows.
func collectValuesForColumn(sampleData []map[string]interface{}, columnName string) []interface{} {
	var values []interface{}
	for _, row := range sampleData {
		if v, ok := row[columnName]; ok {
			values = append(values, v)
		}
	}
	return values
}

// DetectColumnPattern analyzes individual column data to detect value distribution,
// patterns, ranges, and examples. Pure function — no server dependencies.
func DetectColumnPattern(tableName string, columnName string, values []interface{}) *DataPattern {
	dist := AnalyzeValueDistribution(values)
	acc := newColumnPatternAccumulator(DetectDataType(values))
	for _, value := range values {
		acc.consume(value)
	}
	return acc.buildPattern(values, dist)
}

// columnPatternAccumulator collects statistics while scanning column values.
type columnPatternAccumulator struct {
	patternType     string
	validationRegex string
	nullCount       int
	uniqueSet       map[string]struct{}
	min             float64
	max             float64
	minSet          bool
	maxSet          bool
	decimalPlaces   int
}

func newColumnPatternAccumulator(semanticType string) *columnPatternAccumulator {
	return &columnPatternAccumulator{
		patternType: semanticType,
		uniqueSet:   map[string]struct{}{},
	}
}

func (a *columnPatternAccumulator) consume(value interface{}) {
	if value == nil {
		a.nullCount++
		return
	}
	strVal := fmt.Sprintf("%v", value)
	a.uniqueSet[strVal] = struct{}{}
	a.consumeNumeric(value, strVal)
	a.consumeSemanticPattern(value)
}

func (a *columnPatternAccumulator) consumeNumeric(value interface{}, strValue string) {
	switch value.(type) {
	case int, int32, int64, float32, float64:
		floatValue, err := toFloat64(value)
		if err != nil {
			return
		}
		if !a.minSet || floatValue < a.min {
			a.min = floatValue
			a.minSet = true
		}
		if !a.maxSet || floatValue > a.max {
			a.max = floatValue
			a.maxSet = true
		}
		dp := countDecimalPlaces(strValue)
		if dp > a.decimalPlaces {
			a.decimalPlaces = dp
		}
	}
}

func (a *columnPatternAccumulator) consumeSemanticPattern(value interface{}) {
	textValue, ok := value.(string)
	if !ok {
		return
	}
	regexes := SemanticTypeRegexes()
	for _, patternType := range semanticTypeOrder {
		regex, ok := regexes[patternType]
		if !ok {
			continue
		}
		if matchRegex(regex, textValue) {
			a.patternType = patternType
			a.validationRegex = regex
			return
		}
	}
}

func (a *columnPatternAccumulator) buildPattern(values []interface{}, dist map[string]interface{}) *DataPattern {
	return &DataPattern{
		PatternType:     a.patternType,
		ValidationRegex: a.validationRegex,
		Uniqueness:      ratio(float64(len(a.uniqueSet)), float64(len(values))),
		NullPercentage:  ratio(float64(a.nullCount), float64(len(values))),
		DecimalPlaces:   a.decimalPlaces,
		Range:           a.valueRange(),
		Distribution:    distributionType(dist),
		Values:          enumValuesFromSet(a.uniqueSet),
	}
}

func (a *columnPatternAccumulator) valueRange() *ValueRange {
	if !a.minSet || !a.maxSet {
		return nil
	}
	return &ValueRange{Min: a.min, Max: a.max}
}

// semanticTypeOrder defines the order to check semantic types.
// More specific patterns must come before broader ones to avoid false matches.
var semanticTypeOrder = []string{"uuid", "date", "email", "phone", "url", "currency", "id"}

// SemanticTypeRegexes returns regex patterns for semantic type detection.
func SemanticTypeRegexes() map[string]string {
	return map[string]string{
		"email":    `^[\w\.\-]+@[\w\.\-]+\.\w+$`,
		"phone":    `^\+?\d[\d\-\s]{7,}$`,
		"url":      `^https?://[^\s]+$`,
		"id":       `^[a-zA-Z0-9_\-]{8,}$`,
		"uuid":     `^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$`,
		"date":     `^\d{4}-\d{2}-\d{2}`,
		"currency": `^\$?\d+(\.\d{2})?$`,
	}
}

// enumValuesFromSet converts a unique value set to a sorted slice.
// Returns nil for empty sets or sets with more than 10 values.
func enumValuesFromSet(uniqueSet map[string]struct{}) []string {
	if len(uniqueSet) == 0 || len(uniqueSet) > 10 {
		return nil
	}
	values := make([]string, 0, len(uniqueSet))
	for value := range uniqueSet {
		values = append(values, value)
	}
	return values
}

// distributionType extracts the distribution string from stats map.
func distributionType(dist map[string]interface{}) string {
	if value, ok := dist["distribution"].(string); ok {
		return value
	}
	return "unknown"
}

// ratio safely divides numerator by denominator, returning 0 for zero denominator.
func ratio(numerator, denominator float64) float64 {
	if denominator == 0 {
		return 0
	}
	return numerator / denominator
}

// AnalyzeValueDistribution calculates statistics: unique values, null count, most common values.
func AnalyzeValueDistribution(values []interface{}) map[string]interface{} {
	stats := map[string]interface{}{}
	counts := map[string]int{}
	nullCount := 0
	for _, v := range values {
		if v == nil {
			nullCount++
			continue
		}
		counts[fmt.Sprintf("%v", v)]++
	}
	stats["unique_count"] = len(counts)
	stats["null_count"] = nullCount
	stats["most_common"] = mostCommonValues(counts, 3)
	stats["distribution"] = classifyDistribution(len(counts), len(values))
	return stats
}

// mostCommonValues returns the top N most frequent values.
func mostCommonValues(counts map[string]int, n int) []string {
	type kv struct {
		Key   string
		Value int
	}
	freq := make([]kv, 0, len(counts))
	for k, v := range counts {
		freq = append(freq, kv{k, v})
	}
	sort.Slice(freq, func(i, j int) bool { return freq[i].Value > freq[j].Value })
	common := make([]string, 0, n)
	for i := 0; i < len(freq) && i < n; i++ {
		common = append(common, freq[i].Key)
	}
	return common
}

// classifyDistribution determines if values are constant, categorical, or variable.
func classifyDistribution(uniqueCount, totalCount int) string {
	if uniqueCount == 1 {
		return "constant"
	}
	if uniqueCount < totalCount/2 {
		return "categorical"
	}
	return "variable"
}

// DetectDataType infers semantic data type beyond SQL type using regex matching.
// Uses ordered pattern checking so specific types (uuid, date) are checked before
// broader ones (id) to avoid false matches.
func DetectDataType(values []interface{}) string {
	regexes := SemanticTypeRegexes()
	typeCounts := map[string]int{}
	for _, v := range values {
		if v == nil {
			continue
		}
		strVal := fmt.Sprintf("%v", v)
		// Check in order — first match wins for this value
		for _, typ := range semanticTypeOrder {
			rx, ok := regexes[typ]
			if !ok {
				continue
			}
			if matchRegex(rx, strVal) {
				typeCounts[typ]++
				break
			}
		}
	}
	maxType, maxCount := mostFrequentType(typeCounts)
	if maxType != "" && ratio(float64(maxCount), float64(len(values))) > 0.5 {
		return maxType
	}
	return "unknown"
}

// mostFrequentType returns the type with the highest count.
func mostFrequentType(typeCounts map[string]int) (string, int) {
	maxType := ""
	maxCount := 0
	for typ, cnt := range typeCounts {
		if cnt > maxCount {
			maxType = typ
			maxCount = cnt
		}
	}
	return maxType, maxCount
}

// toFloat64 converts numeric types to float64.
func toFloat64(v interface{}) (float64, error) {
	switch val := v.(type) {
	case int:
		return float64(val), nil
	case int32:
		return float64(val), nil
	case int64:
		return float64(val), nil
	case float32:
		return float64(val), nil
	case float64:
		return val, nil
	case string:
		return strconv.ParseFloat(val, 64)
	default:
		return 0, fmt.Errorf("not a number")
	}
}

// countDecimalPlaces returns the number of digits after the decimal point.
func countDecimalPlaces(s string) int {
	parts := strings.Split(s, ".")
	if len(parts) == 2 {
		return len(parts[1])
	}
	return 0
}

// matchRegex compiles and matches a regex pattern against a value.
func matchRegex(pattern, value string) bool {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return false
	}
	return re.MatchString(value)
}

// --- Quality Metrics (moved from server.go, converted to pure functions) ---

const maxQualityIssuesPerColumn = 10

// GenerateDataQualityMetrics computes quality metrics for each column in a table.
func GenerateDataQualityMetrics(sampleData []map[string]interface{}, columns []SchemaColumnInfo) map[string]QualityMetrics {
	metrics := make(map[string]QualityMetrics)
	for _, col := range columns {
		values := CollectColumnSampleValues(sampleData, col.ColumnName)
		metrics[col.ColumnName] = ComputeColumnQualityMetrics(col, values)
	}
	return metrics
}

// CollectColumnSampleValues extracts values for a single column from sample rows.
func CollectColumnSampleValues(sampleData []map[string]interface{}, columnName string) []interface{} {
	values := make([]interface{}, 0, len(sampleData))
	for _, row := range sampleData {
		if value, ok := row[columnName]; ok {
			values = append(values, value)
		}
	}
	return values
}

// ComputeColumnQualityMetrics computes completeness, uniqueness, validity, etc. for a column.
func ComputeColumnQualityMetrics(col SchemaColumnInfo, values []interface{}) QualityMetrics {
	total := len(values)
	if total == 0 {
		return QualityMetrics{
			Issues:       []string{"No sample data available"},
			OverallScore: 0,
		}
	}

	acc := newQualityAccumulator(col)
	for index, value := range values {
		acc.consume(index, value)
	}
	return acc.build(total)
}

type qualityAccumulator struct {
	columnName            string
	patternType           string
	validationRegex       string
	isTemporal            bool
	nonNull               int
	valid                 int
	uniqueSet             map[string]struct{}
	temporalOrderBroken   bool
	lastTime              string
	temporalConsistent    int
	businessRuleCompliant int
	issues                []string
}

func newQualityAccumulator(col SchemaColumnInfo) *qualityAccumulator {
	return &qualityAccumulator{
		columnName:      col.ColumnName,
		patternType:     col.PatternType,
		validationRegex: col.ValidationRegex,
		isTemporal:      col.DataType == "date" || col.DataType == "datetime" || col.DataType == "timestamp",
		uniqueSet:       map[string]struct{}{},
	}
}

func (a *qualityAccumulator) consume(index int, value interface{}) {
	if value == nil {
		a.issues = append(a.issues, fmt.Sprintf("Null value in %s", a.columnName))
		return
	}

	a.nonNull++
	valueText := fmt.Sprintf("%v", value)
	a.uniqueSet[valueText] = struct{}{}
	a.trackValidity(valueText)
	a.trackTemporalConsistency(index, valueText)
	a.businessRuleCompliant++
}

func (a *qualityAccumulator) trackValidity(valueText string) {
	if a.patternType == "" || a.validationRegex == "" {
		a.valid++
		return
	}
	if matchRegex(a.validationRegex, valueText) {
		a.valid++
		return
	}
	a.issues = append(a.issues, fmt.Sprintf("Invalid value for %s: %s", a.columnName, valueText))
}

func (a *qualityAccumulator) trackTemporalConsistency(index int, valueText string) {
	if !a.isTemporal {
		return
	}
	if index > 0 && a.lastTime != "" && valueText < a.lastTime {
		a.temporalOrderBroken = true
	}
	a.lastTime = valueText
	a.temporalConsistent++
}

func (a *qualityAccumulator) build(total int) QualityMetrics {
	completeness := ratio(float64(a.nonNull), float64(total))
	uniqueness := ratio(float64(len(a.uniqueSet)), float64(total))
	validity := ratio(float64(a.valid), float64(total))
	temporalConsistency := 1.0
	if a.isTemporal && a.temporalOrderBroken {
		temporalConsistency = 0.0
		a.issues = append(a.issues, "Temporal inconsistency detected")
	}
	businessRuleCompliance := ratio(float64(a.businessRuleCompliant), float64(total))
	consistency := 1.0
	overall := (completeness + uniqueness + validity + consistency + temporalConsistency + businessRuleCompliance) / 6.0

	return QualityMetrics{
		Completeness:           completeness,
		Uniqueness:             uniqueness,
		Validity:               validity,
		Consistency:            consistency,
		TemporalConsistency:    temporalConsistency,
		BusinessRuleCompliance: businessRuleCompliance,
		OverallScore:           overall,
		Issues:                 TruncateQualityIssues(a.issues),
	}
}

// TruncateQualityIssues limits the number of issues returned per column.
func TruncateQualityIssues(issues []string) []string {
	if len(issues) <= maxQualityIssuesPerColumn {
		return issues
	}
	truncated := issues[:maxQualityIssuesPerColumn]
	truncated = append(truncated, fmt.Sprintf("... %d more issues truncated", len(issues)-maxQualityIssuesPerColumn))
	return truncated
}

// AddTableAggregateMetric adds a __table__ entry with the average overall score.
func AddTableAggregateMetric(columnMetrics map[string]QualityMetrics) {
	if len(columnMetrics) == 0 {
		return
	}
	var sum, count float64
	for _, qualityMetric := range columnMetrics {
		sum += qualityMetric.OverallScore
		count++
	}
	if count == 0 {
		return
	}
	columnMetrics["__table__"] = QualityMetrics{
		OverallScore: sum / count,
		Issues:       []string{},
	}
}

// FlattenQualityMetrics merges column metrics into a flat map with table.column keys.
func FlattenQualityMetrics(metrics map[string]QualityMetrics, tableName string, columnMetrics map[string]QualityMetrics) {
	for columnName, qualityMetric := range columnMetrics {
		key := tableName
		if columnName != "__table__" {
			key += "." + columnName
		}
		metrics[key] = qualityMetric
	}
}

// AddDatabaseAggregateMetric adds a __database__ entry with the average score across all tables.
func AddDatabaseAggregateMetric(metrics map[string]QualityMetrics) {
	var dbSum, dbCount float64
	for key, qualityMetric := range metrics {
		if !strings.HasSuffix(key, "__table__") {
			dbSum += qualityMetric.OverallScore
			dbCount++
		}
	}
	if dbCount == 0 {
		return
	}
	metrics["__database__"] = QualityMetrics{
		OverallScore: dbSum / dbCount,
		Issues:       []string{},
	}
}

// BuildQualityMetrics orchestrates quality metric computation for all tables.
// Returns a flat map of "table.column" → QualityMetrics, plus __table__ and __database__ aggregates.
func BuildQualityMetrics(analysisLevel string, tableSchemas map[string]TableInfo, sampleDataMap map[string][]map[string]interface{}) map[string]QualityMetrics {
	metrics := make(map[string]QualityMetrics)
	if analysisLevel != AnalysisLevelDetailed && analysisLevel != AnalysisLevelComprehensive {
		return metrics
	}
	for tableName, schema := range tableSchemas {
		columnMetrics := GenerateDataQualityMetrics(sampleDataMap[tableName], schema.Columns)
		AddTableAggregateMetric(columnMetrics)
		FlattenQualityMetrics(metrics, tableName, columnMetrics)
	}
	AddDatabaseAggregateMetric(metrics)
	return metrics
}

// CategorizeTables classifies tables by business role (core, lookup, junction, audit).
// Uses FK structural analysis when available: tables with 2+ outgoing FKs and few
// non-FK columns are likely junction tables; tables referenced by many FKs are core/lookup.
func CategorizeTables(tableNames []string, schemas map[string]TableInfo, fks []ForeignKeyRelationship) TableCatalog {
	// Build FK signal maps
	outgoingFKs, incomingFKs := buildFKSignalMaps(fks)

	var coreEntities, lookupTables, junctionTables, auditTables []TableEntity
	for _, tbl := range tableNames {
		info := schemas[tbl]
		entity := TableEntity{
			TableName:   tbl,
			ColumnCount: info.ColumnCount,
			PrimaryKey:  info.KeyColumns.PrimaryKey,
			OutgoingFKs: outgoingFKs[tbl],
			IncomingFKs: incomingFKs[tbl],
		}
		entity.BusinessRole = classifyTable(tbl, info.ColumnCount, outgoingFKs[tbl])

		switch entity.BusinessRole {
		case "audit":
			auditTables = append(auditTables, entity)
		case "junction":
			junctionTables = append(junctionTables, entity)
		case "lookup":
			lookupTables = append(lookupTables, entity)
		default:
			coreEntities = append(coreEntities, entity)
		}
	}
	return TableCatalog{
		CoreEntities:   coreEntities,
		LookupTables:   lookupTables,
		JunctionTables: junctionTables,
		AuditTables:    auditTables,
	}
}

// buildFKSignalMaps returns outgoing and incoming FK counts per table.
func buildFKSignalMaps(fks []ForeignKeyRelationship) (outgoing, incoming map[string]int) {
	outgoing = make(map[string]int)
	incoming = make(map[string]int)
	for _, fk := range fks {
		outgoing[fk.FromTable]++
		incoming[fk.ToTable]++
	}
	return
}

// classifyTable returns the business role for a table: "audit", "junction", "lookup", or "core".
func classifyTable(tableName string, columnCount, outgoingFKCount int) string {
	lower := strings.ToLower(tableName)

	if isAuditTable(lower) {
		return "audit"
	}
	if isJunctionTable(lower, columnCount, outgoingFKCount) {
		return "junction"
	}
	if isLookupTable(lower) {
		return "lookup"
	}
	return "core"
}

// isAuditTable detects audit/history tables by naming convention.
func isAuditTable(lowerName string) bool {
	return strings.Contains(lowerName, "log") || strings.Contains(lowerName, "audit")
}

// isJunctionTable detects junction/mapping tables by FK structure or naming.
func isJunctionTable(lowerName string, columnCount, outgoingFKCount int) bool {
	if outgoingFKCount >= 2 && columnCount > 0 && columnCount-outgoingFKCount <= 2 {
		return true
	}
	return strings.Contains(lowerName, "junction") || strings.Contains(lowerName, "join")
}

// isLookupTable detects lookup/reference tables by naming convention.
func isLookupTable(lowerName string) bool {
	return strings.Contains(lowerName, "lookup") || strings.HasSuffix(lowerName, "_type") || strings.HasSuffix(lowerName, "_status")
}
