# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

### Added
- `validate-query` MCP tool for pre-execution SQL syntax/logic/security checks.
- Vitess SQL parser dependency for validation.
- Documentation updates: README, API docs, memory bank entries.

## [v1.0.1] - 2025-12-09

- Fix MCP tool discovery by upgrading to `github.com/modelcontextprotocol/go-sdk v1.1.0`.
- Add structured error payload examples and troubleshooting for invalid credentials, network drop, and read-only enforcement.
- Validate profile management workflows end-to-end (SQLite) and live connectivity to podman MySQL/PostgreSQL containers via integration tests.
- Document release build/smoke-test commands and live DB env var matrix in README.

## [v1.0.0] - 2025-11-?? (initial)

- Initial release with 12 MCP tools (profile management, SQL execution, schema introspection, resources).
