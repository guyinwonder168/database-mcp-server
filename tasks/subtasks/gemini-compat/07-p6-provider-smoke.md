# P6 - Provider Smoke Tests

## Objective

Validate discovery and first tool call stability for Gemini and Claude client paths.

## In Scope

- smoke test harness under `internal/mcp/` or scripts wrapper
- CI job hook (optional if env-gated)

## Tasks

- Add lightweight smoke checks for:
  - server startup
  - `tools/list` success
  - one representative tool call success
- Capture concise logs/artifacts for triage.

## Out of Scope

- Full end-to-end conversational testing.

## Acceptance Checks

- Smoke checks are deterministic and fast.
- Failures provide actionable logs.

## Delegation Prompt Seed

Create lightweight provider smoke checks for discovery and first call success. Keep tests deterministic and produce useful failure logs.
