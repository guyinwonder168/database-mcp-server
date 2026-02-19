# P2 - Compact Tool Descriptions

## Objective

Serve concise one-line descriptions in `compact` mode to reduce declaration payload size.

## In Scope

- `internal/mcp/server.go`
- optional small helper file for description selection

## Tasks

- Define compact descriptions for all tools.
- Keep `standard` mode for richer descriptions.
- Remove inline JSON examples from compact descriptions.
- Ensure compact descriptions do not contain `\n` or `\t`.

## Out of Scope

- No CI gate wiring.

## Acceptance Checks

- Compact mode descriptions are single-line and short.
- Standard mode remains functional.

## Delegation Prompt Seed

Implement compact-vs-standard tool description selection with compact as one-line, no embedded examples, no multiline escapes.
