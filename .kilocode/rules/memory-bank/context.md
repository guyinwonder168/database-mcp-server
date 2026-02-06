# Database MCP Server - Context

## Current State

### Project Status
- Version: v1.0.7
- Author: guyinwonder
- Created using: OpenAI GPT-4.1 via VSCode Kilocode AI code assistant extension
- Stage: Production-ready with schema evolution handler integration complete
- Last Updated: February 2026
- Toolchain: Go 1.25.7 (default via gvm)

### Implementation Status
- ✅ Core MCP server using official Go SDK (v1.2.0, upgraded from v1.1.0)
- ✅ Multi-database support (MySQL, MariaDB, PostgreSQL, SQLite)
- ✅ Interactive CLI setup wizard with auto-configuration
- ✅ Profile management (create, list, update, delete)
- ✅ SQL execution with enhanced read-only enforcement and multi-statement protection
- ✅ Schema introspection (list tables, describe table, list databases) with comprehensive metadata
- ✅ Sample data fetching from tables with configurable limits
- ✅ AES-GCM password encryption (32-char key) with auto-generation
- ✅ Connection pooling with configurable limits
- ✅ Structured JSON logging with rotation and credential redaction
- ✅ 17 MCP tools implemented and documented:
  - configure-profile, list-profiles, execute-sql
  - list-tables, describe-table, list-databases
  - analyze-schema (with 3 analysis levels)
  - smart-query-builder, discover-joins, sample-data
  - mcp-info, list-tools
  - optimize-query, validate-query
  - analyze-data-lineage, discover-insights
  - track-schema-changes
- ✅ Schema evolution implementation completed for `track-schema-changes`:
  - Phase 1: snapshot/types (`schema_snapshot_types.go`)
  - Phase 2: snapshot storage and diff/drift detection (`schema_storage.go`)
  - Phase 3: migration generator (`schema_migrations.go`)
  - Phase 4: MCP handler integration and tool registration (`schema_tracker.go`)
- ✅ Valid JSON Schema for tool parameters (`params` arrays)
- ✅ Documentation for base64-encoded BLOB/BINARY parameters
- ✅ Comprehensive error analysis with structured error responses and actionable suggestions
- ✅ Enhanced analyze-schema implementation:
  - Business context inference and domain detection
  - Data quality metrics with scoring
  - Relationship discovery (FK + semantic)
  - Smart Query Builder integration
  - Pattern recognition and validation
- ✅ Advanced security features:
  - Credential redaction in logs
  - Enhanced SQL injection prevention
  - Read-only profile enforcement with CTE support
- ✅ MCP resources added:
  - `tools://list` (JSON snapshot of registered tools)
  - `profile://{profile}` (profile metadata with secrets redacted)
- ✅ Optional HTTP/SSE transport: enable via `MCP_SSE_ADDR` (Claude: point provider to http://localhost:PORT; Codex: use if SSE supported, else stdio; Kilocode: prefers stdio unless testing SSE)
- ✅ Logging: stdout disabled by default to avoid MCP stdio contamination; enable via `MCP_LOG_TO_STDOUT=true`
- ✅ Live DB smoke tests: Postgres and MySQL/MariaDB via env vars `DB_MCP_IT_*`
- ✅ Codex/Kilocode tool discovery confirmed (16 tools visible)

### Documentation Enhancements
- ✅ Consolidated enhancement roadmap into docs/roadmap.md
- ✅ Updated documentation links to the consolidated roadmap
- ✅ Planning history preserved in docs/history

### Known Issues
- None identified

### MCP Tool Detection Fix (December 2024, validated Dec 2025)
- **Critical Issue Resolved**: MCP Go SDK v0.2.0 had a broken `tools/list` method that prevented MCP clients from discovering any tools
- **Solution Applied**: Upgraded to MCP Go SDK v1.1.0 which fixes the tools/list routing bug
- **Testing Added**: Comprehensive regression tests in `internal/mcp/tools_list_integration_test.go` to prevent future regressions
- **Impact**: All 12 MCP tools are now discoverable by MCP clients (Codex, Kilocode, Claude Desktop)
- **Verification**: In-memory client tests confirm tools/list works correctly and capabilities are properly advertised

## Recent Changes (February 2026)
- Added GHCR container packaging workflow (`.github/workflows/package.yml`) to publish versioned container images for release tags
- Added production multi-stage `Dockerfile` and README container usage instructions
- Implemented F2 Phase 4 MCP handler integration (`schema_tracker.go`)
- Registered `track-schema-changes` as a public MCP tool
- Added end-to-end schema tracker tests (`schema_tracker_test.go`)
- Implemented F2 Phase 3 migration generator (`schema_migrations.go`)
- Added migration generator test coverage (`schema_migrations_test.go`)
- Added SQL conversion, validation, and impact estimation utilities for schema diffs
- Updated roadmap/TDD/docs status to reflect F2 phase progress
- Upgraded Go SDK to v1.2.0
- Fixed JSON Schema for tool parameters to avoid validation errors
- Added documentation for BLOB/BINARY base64 encoding
- Updated project implementation status and roadmap tracking
- Version bumped to v1.0.7

## Recent Changes (January 2026)
- Consolidated enhancement roadmap into docs/roadmap.md
- Moved legacy planning docs into docs/history for reference
- Updated documentation links to point at the consolidated roadmap
- Recorded optimize/validate/lineage completions and current NLP work in roadmap
- Memory bank refreshed for v1.0.2

## Next Steps

### Immediate Priorities
- Start F3 advanced profiling enhancements for `analyze-schema`
- Monitor for bug reports and user feedback

### Future Enhancements
- Advanced data profiling
- Multi-database federation
