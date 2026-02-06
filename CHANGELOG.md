# Changelog

All notable changes to this project will be documented in this file.

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
