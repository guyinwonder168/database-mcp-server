# Changelog

All notable changes to this project will be documented in this file.

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
