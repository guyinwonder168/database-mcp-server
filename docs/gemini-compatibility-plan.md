# Gemini Compatibility Plan (Industry-Standard)

## Executive Summary

When `mcp_database` is enabled, Gemini-based clients can fail during tool loading due to oversized and overly verbose tool declarations.

This plan adopts a production-standard approach:

1. Compact-by-default tool contract
2. Explicit `InputSchema` on every tool
3. CI schema budget gates
4. Cross-provider compatibility smoke tests
5. Single helper tool for on-demand examples and troubleshooting

This keeps behavior stable for Claude while making Gemini integration reliable and repeatable.

## Facts and Standards Used

- Gemini function-calling guidance emphasizes clear declarations and structured schemas; declarations are part of model input context and therefore affect prompt size.
- MCP tools spec defines `name`, `description`, and `inputSchema` as core tool contract fields.

References:
- https://ai.google.dev/gemini-api/docs/function-calling
- https://modelcontextprotocol.io/specification/2025-06-18/server/tools

## Current State (Repository)

- Tool descriptions are long and include embedded examples/newlines in `internal/mcp/server.go`.
- Several tools rely too much on prose for argument shape instead of strict schema-first declarations.
- No configuration switch exists yet for compact vs verbose schema delivery.
- Current tool inventory in README is 18 tools (not 15).

## Goals

- Prevent Gemini tool-loading failures when `mcp_database` is enabled.
- Preserve full feature parity and behavior across providers.
- Make compatibility enforceable in CI, not dependent on manual discipline.

## Non-Goals

- Redesigning tool business logic.
- Removing tools from the default catalog.
- Adding provider-specific runtime branching beyond schema format selection.
- Duplicating examples across every tool description.

## Target Architecture

### 1) Compact-By-Default Contract

- Introduce `schema_mode` with two values:
  - `compact` (default): short descriptions, schema-first metadata
  - `standard`: longer descriptions for human readability
- Keep parameter examples out of tool `description`; move examples to docs.
- Tool descriptions should be one short sentence focused on purpose and constraints.

### 2) Explicit Schemas for All Tools

- Every registered tool must have `InputSchema`.
- Required vs optional fields must be encoded in schema, not only narrative text.
- Keep schema strict, shallow, and type-accurate; avoid ambiguous free-form fields unless required.

### 3) CI Budget Gates

Add deterministic checks that fail the build if contract size regresses.

Recommended gates for `compact` mode:

- `GATE_SCHEMA_001`: 100% tools define `InputSchema`
- `GATE_SCHEMA_002`: max description length per tool <= 160 chars
- `GATE_SCHEMA_003`: no multiline descriptions (`\n`, `\t`) in compact mode
- `GATE_SCHEMA_004`: no embedded JSON examples in compact descriptions
- `GATE_SCHEMA_005`: serialized `tools/list` payload size <= 12 KB

Notes:
- These are project quality budgets (not claimed Gemini hard limits).
- Thresholds can be adjusted after baseline measurement, but must remain enforced in CI.

### 4) Compatibility Smoke Tests

- Add provider smoke tests that validate:
  - server starts
  - `tools/list` succeeds
  - at least one representative tool call succeeds
- Run for at least Gemini and Claude integration paths.
- Keep these tests lightweight and deterministic.

### 5) Progressive Disclosure via Single Helper Tool

- Add one compact helper tool: `get-tool-help`.
- Purpose: provide on-demand usage examples and troubleshooting without inflating every tool declaration.
- Keep this tool schema explicit and compact so it remains reliable across tool-first clients.
- Optionally expose richer MCP resources (for clients that support resources well), but do not rely on resources alone for compatibility.

Proposed contract:

- Input:
  - `tool_name` (required, string)
  - `topic` (optional, enum: `summary|minimal_example|advanced_example|errors|all`)
- Output:
  - `tool_name`
  - `summary`
  - `minimal_example` (JSON object)
  - `advanced_example` (JSON object, optional)
  - `common_errors` (array of `{error, cause, fix}`)
  - `notes` (array of strings)

## Implementation Plan

### Phase 0: Baseline and Instrumentation

- Measure current `tools/list` payload bytes and per-tool description lengths.
- Record baseline in test output artifacts.
- Add helper to serialize current tool declarations for budget checks.

Deliverables:
- Baseline metrics file/check output
- Initial failing tests that define budget expectations

### Phase 1: Schema Contract Hardening

- Update tool registration in `internal/mcp/server.go`:
  - compact descriptions
  - explicit `InputSchema` for all tools
- Add helper tool registration:
  - `get-tool-help` with explicit schema and compact description
  - internal map/registry of tool help content (summary, examples, common errors)
- Add schema mode to config (`compact` default, `standard` optional).
- Validate config values on startup.

Deliverables:
- Stable tool declarations in both modes
- Backward-compatible runtime behavior
- On-demand examples/troubleshooting available without bloating core tool descriptions

### Phase 2: CI Gates

- Add unit tests/lint checks for `GATE_SCHEMA_001..005`.
- Wire checks into CI so merge is blocked on failure.
- Keep checks sequential with existing repo validation flow.

Deliverables:
- Automated contract regression protection

### Phase 3: Provider Smoke Tests

- Add smoke harness and commands for Gemini + Claude.
- Validate startup + discovery + representative tool call.
- Store logs/errors for quick triage.

Deliverables:
- Compatibility confidence before release

### Phase 4: Rollout and Monitoring

- Release with `compact` as default.
- Document migration for users who want verbose metadata.
- Watch error rates for tool discovery/call failures.

Rollback criteria:
- If compact mode introduces functional regressions, switch default to `standard` temporarily via config while maintaining CI gates and fixing root cause.

## File-Level Change Plan

- `internal/mcp/server.go`
  - compact descriptions for all tools
  - explicit `InputSchema` coverage at 100%
  - register `get-tool-help`
- `internal/mcp/tool_help.go` (new)
  - structured help catalog for each tool (summary/examples/errors)
- `internal/mcp/tool_help_test.go` (new)
  - coverage for helper tool inputs, unknown tool handling, and output shape
- `internal/config/config.go`
  - add `SchemaMode` field and validation
- `config.yaml`
  - set default `schema_mode: compact`
- `internal/mcp/*_test.go` (or dedicated schema contract tests)
  - add schema budget and schema coverage tests
- `.github/workflows/ci.yml`
  - include schema budget + smoke test jobs (as appropriate)
- `README.md`
  - document schema mode and compatibility behavior
  - document `get-tool-help` usage
- `docs/mcp-examples.md` (or equivalent)
  - move rich examples out of tool descriptions

## Verification Strategy

### Automated

- Unit: schema coverage and budget gates
- Unit: helper tool schema/output tests
- Integration: existing Go test suite remains green
- Compatibility smoke: Gemini and Claude discovery + one call path each

Suggested command sequence (sequential):

```bash
go test ./...
go vet ./...
go test -run TestSchemaContract -v ./internal/mcp/...
go test -run TestProviderSmoke -v ./internal/mcp/...
```

### Manual

For each provider client:

1. Enable `mcp_database`
2. Trigger tool loading
3. Confirm no agent termination during discovery
4. Run `list-tools`
5. Execute one representative DB action

Success means discovery and first call are both stable.

## Acceptance Criteria

- No Gemini crash on tool discovery with `schema_mode=compact`
- 100% tools define explicit `InputSchema`
- CI enforces schema budgets and blocks regressions
- Claude compatibility remains intact
- Documentation updated and examples moved out of descriptions
- `get-tool-help` returns valid minimal example and troubleshooting for each public tool

## Risks and Mitigations

- Risk: Over-tight budgets create noisy failures
  - Mitigation: establish baseline first; tune once, then freeze thresholds
- Risk: Missing schema on newly added tools in future PRs
  - Mitigation: keep `GATE_SCHEMA_001` mandatory in CI
- Risk: Provider behavior changes over time
  - Mitigation: smoke tests on every release branch
- Risk: Helper content drifts from actual schema
  - Mitigation: add contract tests that validate helper examples against tool `InputSchema`

## Ownership and Release Checklist

- Engineering:
  - implement schema mode + explicit schemas
  - add CI gates and smoke tests
  - implement and maintain `get-tool-help` catalog/tests
- Documentation:
  - update README and usage examples
- Release:
  - include compatibility notes in CHANGELOG
  - verify CI gates pass before tag/release

## Optional Future Improvements

- Add schema budget report artifact in CI for trend tracking.
- Add canary provider smoke run on pre-release tags.
- Add optional tool catalog profiles (basic/advanced) only if real demand appears.
