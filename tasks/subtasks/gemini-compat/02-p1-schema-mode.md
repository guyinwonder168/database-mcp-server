# P1 - Schema Mode Plumbing

## Objective

Add and validate `schema_mode` with `compact` default and `standard` fallback mode.

## In Scope

- `internal/config/config.go`
- `config.yaml`
- relevant config tests

## Tasks

- Add `SchemaMode` to config model.
- Supported values: `compact`, `standard`.
- Default to `compact` when unset.
- Return clear error for invalid values.

## Out of Scope

- No tool description rewrite yet.

## Acceptance Checks

- Unset mode resolves to `compact`.
- Invalid mode fails with clear error.
- Existing config behavior remains stable.

## Delegation Prompt Seed

Implement schema mode config plumbing only (`compact|standard`, default compact). Add tests and keep all unrelated behavior unchanged.
