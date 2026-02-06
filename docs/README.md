# Database MCP Server Documentation

## Overview

This directory contains the canonical project documentation for the current production state.

Current baseline:
- Version: `v1.1.0`
- MCP tools: `18` implemented and registered
- Databases: MySQL, MariaDB, PostgreSQL, SQLite
- Go baseline: `go 1.25` with toolchain `go1.25.7`

## Document Index

### Core

- [implementation-status.md](implementation-status.md)
  - Current feature/tool status and release state.
- [api-documentation.md](api-documentation.md)
  - Runtime tool contracts and usage patterns.
- [technical-specifications.md](technical-specifications.md)
  - Architecture, runtime model, and operational constraints.
- [mcp-openapi.yaml](mcp-openapi.yaml)
  - Machine-readable API schema for tooling and validation.
- [mcp-examples.md](mcp-examples.md)
  - Practical request/response examples.
- [roadmap.md](roadmap.md)
  - Consolidated roadmap (reflects delivered phases).
- [tdd-implementation-plan.md](tdd-implementation-plan.md)
  - TDD implementation record and hardening checklist.

### Product/Design

- [prd.md](prd.md)
  - Product requirements and long-term direction.
- [analyze-schema-design.md](analyze-schema-design.md)
  - Detailed design for analyze-schema orchestration.
- [schema-introspection-queries.md](schema-introspection-queries.md)
  - DB-specific introspection query references.

### History (Reference)

- [history/](history/)
  - Historical plans and migration context. These files are preserved as historical snapshots and may describe earlier phases.

## Source of Truth Rules

When docs conflict, use this precedence:
1. Runtime implementation (`internal/mcp/server.go`)
2. OpenAPI spec (`docs/mcp-openapi.yaml`)
3. This docs folder markdown

Version/source of truth:
- Runtime version string: `internal/mcp/server.go` (`MCPVersion`)
- Release tags/packages: GitHub Releases + GHCR

## Documentation Maintenance

Update docs in the same PR as code changes when any of these change:
- tool contracts (name/params/output)
- supported databases or behavior
- runtime version
- packaging/release workflow

Minimum files to review on MCP tool changes:
- `README.md`
- `docs/api-documentation.md`
- `docs/mcp-openapi.yaml`
- `docs/implementation-status.md`
- `.kilocode/rules/memory-bank/*.md`
