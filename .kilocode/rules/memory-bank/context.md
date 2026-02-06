# Database MCP Server - Context

## Current State

- Version: `v1.1.0`
- Stage: Production-ready
- Toolchain: `go 1.25` with `go1.25.7`
- MCP SDK: `github.com/modelcontextprotocol/go-sdk v1.2.0`
- Registered tools: `18`

## Implemented Capabilities

- Core DB workflow:
  - `configure-profile`, `list-profiles`, `execute-sql`
  - `list-databases`, `list-tables`, `describe-table`
  - `discover-joins`, `sample-data`
- Query intelligence:
  - `smart-query-builder`, `validate-query`, `optimize-query`
- Analysis/governance:
  - `analyze-schema` (with optional profiling)
  - `analyze-data-lineage`
  - `discover-insights`
  - `track-schema-changes` (track/history/generate_migration/detect_drift)
  - `federated-query`
- Runtime metadata:
  - `list-tools`, `mcp-info`

## Packaging and Release State

- Release workflow publishes GitHub Releases from tags
- Package workflow publishes GHCR container images
- Latest release line currently aligned at `v1.1.0`

## Documentation State

- Root README aligned with `v1.1.0` and 18 tools
- docs/ updated to reflect current runtime behavior
- Wiki expanded with onboarding, tool-scenario mapping, and client setup guides

## Key Historical Context

- Previous MCP `tools/list` discoverability issue was resolved by SDK upgrades and regression tests
- F1-F4 roadmap slices are implemented and merged

## Immediate Priorities

1. Maintain documentation/runtime parity for future changes
2. Keep coverage and CI quality gates stable
3. Maintain release and package automation health
