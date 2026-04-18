# Database MCP Server - Project Brief

## Project Overview

Database MCP Server is a production-ready MCP provider for SQL databases, implemented in Go. It exposes a unified tool interface so AI agents and developers can safely interact with MySQL, MariaDB, PostgreSQL, and SQLite.

Current implementation state:
- Version: `v1.5.0`
- MCP tools: `20` implemented and registered
- Packaging: GitHub Releases + GHCR container images

## Core Mission

Provide a secure, practical, and discoverable MCP database interface that works consistently across supported SQL engines and coding clients.

## Functional Requirements (Current)

1. Multi-database connectivity (MySQL, MariaDB, PostgreSQL, SQLite)
2. Profile configuration and listing (`configure-profile`, `list-profiles`)
3. SQL execution with read-only policy enforcement (`execute-sql`)
4. Schema discovery (`list-databases`, `list-tables`, `describe-table`, `list-schemas`, `get-search-path`, `discover-joins`)
5. Data sampling and analysis (`sample-data`, `analyze-schema`, `discover-insights`)
6. Query intelligence (`smart-query-builder`, `validate-query`, `optimize-query`)
7. Governance and advanced workflows (`analyze-data-lineage`, `track-schema-changes`, `federated-query`)
8. Runtime discovery and provider metadata (`list-tools`, `get-tool-help`, `mcp-info`)

## Non-Functional Requirements

1. Security: encrypted credentials (AES-GCM), safe logging, read-only guardrails
2. Performance: pooled connections and efficient stateless handlers
3. Reliability: structured errors and regression-tested tool registration
4. Operability: straightforward setup, documented release/package workflow

## Success Criteria

- Reliable cross-client MCP integration (Codex, Claude Code, OpenCode, Kilo, Roo, Cline, Goose, etc.)
- Zero plaintext credential persistence
- Practical and accurate documentation aligned with runtime contracts
- Stable production operation with clear troubleshooting paths
