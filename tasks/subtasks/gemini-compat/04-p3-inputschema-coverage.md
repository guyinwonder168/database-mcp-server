# P3 - InputSchema Coverage 100%

## Objective

Guarantee every public tool has explicit `InputSchema`.

## In Scope

- `internal/mcp/server.go`
- parameter structs/tags used for schema inference
- schema contract tests

## Tasks

- Add `InputSchema` to any tool missing it.
- Ensure required fields are represented in schema.
- Add/extend contract test that fails if any tool lacks schema.

## Out of Scope

- No helper tool implementation here.

## Acceptance Checks

- Contract test reports 100% schema coverage.
- Existing tool invocation behavior remains unchanged.

## Delegation Prompt Seed

Add explicit InputSchema for all MCP tools and enforce with a failing test if any tool is missing schema.
