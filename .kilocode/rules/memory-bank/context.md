# Database MCP Server - Context

## Current State

- Version: `v1.4.0`
- Stage: Production-ready
- Toolchain: `go 1.26` with `go1.26.2`
- MCP SDK: `github.com/modelcontextprotocol/go-sdk v1.5.0`
- Registered tools: `20`

## Implemented Capabilities

- Core DB workflow:
  - `configure-profile`, `list-profiles`, `execute-sql`
  - `list-databases`, `list-tables`, `describe-table`
  - `list-schemas`, `get-search-path`, `discover-joins`, `sample-data`
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
- `analyzer.go` — Run() orchestration function
- `columns.go` — Bulk column fetching (MySQL/PostgreSQL/SQLite)
- `relationships.go` — Real FK discovery, implicit relationships, relationship graph
- `performance.go` — Index analysis, performance optimization recommendations
- `enrichment.go` — Business context, data patterns, quality metrics, table categorization
- `rows.go` — Sample row fetching and normalization
- `types.go` — Shared types for the analyze pipeline

Server.go has a thin handler that delegates to `analyze.Run()`, keeping only MCP-specific code (query suggestions via smart-builder, profiling).

### Key Refactoring (v1.3.0–v1.4.0)
- Extracted ~40 functions from server.go into `internal/mcp/analyze/` as pure functions
- Fixed 6 analyze-schema bugs (#75–#80): column scanning, FK discovery, implicit relationships, classification signals, index analysis
- Bug fix: SemanticTypeRegexes regex overlap resolved with ordered matching

## Gemini Compatibility

- JSON schemas are automatically sanitized for Google Gemini's OpenAPI 3.0 subset requirements.
- Schema mode `compact` is the default for tool-first clients.
- All 20 tools are compatible with Gemini function calling.

## Packaging and Release State

- Release workflow publishes GitHub Releases from tags
- Package workflow publishes GHCR container images
- Latest release line currently aligned at `v1.4.0`

## Documentation State

- Root README aligned with `v1.4.0` and 20 tools
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
