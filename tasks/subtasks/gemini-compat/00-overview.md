# Gemini Compatibility Delegation Pack (200k-Context Safe)

## Objective

Execute the Gemini compatibility initiative safely with small, verifiable packets that can be delegated to subagents without context overflow.

## Rules

- One packet equals one objective.
- Max 8 files per packet.
- Pass only relevant snippets and constraints, not full repository dumps.
- Stop on failures; report and request approval before fixing.
- Keep validation sequential.

## Packet Order

1. `01-p0-baseline.md`
2. `02-p1-schema-mode.md`
3. `03-p2-compact-descriptions.md`
4. `04-p3-inputschema-coverage.md`
5. `05-p4-helper-tool.md`
6. `06-p5-ci-gates.md`
7. `07-p6-provider-smoke.md`
8. `08-p7-docs-rollout.md`

## Global Acceptance

- `schema_mode=compact` is default.
- 100% tools have explicit `InputSchema`.
- Compact schema gates enforced in CI.
- `get-tool-help` exists and is tested.
- Gemini/Claude smoke checks pass.
