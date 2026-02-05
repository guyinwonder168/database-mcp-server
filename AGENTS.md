# Repository Guidelines

## Project Structure & Module Organization
- Entry point: `cmd/server/main.go` builds the MCP server binary.
- Core logic lives in `internal/`: `mcp/` (tools, handlers, tests), `config/` (YAML loading & AES key handling), `db/` (drivers, pooling), `log/` (structured logging).
- Shared utilities sit in `pkg/` (e.g., `pkg/log/`); local testing artifacts may also live in `pkg/` and should not be committed unless intended. Docs in `docs/` (API, specs, roadmap) and planning notes in `project-plan/`. Logs rotate into `log/`.
- AI memory-bank rules reside in `.kilocode/rules/`—avoid modifying unless you know the policy intent.

## Build, Test, and Development Commands
- `go build -o ./tmp/mcp-server ./cmd/server/main.go` – compile the server binary.
- `./tmp/mcp-server` – run with `config.yaml` (auto-created on first run).
- `go test ./...` – unit test suite (fast, default).
- `go test -cover ./...` – coverage run.
- `DB_MCP_IT_PG_HOST=... DB_MCP_IT_PG_DB=... go test ./internal/mcp -run TestLive -count=1` – optional live integration tests against Postgres; similar envs exist for other DBs.
- `go vet ./...` and `gofmt -w .` – required before pushing.
- Follow Go best practice, use internet to search if needed.
- IMPORTANT!: DO NOT OVER ENGINEERING!

## Coding Style & Naming Conventions
- Go 1.25.5 toolchain; enforce `gofmt` and idiomatic Go patterns.
- Packages/directories are lower_snake; exported identifiers use PascalCase; tests mirror source package names.
- Prefer context-aware functions (`ctx` first parameter) and structured errors wrapped with context.
- Logging: use the JSON logger in `internal/log`; avoid `fmt.Printf` in production paths.

## Testing Guidelines
- Place tests alongside code (`*_test.go`); use table-driven cases where possible.
- Mock DB interactions when feasible; reserve live tests for `integration_live_test.go`.
- Aim to keep coverage steady; add regression tests for bug fixes.

## Commit & Pull Request Guidelines
- Follow Conventional Commits (`feat:`, `fix:`, `chore:`, `docs:`). Example: `feat: add sqlite pool timeouts`.
- Branch naming: `feature/<slug>` or `fix/<slug>`.
- PRs should include: summary, linked issue, test commands run, config notes (if touching `config.yaml`), and screenshots/log excerpts when behavior changes.
- Ensure `go fmt`, `go vet`, and `go test ./...` pass before requesting review.

## Security & Configuration Tips
- Never commit secrets; `config.yaml` holds AES-256 `aes_key` for credential encryption—keep it local or use env injection.
- Validate user input before executing SQL; prefer prepared statements via existing DB helpers.
- Logs are rotated via `file-rotatelogs`; avoid logging raw credentials.

## Agent-Specific Notes
- **Context files location**: Standards and workflows are in `/home/eddy/distrobox/box-go-debian-home/.config/opencode/context/` - always load relevant context files before executing code/docs/tests tasks
  - Code tasks → `standards/code.md` (MANDATORY for any code changes)
  - Docs tasks → `standards/docs.md` (MANDATORY for any documentation)
  - Tests tasks → `standards/tests.md` (MANDATORY for any test changes)
  - Review tasks → `workflows/review.md` (MANDATORY for code reviews)
  - Delegation → `workflows/delegation.md` (MANDATORY when using task tool)
- Prefer `rg` for searches; keep edits minimal and commented only when non-obvious.
- If adding MCP tools, register in `internal/mcp` and update docs (`docs/mcp-openapi.yaml`, `README.md`), then add tests in `internal/mcp/server_test.go`.
- Review and update `./README.md`, `./CHANGELOG.md`, and `./docs/` after code changes; update `./kilocode/memory-bank/` per `.kilocode/rules/memory-bank-instructions.md`.
- Codex, VSCode were installed inside distrobox container, the go were installed using GVM so in every session start always run this first:
`source ~/.gvm/scripts/gvm`

## Learning from Mistakes

### Mistake #1: Go Version Assumption Without Verification (2026-02-05)
**What happened**: Incorrectly assumed Go 1.25.5 didn't exist and lowered go.mod version to 1.23 without checking official sources first. Later verified via go.dev/dl/ that Go 1.25.7 (latest stable) DOES exist.

**Root cause**: Made technical assumption without verification from authoritative sources (go.dev/dl/).

**Lesson learned**:
- ALWAYS verify technical facts from official sources (go.dev, GitHub docs, etc.) before making version/toolchain changes
- When unsure about version availability, check official documentation/websites first
- Version format: `go 1.X` in go.mod, `go1.X.Y` as toolchain - they must match
- The issue was actually the CI workflow having `go-version: ['1.25.5']` (wrong format) not the go.mod file

**Action taken**: Reverted change and fixed the actual issue (CI workflow version format), documented this learning to avoid repetition.

### Mistake #2: Relying on Internal Knowledge Without Verification (2026-02-05)
**What happened**: Made changes based on internal model knowledge/training data without first verifying facts from internet sources. This led to incorrect Go version assumptions and wasted time fixing non-issues.

**Root cause**: Trusted internal training data over real-time, authoritative sources.

**Lesson learned**:
- ALWAYS search the internet for current, accurate information before making changes
- Internal model knowledge has a cutoff date - use internet to get latest info
- Verify facts from multiple authoritative sources (official docs, GitHub repos, etc.)
- Search first, act second - never assume without verification
- Use webfetch/tavily-search/google-search tools for real-time information
- When uncertain: search → verify → then act (not act → discover → fix)

**Action taken**: Documented this principle in AGENTS.md to ensure all future tasks follow verification-first approach.

---

## Critical Rules for All Agents

### Verification-First Principle (MANDATORY)
When making technical decisions or changes:

1. **SEARCH FIRST**: Use webfetch, tavily-search, or google-search to get current information
   - Check official documentation
   - Check GitHub repositories/issues
   - Verify version compatibility
   - Cross-reference multiple sources

2. **VERIFY**: Confirm facts before acting
   - Does this version actually exist?
   - Is this the current best practice?
   - Are there any recent changes/deprecations?

3. **ACT**: Only after verification is complete
   - Make changes based on verified facts
   - Document sources of information
   - Keep track of what was verified

4. **DOCUMENT**: Record what was learned
   - Update AGENTS.md with lessons learned
   - Note sources of truth
   - Avoid repeating same mistakes

### Examples of When to Search:
- ✅ Version numbers/compatibility (Go, libraries, tools)
- ✅ API changes/deprecations
- ✅ Best practices for frameworks/libraries
- ✅ New features or breaking changes
- ✅ Error messages and their solutions
- ✅ Security vulnerabilities or patches

### Sources of Truth (in priority order):
1. **Official documentation** (go.dev, GitHub docs, etc.)
2. **Official repositories** (GitHub, GitLab, etc.)
3. **Recent issues/PRs** (for current problems)
4. **Community discussions** (Stack Overflow, forums) - cross-verify

### Anti-Patterns to Avoid:
- ❌ Trusting internal model knowledge without verification
- ❌ Making assumptions about version availability
- ❌ Following outdated tutorials/guides
- ❌ Guessing API signatures/behavior
- ❌ Skipping web search for "quick" tasks

### Approval Preference
- Prefer compounded approval for multi-step operations (single approval covering the planned sequence), rather than per-step approvals.
