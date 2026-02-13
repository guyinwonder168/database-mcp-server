# P5 - CI Schema Budget Gates

## Objective

Enforce schema quality budgets in CI to prevent regressions.

## In Scope

- `internal/mcp/schema_contract_test.go`
- `.github/workflows/ci.yml`

## Gates

- `GATE_SCHEMA_001`: 100% tools define `InputSchema`
- `GATE_SCHEMA_002`: max compact description length <= 160
- `GATE_SCHEMA_003`: no multiline escapes in compact descriptions
- `GATE_SCHEMA_004`: no embedded JSON examples in compact descriptions
- `GATE_SCHEMA_005`: serialized compact `tools/list` payload <= 12 KB

## Tasks

- Encode gates as deterministic tests.
- Wire tests into CI blocking path.
- Keep validation sequence stable (no parallel flaky checks).

## Out of Scope

- No provider integration here.

## Acceptance Checks

- CI fails on intentional budget violation.
- CI passes on compliant state.

## Delegation Prompt Seed

Implement and wire schema budget gates to CI using deterministic tests and fail-fast behavior.
