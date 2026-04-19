# Database MCP Server - Context

## Current State

- Version: `v1.6.1`
- Stage: Production-ready
- Toolchain: `go 1.26` with `go1.26.2`
- MCP SDK: `github.com/modelcontextprotocol/go-sdk v1.5.0`
- Registered tools: `21`

## Implemented Capabilities

- Core DB workflow:
  - `configure-profile`, `list-profiles`, `execute-sql`
  - `list-databases`, `list-tables`, `describe-table`
  - `get-search-path`, `discover-joins`, `sample-data`
- Query intelligence:
  - `smart-query-builder`, `validate-query`, `optimize-query`
- Analysis/governance:
  - `analyze-schema` (with optional profiling)
  - `analyze-data-lineage`
  - `discover-insights`
  - `track-schema-changes` (track/history/generate_migration/detect_drift)
  - `federated-query`
- Runtime metadata:
  - `list-tools`, `get-tool-help`, `mcp-info`

## Architecture Highlights

### Analyze Package (`internal/mcp/analyze/`)
Dedicated package for `analyze-schema` logic, extracted from server.go:
- `analyzer.go` — Run() orchestration, buildTableSchemas, buildRelationshipGraph, buildClassificationSignals
- `columns.go` — Bulk column fetching (MySQL/PostgreSQL/SQLite) with parameterized TVF queries
- `relationships.go` — Real FK discovery, implicit relationships, relationship graph
- `performance.go` — Index analysis, performance optimization recommendations
- `enrichment.go` — Business context, data patterns, quality metrics, table categorization
- `rows.go` — Sample row fetching and normalization
- `sanitize.go` — `sanitizeIdentifier()`, `quoteMySQL()`, `quotePostgres()`, `quoteSQLite()`, `quoteForDB()`
- `types.go` — Shared types for the analyze pipeline
- `doc.go` — Package documentation

Server.go has a thin handler that delegates to `analyze.Run()`, keeping only MCP-specific code (query suggestions via smart-builder, profiling).

### Additional Sub-Packages
- `internal/mcp/lineage/` — Data lineage analyzer and dependency graph
- `internal/mcp/nlp/` — Entity extractor and intent classifier for smart-query-builder
- `internal/mcp/context/` — Conversation context manager

### Key Refactoring (v1.4.0–v1.6.0)
- Extracted ~40 functions from server.go into `internal/mcp/analyze/` as pure functions
- Fixed MySQL/MariaDB schema resolution bug (#75–#80): `resolveSchemaForAnalyze` now passes `databaseName` instead of empty string
- Added `Warnings []string` to `AnalyzeSchemaResult` for privilege detection
- Security hardening: all `fmt.Sprintf` SQL eliminated, SQLite PRAGMAs converted to parameterized TVF, `sanitizeIdentifier()` + `quoteForDB()` added
- Extracted 8 helper functions for cognitive complexity reduction (SonarCloud S3776)
- **v1.6.0**: Signal-provider architecture — replaced hardcoded domain/entity classification with raw naming prefix frequencies + FK structural analysis. Fixed FK/index data pipeline (applyFKsToColumns, applyFKsToSchemas, applyIndexesToColumns, rebuildKeyColumns). Reduced cognitive complexity in CategorizeTables and BuildPerformanceOptimization.

## Gemini Compatibility

- JSON schemas are automatically sanitized for Google Gemini's OpenAPI 3.0 subset requirements.
- Schema mode `compact` is the default for tool-first clients.
- All 21 tools are compatible with Gemini function calling.

## Packaging and Release State

- Release workflow publishes GitHub Releases from tags
- Package workflow publishes GHCR container images
- Latest release line currently aligned at `v1.6.1`

## Documentation State

- Root README aligned with `v1.6.1` and 21 tools
- docs/ updated to reflect current runtime behavior
- Wiki expanded with onboarding, tool-scenario mapping, and client setup guides

## Key Historical Context

- Previous MCP `tools/list` discoverability issue was resolved by SDK upgrades and regression tests
- F1-F4 roadmap slices are implemented and merged
- Gemini schema compatibility implemented via `sanitizeSchemaForGemini()` post-processor

## Immediate Priorities

1. Maintain documentation/runtime parity for future changes
2. Keep coverage and CI quality gates stable
3. Maintain release and package automation health
