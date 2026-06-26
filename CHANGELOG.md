# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

## [v1.6.3] - 2026-06-26

### Fixed

- Fixed `execute-sql` discarding direct and row-stream query errors and returning `UNKNOWN_ERROR` with `details: "<nil>"`. SQL driver details are now preserved, MySQL/MariaDB errno and SQLSTATE metadata are exposed, and unknown-column errors map to `COLUMN_NOT_FOUND`.
- Fixed MySQL/MariaDB missing-table diagnostics for schema-qualified names such as `db.table`, preserving the full table name in `TABLE_NOT_FOUND` responses.
- Fixed MySQL/MariaDB unknown-database diagnostics so `DATABASE_NOT_FOUND` responses preserve the configured target database name for connection and database-switch failures.

### Security

- Updated the Go toolchain and CI/Docker build images from vulnerable patch releases to Go 1.26.4, resolving reachable standard-library findings reported by `govulncheck`.
- Switched CI vulnerability scanning to the official `golang/govulncheck-action` v1.0.4 pinned by immutable commit SHA.
- Configured SonarCloud analysis to use pinned Java 21 and skip scanner JRE auto-provisioning, avoiding intermittent HTTP 403 failures from Sonar's JRE endpoint.

## [v1.6.2] - 2026-04-19

### Fixed

- **Code scanning alerts (gosec)**: Resolved 4 open security alerts:
  - **G115**: Replaced `byte(unicode.ToLower(rune(ch)))` with explicit `A-Z` range check in `sanitizeReadOnlySQL` — only converts uppercase ASCII letters, leaving all other characters (`_`, `@`, `[`, etc.) untouched
  - **G201**: Added proper identifier validation and dialect-specific quoting in `sampleTableData` via new `sanitizeSQLIdentifier()` regex validator and `safeQuoteIdentifier()` function
  - **G117**: Added `#nosec G117` with justification — Password field is redacted to `""` before JSON marshaling
  - **G602**: Added `#nosec G602` with justification — `bucketIndex` is bounded to `[0,9]` by explicit range checks
- Added 29 new test cases covering identifier validation, dialect-specific quoting, and SQL injection rejection
- All 18 CI checks passing: gosec, SonarCloud, lint, coverage (85.9%), CodeQL, security

## [v1.6.1] - 2026-04-19

### Fixed

- Updated `mcp-info` tool count from "20 tools" to "21 tools" to reflect the `list-schemas` tool added in PR #88.

## [v1.6.0] - 2026-04-19

### Added

- **FK/index data pipeline fix** (Issues #77, #80): Discovered foreign keys and indexes now flow correctly into `table_schemas[].key_columns` output.
  - New `applyFKsToColumns()` — sets `IsForeignKey=true` and `ForeignKeyRef` on FK columns in `SchemaColumnInfo`.
  - New `applyFKsToSchemas()` — populates `KeyColumns.ForeignKeys` from discovered FK relationships.
  - New `applyIndexesToColumns()` — sets `Indexed=true` on columns present in fetched indexes (including composite indexes missed by column metadata queries).
  - New `rebuildKeyColumns()` — re-extracts `KeyColumns` from enriched column data after all apply functions run.
- 6 regression tests for FK/index data pipeline (empty-input safety, PK index skip, end-to-end integration).

### Changed

- **Signal-provider architecture for `analyze-schema`** (Issues #77-#80): The MCP server now provides raw structured signals for the calling LLM to interpret, rather than making hardcoded domain/entity/performance classifications.
  - **Issue #77**: Removed legacy dead code producing 2,336 false-positive `shared_column` relationships. Added `commonFKSuffixes` filter to skip generic FK patterns (`status_id`, `type_id`, etc.). FK/index/row-count fetch errors now produce warnings instead of being silently discarded.
  - **Issue #78**: Replaced hardcoded 7-domain `DetectDomain` with signal-based `ComputeDomainSignals` that produces naming prefix frequencies (e.g., `{"call": 5, "broadcast": 3, "sip": 2}`). The calling LLM interprets domain from raw signals using its own world knowledge. Updated tool description to inform LLMs that `domain_indicators` provides signal frequencies, not authoritative labels.
  - **Issue #79**: Enhanced `CategorizeTables` with FK structural analysis: tables with 2+ outgoing FKs and few non-FK columns are classified as junction tables. Added `OutgoingFKs`/`IncomingFKs` signal fields to `TableEntity`.
  - **Issue #80**: Added `IndexCoverage` struct to `PerformanceOptimization` reporting total/indexed/unindexed FK columns and tables without primary keys. Added tables-without-PK detection.
- **Cognitive complexity reduction** (SonarCloud go:S3776):
  - Extracted `classifyTable`, `buildFKSignalMaps`, `isAuditTable`, `isJunctionTable`, `isLookupTable` from `CategorizeTables` (complexity 17 → ~5).
  - Extracted `buildIndexedColSet`, `recommendFKIndexes`, `findTablesWithoutPK` from `BuildPerformanceOptimization` (complexity 24 → ~5).

### Removed

- Removed legacy dead code from `server.go`: `detectImplicitRelationships`, `analyzeNamingRelationships`, `correlateDataValues`, `referenceColumnsForTarget`, `buildIDSet`, `countReferenceMatches` (only used by coverage tests, never by handlers).
- Removed `mergeDomainIndicators` helper (replaced by `ComputeDomainSignals`).

### Fixed

- **PR #88** (Issues #85, #87): Fixed OpenAPI spec missing 10 tool definitions and `list-schemas` tool not registered. Tool count: 20 → 21 across all docs.
- Marked unused parameters in `GenerateBusinessDescription` with `_` (kilo-code-bot review).

## [v1.5.1] - 2026-04-19

### Fixed
- **Bug #75**: `analyze-schema` returned 0 columns for MySQL/MariaDB — `resolveSchemaForAnalyze` now correctly passes `databaseName` for MySQL/MariaDB instead of empty string.
- **Bug #76**: `analyze-schema` returned 0 row counts for MySQL/MariaDB — root cause same as #75 (empty `TABLE_SCHEMA` filter).
- **Bug #77**: `analyze-schema` returned 0 foreign keys for MySQL/MariaDB — root cause same as #75.
- **Bug #78**: Domain detection showed "unknown" for all databases — cascading from 0 columns; classification signals now populate correctly.
- **Bug #79**: All tables categorized as "core" (100%) — cascading from 0 columns; proper categorization restored.
- **Bug #80**: `analyze-schema` returned only generic tips with no query patterns or index recommendations — cascading from 0 columns; full analysis restored.
- Added `Warnings []string` field to `AnalyzeSchemaResult` for non-fatal privilege warnings.
- Added post-analysis privilege detection: warns when tables exist but 0 columns are accessible (likely insufficient database privileges).

## [v1.5.0] - 2026-04-19

### Changed
- **Extracted `internal/mcp/analyze/` package**: ~40 functions refactored out of `server.go` into a dedicated, testable package with pure functions (no MCPServer dependencies).
- **Security hardening**: Eliminated all `fmt.Sprintf` SQL patterns (SonarCloud S2077). SQLite PRAGMAs converted to parameterized table-valued functions. Added `sanitizeIdentifier()`, `quoteMySQL()`, `quotePostgres()`, `quoteSQLite()`, and `quoteForDB()` helpers for safe SQL identifier handling.
- **Cognitive complexity reduction** (SonarCloud S3776): Extracted 8 private helper functions across `analyzer.go`, `performance.go`, `relationships.go`, and `server.go` to bring all functions below complexity threshold.
- **CI workflow hardening** (S6439): Moved `${{ }}` expressions to `env:` blocks in release workflow to prevent script injection.

### Fixed
- **Bug #75**: Column scanning — fixed column metadata extraction for all database types.
- **Bug #76**: FK discovery — fixed foreign key relationship detection.
- **Bug #77**: Implicit relationships — fixed column-to-table matching for implicit FK inference.
- **Bug #78**: Classification signals — fixed signal extraction for LLM-based domain inference.
- **Bug #79**: Index analysis — fixed index metadata parsing for PostgreSQL and SQLite.
- **Bug #80**: Semantic type regex overlap — resolved with ordered matching.
- All SonarCloud quality gate issues resolved (S2077, S3776, S6439, gosec G201/G202).

## [v1.4.0] - 2026-03-30

### Added
- **PostgreSQL Multi-Schema Support**: Full schema awareness across all relevant MCP tools.
  - `list-schemas` MCP tool to discover all accessible database schemas.
  - `get-search-path` MCP tool for read-only diagnostic of current `search_path` and effective schema.
  - `list-tables` now returns `[]TableRef` (schema + table name) instead of `[]string`.
  - `describe-table`, `sample-data`, and `analyze-schema` accept optional `schema` parameter with auto-detection fallback (`current_schema()` → first accessible → `'public'`).
  - Schema quoting via `pq.QuoteIdentifier()` for safe SQL interpolation.
  - Default schema detection utility (`getDefaultSchema`) with graceful fallback chain.
- Schema-aware tests converted to [go-sqlmock](https://github.com/DATA-DOG/go-sqlmock) for MySQL and PostgreSQL (no live DB required).
- Coverage tests for schema resolution functions, table info scanning, and error paths.

### Changed
- Go toolchain bumped to 1.26.1.
- `describePostgresTableColumns` refactored to use parameterized schema query (gosec G202 fix).
- Extracted helper functions from `handleListSchemas` and `describePostgresTableColumns` to reduce cognitive complexity.
- CI govulncheck step replaced with manual `go install` + `govulncheck ./...` to avoid intermittent `Duplicate header: Authorization` bug from `govulncheck-action`.

### Fixed
- Resolved SonarCloud `go:S107` (too many parameters) and `unparam` lint issues.
- Resolved lint warnings in coverage tests.
- Fixed gosec G202 (SQL string concatenation) by using parameterized queries for schema filtering.

## [v1.3.0] - 2026-02-21

### Added
- `configure-profile` now supports `action` field with `"delete"` and `"clone"` actions.
- Delete action: remove a profile by name (`{"action": "delete", "profile_name": "mydb"}`).
- Clone action: copy an existing profile with optional overrides (`{"action": "clone", "profile_name": "new", "source_profile": "existing"}`).
- Updated tool description and help metadata for the new actions.

## [v1.2.1] - 2026-02-19

### Fixed
- JSON Schema sanitization for Google Gemini compatibility (OpenAPI 3.0 subset compliance).
  - Converts `type: ["null","array"]` to `type: "array"` (Gemini requires single type strings).
  - Removes `additionalProperties: false` (rejected by Gemini endpoint).
  - Fixes boolean `items: true` to proper schema objects.
- SonarCloud code quality issues: S8193 (unnecessary variable declarations) and S3776 (cognitive complexity reduction via helper function extraction).

## [v1.2.0] - 2026-02-13

### Added
- `get-tool-help` MCP tool for on-demand tool summaries, examples, and troubleshooting.
- Provider smoke tests for compact (Gemini-style) and standard (Claude-style) discovery/call paths.
- Schema contract gate tests (`GATE_SCHEMA_001..005`) for InputSchema coverage and compact payload budgets.

### Changed
- Added `schema_mode` config (`compact|standard`) with default `compact`.
- Switched tool metadata to compact-by-default descriptions in compact mode.
- Added explicit `InputSchema` for all registered tools.
- CI now enforces schema contract gates via `go test ./internal/mcp -run TestSchemaGate`.

## [v1.1.1] - 2026-02-07

### Fixed
- Resolved remaining SonarCloud new-code findings in analyze-schema helpers (`S8193`, `S107`).
- Refactored analyze-schema result assembly to use a structured input object for maintainability.

## [v1.1.0] - 2026-02-06

### Added
- GHCR container package publishing workflow (`.github/workflows/package.yml`) for version tags and manual backfill.
- Multi-stage production Dockerfile for publishing `database-mcp-server` images to GitHub Packages.
- Container usage guidance in README for pulling and running released package images.
- Advanced data profiling implementation for `analyze-schema` (F3): profiling types, statistical engine, and concurrent table profiling.
- Optional `profiling` request parameter for `analyze-schema` with backward-compatible response payload.
- New profiling tests: pattern detection, statistics, quality scoring, concurrent enhancer, and handler integration coverage.
- `federated-query` MCP tool (F4) with planner, join engine, concurrent executor, and MCP handler.
- Federation SQL parsing support for `profile.table` syntax with optional explicit `sub_queries`.
- Cross-profile JOIN support (`INNER`, `LEFT`, `RIGHT`, `FULL`) and optional post-join aggregations.
- Federated execution metadata with per-subquery row counts and partial-failure error payloads.
- New federation tests: types, planner, join, executor, handler, and tool registration coverage.
- Version sync helper script: `scripts/sync-version-from-server.sh` (syncs README/OpenAPI from `MCPVersion`).

### Changed
- `analyze-schema` tool description and OpenAPI contract updated to document optional profiling output (`column_profiling`).
- Tool registry/documentation updated from 17 to 18 tools to include `federated-query`.
- Release/CI version source-of-truth standardized to `internal/mcp/server.go` (`MCPVersion`).
- README/OpenAPI version sync is now enforced by CI/release workflows.

## [v1.0.7] - 2026-02-06

### Added
- `track-schema-changes` Phase 4: MCP Tool Handler (F2)
- Operation routing for schema tracking workflows: `track`, `history`, `generate_migration`, `detect_drift`
- Schema snapshot capture from live database metadata with SHA-256 integrity hash
- Snapshot retention enforcement with configurable `retention_days` (default 30 days)
- Schema drift detection against baseline snapshots
- End-to-end tests for schema tracker handler operations and tool registration

### Changed
- Registered `track-schema-changes` in MCP tool registry
- Updated tool count from 16 to 17 across project docs

### Fixed
- Raised schema tracker new-code coverage with additional helper and error-path tests to satisfy SonarCloud Quality Gate thresholds
- Updated SonarCloud workflow to use `sonarqube-scan-action` and emit `report.json` for Go test report ingestion
- Resolved 9 SonarCloud PR code-smell issues in schema tracker/migration logic (cognitive complexity, duplicated literals, and readability findings)

## [v1.0.6] - 2026-02-06

### Added
- `track-schema-changes` Phase 2: Snapshot Storage (F2)
- `track-schema-changes` Phase 3: Migration Generator (F2)
- Dialect-aware SQL generation for schema changes (`mysql`, `postgresql`, `sqlite`)
- Migration validation with structured validation errors
- Migration impact estimation (risk level, downtime heuristic, estimated duration)
- Manual-action fallback comments for dialect-limited operations
- Comprehensive unit tests for migration conversion/generation/validation/impact estimation
- Schema snapshot persistence to filesystem (JSON format)
- Snapshot retrieval by profile and ID
- Snapshot listing with configurable limit
- Schema comparison and diff detection (added/removed/modified tables and columns)
- Schema drift detection between current state and last snapshot
- SHA-256 hash generation for schema integrity verification
- Column type and constraint change detection
- Impact classification (breaking, compatible, informational)

### Changed
- Update tool count from 16 to 17

## [v1.0.5] - 2026-02-06

### Added
- `discover-insights` MCP tool for automatic business intelligence discovery
- KPI detection (total, average, min, max) for numeric columns
- Trend analysis using linear regression with R² confidence scoring
- Anomaly detection using Z-score threshold
- Distribution analysis with histogram bucketing
- Time-series column detection for automatic trend analysis
- Insight prioritization (anomalies > trends > KPIs > distributions)

### Changed
- Update README to include discover-insights tool documentation
- Update tool count from 15 to 16

## [v1.0.4] - 2026-02-04

### Added
- CI summary job (`build-binaries-summary`) for reliable branch protection with matrix builds.
- SonarCloud, CodeQL, dependency review, and gosec CI workflows.
- Makefile for common development commands.
- AGENTS.md with contributor guidelines.
- Comprehensive unit and integration test coverage for helpers, errors, SQLite lineage, and connection overrides.
- `sonar-project.properties` for SonarCloud integration.

### Changed
- Upgrade `github.com/modelcontextprotocol/go-sdk` to v1.2.0.
- Refactor MCP handlers to use shared helper functions, reducing code duplication.
- Extract `sqlite_master` query to constant to satisfy linter (go:S1192).
- Update README attribution to reflect vibe coding tools.
- Update memory bank documentation to v1.0.4 status.

### Fixed
- Provide valid JSON Schema for tool params arrays and document base64-encoded BLOB/BINARY values.
- Resolve SonarCloud security hotspots and improve new code coverage.
- Remove unused `nolint:gosec` directive in helpers.go.
- Pin Go version to 1.25.7 across all CI workflows for consistency.
- Resolve all 12 SonarCloud issues: reduced cognitive complexity, fixed variable naming, removed duplicate literals.

## [v1.0.3] - 2026-01-18

### Added
- NLP config options for smart-query-builder (context, domain hints).
- Business domain hints and multi-turn context support in smart-query-builder.
- Conversation context helpers and tests for smart-query-builder flows.

### Changed
- Context handling moved into dedicated conversation types.

## [v1.0.2] - 2026-01-17

### Added
- `validate-query` MCP tool for pre-execution SQL syntax/logic/security checks.
- `optimize-query` MCP tool for EXPLAIN-based findings and performance estimates.
- `analyze-data-lineage` MCP tool for tracing table dependencies.
- Vitess SQL parser dependency for validation.

### Changed
- Documentation updates: README, API docs, memory bank entries.
- README Go badge URL fixed for proper rendering.
- AGENTS guidance clarified for local testing artifacts in `pkg/`.

## [v1.0.1] - 2025-12-09

- Fix MCP tool discovery by upgrading to `github.com/modelcontextprotocol/go-sdk v1.1.0`.
- Add structured error payload examples and troubleshooting for invalid credentials, network drop, and read-only enforcement.
- Validate profile management workflows end-to-end (SQLite) and live connectivity to podman MySQL/PostgreSQL containers via integration tests.
- Document release build/smoke-test commands and live DB env var matrix in README.

## [v1.0.0] - 2025-11-?? (initial)

- Initial release with 12 MCP tools (profile management, SQL execution, schema introspection, resources).
