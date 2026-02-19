# P0 - Baseline Metrics

## Objective

Measure current MCP tool contract size/shape before changes.

## In Scope

- `internal/mcp/server.go`
- `internal/mcp/schema_contract_test.go` (new)

## Tasks

- Add a test helper to serialize tool declarations from `tools/list` equivalent path.
- Record:
  - tool count
  - per-tool description length
  - total serialized payload bytes
  - tools missing `InputSchema`
- Print deterministic metrics in test output.

## Out of Scope

- No schema mode changes.
- No behavior changes.

## Acceptance Checks

- New baseline test runs and outputs metrics.
- No existing tests regress.

## Delegation Prompt Seed

Implement baseline-only schema metrics test for MCP tool declarations. Do not change runtime behavior. Return changed files and test output.
