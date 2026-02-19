# P7 - Docs, Migration, and Rollout

## Objective

Ship clear user-facing guidance for compact mode and helper tool usage.

## In Scope

- `README.md`
- `docs/mcp-examples.md` (or current examples file)
- `CHANGELOG.md`

## Tasks

- Document `schema_mode` (`compact` default, `standard` optional).
- Document `get-tool-help` usage and examples.
- Move rich payload examples out of tool descriptions into docs.
- Add release notes for compatibility change.

## Out of Scope

- No code logic changes.

## Acceptance Checks

- Binary-only users can discover examples via helper tool.
- README and changelog are aligned with shipped behavior.

## Delegation Prompt Seed

Update docs and changelog for compact schema mode and `get-tool-help`, ensuring examples are discoverable without repository browsing.
