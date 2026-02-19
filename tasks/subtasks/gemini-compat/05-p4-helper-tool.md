# P4 - Single Helper Tool (`get-tool-help`)

## Objective

Provide on-demand usage examples and troubleshooting through one compact helper tool.

## In Scope

- `internal/mcp/server.go`
- `internal/mcp/tool_help.go` (new)
- `internal/mcp/tool_help_test.go` (new)

## Proposed Contract

- Input:
  - `tool_name` (required string)
  - `topic` (optional enum: `summary|minimal_example|advanced_example|errors|all`)
- Output:
  - `tool_name`, `summary`, `minimal_example`, `advanced_example`, `common_errors`, `notes`

## Tasks

- Register `get-tool-help` with explicit `InputSchema`.
- Implement deterministic catalog-backed responses.
- Return helpful response for unknown tool names.

## Out of Scope

- No external network dependency.

## Acceptance Checks

- Helper tool returns expected shape for valid tool.
- Unknown tool path returns clear guidance.
- Tests cover topic filtering and output shape.

## Delegation Prompt Seed

Implement MCP helper tool `get-tool-help` with explicit schema, deterministic help catalog, and tests for valid/unknown tools.
