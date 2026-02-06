# Lint Error Fix Plan - TDD Approach

## Document Metadata

> Historical note: This document records a completed lint remediation campaign from 2026-02-06 and is kept for reference.

**Created**: 2026-02-06
**Status**: Historical Reference (completed)
**Lint Tool**: golangci-lint v2.8.0
**Total Issues**: 29 (18 errcheck, 11 noctx)
**Approach**: Test-Driven Development (TDD) with Session-Aware Checkpoints
**Approval Type**: Compounded (single approval for entire plan)
**Session Management**: Atomic tasks with checkpoint recovery + /compact auto cleanup
**Document Version**: 1.1

---

## Executive Summary

This document outlines a TDD-based approach to fix 29 golangci-lint errors in the Database MCP Server codebase. All fixes follow Go best practices verified from official documentation and community standards.

**Issue Breakdown:**
- **11 noctx errors** (HIGH PRIORITY): Replace non-context database methods with context-aware versions
- **12 deferred close errcheck errors** (LOW PRIORITY): Add `//nolint:errcheck` comments per Go idiomatic patterns
- **6 error builder errcheck errors** (MEDIUM PRIORITY): Investigate and fix error builder pattern in `errors.go`

**Verification Sources:**
- [Go Official Documentation - Executing Transactions](https://go.dev/doc/database/execute-transactions)
- [Medium: Context in Database Methods](https://medium.com/@phu09032000/should-i-pass-context-context-to-database-methods-in-go-cdace499d594)
- [Blog: Handling Deferred Function Errors](https://trstringer.com/golang-deferred-function-error-handling/)
- [errcheck GitHub Issues #101 & #105](https://github.com/kisielk/errcheck/issues/101)

---

## Session Management & Resilience

This plan is designed to withstand long coding sessions with interruptions. Key features:

### Checkpoint System
- **Atomic Tasks**: Each file/function is an independent task with clear entry/exit points
- **Checkpoint Verification**: After each task, run verification commands to confirm state
- **State Tracking**: Track completed files/functions in progress table
- **Session Recovery**: Clear procedures to resume from any checkpoint
- **Auto-Cleanup**: Use `/compact` command after each checkpoint to clear context

### Progress Tracking

Use this table to track completion (update as you work):

| Phase | File | Line | Status | Last Verified | Notes |
|-------|------|------|--------|---------------|-------|
| Phase 1 | `internal/db/driver.go` | 30 | ⬜ Pending | - | Ping → PingContext |
| Phase 1 | `internal/mcp/server.go` | 663 | ⬜ Pending | - | Query → QueryContext |
| Phase 1 | `internal/mcp/server.go` | 678 | ⬜ Pending | - | Query → QueryContext |
| Phase 1 | `internal/mcp/server.go` | 708 | ⬜ Pending | - | Query → QueryContext |
| Phase 1 | `internal/mcp/server.go` | 2112 | ⬜ Pending | - | Exec → ExecContext |
| Phase 1 | `internal/mcp/server.go` | 2130 | ⬜ Pending | - | Prepare → PrepareContext |
| Phase 1 | `internal/mcp/server.go` | 2148 | ⬜ Pending | - | stmt.Query |
| Phase 1 | `internal/mcp/server.go` | 2258 | ⬜ Pending | - | Prepare → PrepareContext |
| Phase 1 | `internal/mcp/server.go` | 2263 | ⬜ Pending | - | stmt.Exec |
| Phase 1 | `internal/mcp/server.go` | 2281 | ⬜ Pending | - | Exec → ExecContext |
| Phase 1 | `internal/mcp/server.go` | 2417 | ⬜ Pending | - | Exec → ExecContext |
| Phase 2 | `cmd/server/main.go` | 53 | ⬜ Pending | - | defer f.Close() |
| Phase 2 | `internal/config/config.go` | 52 | ⬜ Pending | - | defer f.Close() |
| Phase 2 | `internal/config/config.go` | 92 | ⬜ Pending | - | defer f.Close() |
| Phase 2 | `internal/config/config.go` | 94 | ⬜ Pending | - | defer encoder.Close() |
| Phase 2 | `internal/mcp/query_optimizer.go` | 54 | ⬜ Pending | - | defer conn.Close() |
| Phase 2 | `internal/mcp/query_optimizer.go` | 60 | ⬜ Pending | - | defer rows.Close() |
| Phase 2 | `internal/mcp/server.go` | 619 | ⬜ Pending | - | defer conn.Close() |
| Phase 2 | `internal/mcp/server.go` | 727 | ⬜ Pending | - | defer rows.Close() |
| Phase 2 | `internal/mcp/server.go` | 1290 | ⬜ Pending | - | defer conn.Close() |
| Phase 2 | `internal/mcp/server.go` | 1327 | ⬜ Pending | - | defer rows.Close() |
| Phase 2 | `internal/mcp/server.go` | 2147 | ⬜ Pending | - | defer stmt.Close() |
| Phase 2 | `internal/mcp/server.go` | 2262 | ⬜ Pending | - | defer stmt.Close() |
| Phase 3 | `internal/mcp/errors.go` | 186 | ⬜ Pending | - | WithSuggestions |
| Phase 3 | `internal/mcp/errors.go` | 219 | ⬜ Pending | - | WithContext |
| Phase 3 | `internal/mcp/errors.go` | 240 | ⬜ Pending | - | WithContext |
| Phase 3 | `internal/mcp/errors.go` | 243 | ⬜ Pending | - | WithSuggestions |
| Phase 3 | `internal/mcp/errors.go` | 269 | ⬜ Pending | - | WithContext |
| Phase 3 | `internal/mcp/errors.go` | 293 | ⬜ Pending | - | WithSuggestions |

**Status Key**: ⬜ Pending | 🟡 In Progress | ✅ Complete | ❌ Failed | ⏭️ Skipped

### Session Recovery Procedure

If session is interrupted, follow these steps to resume:

1. **Check Current State**:
   ```bash
   # Run linter to see what's still broken
   golangci-lint run --timeout=5m | tee /tmp/lint-status.txt

   # Run tests to ensure nothing is broken
   go test ./... -v 2>&1 | tee /tmp/test-status.txt
   ```

2. **Review Progress Table**: Find last completed task
3. **Resume from Next Task**: Start with next ⬜ Pending item
4. **Verify Previous Changes**: Run checkpoint verification for completed phase

### Auto-Cleanup Context Commands

After each checkpoint or task, use `/compact` command to clean up accumulated context/state:

#### Cleanup Command (run after each checkpoint):
```
/compact
```

#### Automated Cleanup Workflow:

**After each checkpoint/task**:
1. Run verification: `go test ./... -v`
2. Run linter: `golangci-lint run --timeout=5m`
3. Run cleanup: `/compact`
4. Update progress table
5. Commit checkpoint: `git commit -am "Phase X: checkpoint - [description]"`

**Before starting new session**:
1. Run cleanup: `/compact`
2. Check git status: `git status`
3. Review progress table
4. Resume from next pending task

**After completing all phases**:
1. Run cleanup: `/compact`
2. Full verification: `go test ./... && golangci-lint run --timeout=5m`
3. Final commit: `git commit -am "fix: resolve all 29 lint errors via TDD approach"`

### Mid-Session Verification Commands

Run these after completing each task or file:

```bash
# Quick lint check for specific file
golangci-lint run --timeout=5m internal/db/driver.go

# Run tests for specific package
go test ./internal/db/... -v

# Full verification (use sparingly, slower)
go test ./... && golangci-lint run --timeout=5m

# Auto-cleanup after verification
/compact
```

### Error Recovery

If a task fails:

1. **Immediate Recovery**: Undo the specific file change
   ```bash
   git checkout HEAD -- <file-path>
   ```

2. **Investigate**: Review logs, check test output
3. **Reattempt**: Fix issue and try again
4. **Document**: Add notes to progress table if task is blocked

### Context Preservation

Between sessions, maintain:
- Progress table updates
- Git commit history (don't squash yet)
- Temporary files from checkpoints
- Notes on any decisions or deviations

---

## TDD Approach for Each Fix Type

### General TDD Workflow

For each fix type, we follow this TDD cycle:

1. **Write Failing Test First**
   - Test verifies the lint issue is present (before fix) or resolved (after fix)
   - Test captures the intended behavior
   - Test runs: `go test ./... -v` to confirm failure

2. **Implement Fix**
   - Apply the minimal fix to make test pass
   - Run test: `go test ./... -v` to confirm success

3. **Verify with Linter**
   - Run: `golangci-lint run --timeout=5m`
   - Confirm 0 errors for the specific fix category

4. **Regression Test**
   - Run full test suite: `go test ./...`
   - Verify no test failures

---

## Phase 1: Fix 11 noctx Errors (HIGH PRIORITY)

### Why This Must Be Fixed (Verified by Official Sources)

**From Go Official Documentation:**
> "This example uses `Tx` methods that take a `context.Context` argument. This makes it possible for the function's execution – including database operations – to be canceled if it runs too long or when the client connection closes."

**Critical Benefits of Context-Aware Methods:**
- ✅ **Automatic Request Cancellation** - Cleanup on client disconnect/timeout
- ✅ **Resource Leak Prevention** - Connections returned to pool immediately
- ✅ **Request Tracing** - Request ID, correlation ID, observability
- ✅ **Graceful Shutdown** - Cancel in-flight ops during server shutdown

### Files and Errors

| File | Line | Method | Fix Required |
|------|------|---------|--------------|
| `internal/db/driver.go` | 30 | `db.Ping()` | `db.PingContext(ctx)` |
| `internal/mcp/server.go` | 663 | `conn.Query()` | `conn.QueryContext(ctx, ...)` |
| `internal/mcp/server.go` | 678 | `conn.Query()` | `conn.QueryContext(ctx, ...)` |
| `internal/mcp/server.go` | 708 | `conn.Query()` | `conn.QueryContext(ctx, ...)` |
| `internal/mcp/server.go` | 2112 | `conn.Exec()` | `conn.ExecContext(ctx, ...)` |
| `internal/mcp/server.go` | 2130 | `conn.Prepare()` | `conn.PrepareContext(ctx, ...)` |
| `internal/mcp/server.go` | 2148 | `stmt.Query()` | Use prepared statement with context |
| `internal/mcp/server.go` | 2258 | `conn.Prepare()` | `conn.PrepareContext(ctx, ...)` |
| `internal/mcp/server.go` | 2263 | `stmt.Exec()` | Use prepared statement with context |
| `internal/mcp/server.go` | 2281 | `conn.Exec()` | `conn.ExecContext(ctx, ...)` |
| `internal/mcp/server.go` | 2417 | `conn.Exec()` | `conn.ExecContext(ctx, ...)` |

### TDD Test Strategy

**Test Name Pattern:**
```
func TestDatabaseOperationsUseContext(t *testing.T) {
    // Test verifies context is passed to all database operations
}
```

**Test Coverage Required:**
- ✅ Context passed to Ping/PingContext
- ✅ Context passed to all Query calls
- ✅ Context passed to all Exec calls
- ✅ Context passed to all Prepare calls
- ✅ Context propagation through call chain (handler → service → database)

---

### Atomic Task Breakdown

Each task can be completed independently and verified.

#### Task 1.1: Fix driver.go Ping (Line 30)

**File**: `internal/db/driver.go`
**Current Code**:
```go
if err := db.Ping(); err != nil {
```

**Fix**:
```go
if err := db.PingContext(ctx); err != nil {
```

**Context Check**: Verify function has `ctx context.Context` parameter or add it.

**Verification**:
```bash
# Lint check for this specific file
golangci-lint run --timeout=5m internal/db/driver.go

# Test the db package
go test ./internal/db/... -v
```

**Checkpoint**: Update progress table, commit if complete.

---

#### Task 1.2: Fix server.go Query (Line 663)

**File**: `internal/mcp/server.go`
**Current Code**:
```go
rows, err := conn.Query(query, args...)
```

**Fix**:
```go
rows, err := conn.QueryContext(ctx, query, args...)
```

**Context Check**: Verify function signature has `ctx context.Context`.

**Verification**:
```bash
# Lint check for this specific file
golangci-lint run --timeout=5m internal/mcp/server.go

# Test the mcp package
go test ./internal/mcp/... -v
```

**Checkpoint**: Update progress table, commit if complete.

---

#### Task 1.3: Fix server.go Query (Line 678)

**File**: `internal/mcp/server.go`
**Current Code**:
```go
rows, err := conn.Query(query, args...)
```

**Fix**:
```go
rows, err := conn.QueryContext(ctx, query, args...)
```

**Context Check**: Verify function signature has `ctx context.Context`.

**Verification**:
```bash
golangci-lint run --timeout=5m internal/mcp/server.go
go test ./internal/mcp/... -v
```

**Checkpoint**: Update progress table, commit if complete.

---

#### Task 1.4: Fix server.go Query (Line 708)

**File**: `internal/mcp/server.go`
**Current Code**:
```go
rows, err := conn.Query(query, args...)
```

**Fix**:
```go
rows, err := conn.QueryContext(ctx, query, args...)
```

**Context Check**: Verify function signature has `ctx context.Context`.

**Verification**:
```bash
golangci-lint run --timeout=5m internal/mcp/server.go
go test ./internal/mcp/... -v
```

**Checkpoint**: Update progress table, commit if complete.

---

#### Task 1.5: Fix server.go Exec (Line 2112)

**File**: `internal/mcp/server.go`
**Current Code**:
```go
_, err := conn.Exec(sql, args...)
```

**Fix**:
```go
_, err := conn.ExecContext(ctx, sql, args...)
```

**Context Check**: Verify function signature has `ctx context.Context`.

**Verification**:
```bash
golangci-lint run --timeout=5m internal/mcp/server.go
go test ./internal/mcp/... -v
```

**Checkpoint**: Update progress table, commit if complete.

---

#### Task 1.6: Fix server.go Prepare (Line 2130)

**File**: `internal/mcp/server.go`
**Current Code**:
```go
stmt, err := conn.Prepare(sql)
```

**Fix**:
```go
stmt, err := conn.PrepareContext(ctx, sql)
```

**Context Check**: Verify function signature has `ctx context.Context`.

**Verification**:
```bash
golangci-lint run --timeout=5m internal/mcp/server.go
go test ./internal/mcp/... -v
```

**Checkpoint**: Update progress table, commit if complete.

---

#### Task 1.7: Fix server.go stmt.Query (Line 2148)

**File**: `internal/mcp/server.go`
**Current Code**:
```go
rows, err := stmt.Query(args...)
```

**Fix**: Prepared statement already created with context, so no change needed. Verify that `stmt` was created with `PrepareContext(ctx, ...)`.

**Context Check**: Ensure the surrounding prepare statement uses context.

**Verification**:
```bash
golangci-lint run --timeout=5m internal/mcp/server.go
go test ./internal/mcp/... -v
```

**Checkpoint**: Update progress table, commit if complete.

---

#### Task 1.8: Fix server.go Prepare (Line 2258)

**File**: `internal/mcp/server.go`
**Current Code**:
```go
stmt, err := conn.Prepare(sql)
```

**Fix**:
```go
stmt, err := conn.PrepareContext(ctx, sql)
```

**Context Check**: Verify function signature has `ctx context.Context`.

**Verification**:
```bash
golangci-lint run --timeout=5m internal/mcp/server.go
go test ./internal/mcp/... -v
```

**Checkpoint**: Update progress table, commit if complete.

---

#### Task 1.9: Fix server.go stmt.Exec (Line 2263)

**File**: `internal/mcp/server.go`
**Current Code**:
```go
result, err := stmt.Exec(args...)
```

**Fix**: Prepared statement already created with context, so no change needed. Verify that `stmt` was created with `PrepareContext(ctx, ...)`.

**Context Check**: Ensure the surrounding prepare statement uses context.

**Verification**:
```bash
golangci-lint run --timeout=5m internal/mcp/server.go
go test ./internal/mcp/... -v
```

**Checkpoint**: Update progress table, commit if complete.

---

#### Task 1.10: Fix server.go Exec (Line 2281)

**File**: `internal/mcp/server.go`
**Current Code**:
```go
_, err := conn.Exec(sql, args...)
```

**Fix**:
```go
_, err := conn.ExecContext(ctx, sql, args...)
```

**Context Check**: Verify function signature has `ctx context.Context`.

**Verification**:
```bash
golangci-lint run --timeout=5m internal/mcp/server.go
go test ./internal/mcp/... -v
```

**Checkpoint**: Update progress table, commit if complete.

---

#### Task 1.11: Fix server.go Exec (Line 2417)

**File**: `internal/mcp/server.go`
**Current Code**:
```go
_, err := conn.Exec(sql, args...)
```

**Fix**:
```go
_, err := conn.ExecContext(ctx, sql, args...)
```

**Context Check**: Verify function signature has `ctx context.Context`.

**Verification**:
```bash
golangci-lint run --timeout=5m internal/mcp/server.go
go test ./internal/mcp/... -v
```

**Checkpoint**: Update progress table, commit if complete.

---

### Auto-Cleanup Integration in Tasks

**For ALL tasks in all phases, include this workflow**:

```bash
# After completing task fix:
1. Run verification: go test ./... -v
2. Run linter: golangci-lint run --timeout=5m
3. Run cleanup: /compact
4. Update progress table in this document
5. Git commit: git commit -am "Phase X: checkpoint - [task description]"
```

This ensures context doesn't accumulate and sessions stay lightweight.

#### Phase 1 Completion & Cleanup

After completing all 11 tasks:

```bash
# 1. Final verification for Phase 1
golangci-lint run --timeout=5m | grep "noctx" | wc -l
# Expected output: 0

# 2. Run all tests
go test ./... -v

# 3. Auto-cleanup context
/compact

# 4. Commit Phase 1 completion
git add .
git commit -m "fix(phase1): add context to 11 database operations (TDD)

- Replaced db.Ping() with db.PingContext(ctx) in driver.go
- Replaced 10 Query/Exec/Prepare calls with Context variants in server.go
- All noctx lint errors resolved (0 remaining)
- All tests passing"

echo "✅ Phase 1 Complete: noctx errors resolved"
```

**Phase 1 Success Criteria**:
- ✅ 0 noctx errors from linter
- ✅ All tests pass
- ✅ Context propagated through all database operations
- ✅ No regressions in functionality

### Phase 1 Final Verification

After completing all tasks:

```bash
# Verify all noctx errors are resolved
golangci-lint run --timeout=5m | grep "noctx" | wc -l
# Expected output: 0

# Run all tests
go test ./... -v

# Quick status check
echo "Phase 1 complete: noctx errors resolved"
```

---

## Phase 2: Fix 12 Deferred Close errcheck Errors (LOW PRIORITY)

### Why nolint is Appropriate (Verified by Community Standards)

**From errcheck Maintainer Issues #101 & #105:**
> "defer resp.Body.Close() is perfectly cromulent and there's no need for error checking contortions to satisfy lint tool."

**Idiomatic Go Pattern:**
- Standard practice is to ignore deferred close errors
- Main operation success/failure is more important
- Close errors in deferred context are typically non-critical

### Files and Errors

| File | Line | Resource | Pattern |
|------|------|-----------|----------|
| `cmd/server/main.go` | 53 | `f.Close()` | `defer f.Close() //nolint:errcheck` |
| `internal/config/config.go` | 52 | `f.Close()` | `defer f.Close() //nolint:errcheck` |
| `internal/config/config.go` | 92 | `f.Close()` | `defer f.Close() //nolint:errcheck` |
| `internal/config/config.go` | 94 | `encoder.Close()` | `defer encoder.Close() //nolint:errcheck` |
| `internal/mcp/query_optimizer.go` | 54 | `conn.Close()` | `defer conn.Close() //nolint:errcheck` |
| `internal/mcp/query_optimizer.go` | 60 | `rows.Close()` | `defer rows.Close() //nolint:errcheck` |
| `internal/mcp/server.go` | 619 | `conn.Close()` | `defer conn.Close() //nolint:errcheck` |
| `internal/mcp/server.go` | 727 | `rows.Close()` | `defer rows.Close() //nolint:errcheck` |
| `internal/mcp/server.go` | 1290 | `conn.Close()` | `defer conn.Close() //nolint:errcheck` |
| `internal/mcp/server.go` | 1327 | `rows.Close()` | `defer rows.Close() //nolint:errcheck` |
| `internal/mcp/server.go` | 2147 | `stmt.Close()` | `defer stmt.Close() //nolint:errcheck` |
| `internal/mcp/server.go` | 2262 | `stmt.Close()` | `defer stmt.Close() //nolint:errcheck` |

### TDD Test Strategy

**Test Name Pattern:**
```
func TestDeferredResourceCleanup(t *testing.T) {
    // Test verifies resources are properly cleaned up
    // Note: Close errors in defer are intentionally ignored per Go idioms
}
```

**Test Coverage Required:**
- ✅ Resources are properly acquired and closed
- ✅ No resource leaks (verify with connection pool metrics)
- ✅ Functions handle cleanup correctly in success/error paths

### Implementation Steps

**Step 2.1: Add nolint comment to all deferred close operations**
```go
// Pattern
defer resource.Close() //nolint:errcheck // Standard pattern: error in deferred close is not critical
```

**Step 2.2: Verify errcheck errors reduced**
```bash
golangci-lint run --timeout=5m | grep "defer.*Close" | wc -l
# Expected output: 0
```

### Phase 2 Completion & Cleanup

After completing all 12 tasks:

```bash
# 1. Final verification for Phase 2
golangci-lint run --timeout=5m | grep "defer.*Close" | wc -l
# Expected output: 0

# 2. Run all tests
go test ./... -v

# 3. Auto-cleanup context
/compact

# 4. Commit Phase 2 completion
git add .
git commit -m "fix(phase2): add nolint comments to 12 deferred close operations

- Added //nolint:errcheck to all deferred Close() calls
- Follows idiomatic Go pattern per errcheck maintainer guidance
- All deferred close errcheck errors resolved (0 remaining)
- All tests passing"

echo "✅ Phase 2 Complete: deferred close errors resolved"
```

**Phase 2 Success Criteria**:
- ✅ 0 deferred close errcheck errors from linter
- ✅ All tests pass
- ✅ Idiomatic Go error handling pattern applied
- ✅ No functional changes (just nolint comments)

---

## Phase 3: Fix 6 Error Builder errcheck Errors (MEDIUM PRIORITY)

### Investigation Required

These errors are in `internal/mcp/errors.go` and involve error builder methods:
- `err.WithContext()` (3 occurrences: lines 219, 240, 269)
- `err.WithSuggestions()` (3 occurrences: lines 186, 243, 293)

**Question:** Does the error builder pattern return a new error or modify in-place?

### TDD Investigation Steps

**Step 3.1: Read errors.go to understand builder pattern**
```go
// Example investigation
type MCPErr struct {
    msg string
    ctx map[string]interface{}
    suggestions []string
}

func (e *MCPErr) WithContext(key string, value interface{}) error {
    // Does this return *MCPErr or modify in-place?
}
```

**Step 3.2: Write test to verify builder behavior**
```go
func TestErrorBuilderPattern(t *testing.T) {
    err := NewMCPErr("test error")

    // Test 1: Does WithContext return new error?
    newErr := err.WithContext("key", "value")
    if newErr == err {
        t.Error("WithContext should return new error")
    }

    // Test 2: Does WithSuggestions return new error?
    newErr = err.WithSuggestions("suggestion1", "suggestion2")
    if newErr == err {
        t.Error("WithSuggestions should return new error")
    }
}
```

**Step 3.3: Apply fix based on investigation**
```go
// Option A: Builder returns new error (capture it)
err = err.WithContext("key", value)

// Option B: Builder modifies in-place (add nolint)
err.WithContext("key", "value") //nolint:errcheck // Builder modifies in-place
```

### Implementation Steps

**Step 3.1**: Read `internal/mcp/errors.go` to understand builder implementation
**Step 3.2**: Write test to verify builder pattern
**Step 3.3**: Apply appropriate fix (capture return or add nolint)
**Step 3.4**: Verify with linter and tests

### Phase 3 Completion & Cleanup

After completing all 6 tasks:

```bash
# 1. Final verification for Phase 3
golangci-lint run --timeout=5m | grep "errors.go" | wc -l
# Expected output: 0 (no errcheck errors for WithContext/WithSuggestions)

# 2. Run all tests
go test ./... -v

# 3. Auto-cleanup context
/compact

# 4. Commit Phase 3 completion
git add .
git commit -m "fix(phase3): resolve 6 error builder errcheck errors

- Investigated error builder pattern in errors.go
- Fixed 3 WithContext() calls (lines 219, 240, 269)
- Fixed 3 WithSuggestions() calls (lines 186, 243, 293)
- All error builder errcheck errors resolved (0 remaining)
- All tests passing"

echo "✅ Phase 3 Complete: error builder errors resolved"
```

**Phase 3 Success Criteria**:
- ✅ 0 error builder errcheck errors from linter
- ✅ All tests pass
- ✅ Builder pattern correctly implemented
- ✅ Error handling behavior verified

---

## Verification Plan

### Pre-Fix Verification

```bash
# Confirm current error count
golangci-lint run --timeout=5m | grep "issues:" | head -1
# Expected: 29 issues (18 errcheck, 11 noctx)
```

### Post-Fix Verification

```bash
# Step 1: Run all tests
go test ./... -v

# Step 2: Run linter to confirm 0 errors
golangci-lint run --timeout=5m

# Step 3: Verify test coverage
go test -cover ./...

# Step 4: Run go vet
go vet ./...

# Step 5: Verify gofmt
gofmt -l . | head -20

# Step 6: Final cleanup
/compact

# Step 7: Final commit
git add .
git commit -m "feat: complete lint error fixes - all 29 errors resolved via TDD

Summary of changes:
- Phase 1: Fixed 11 noctx errors (added context to DB operations)
- Phase 2: Fixed 12 deferred close errcheck errors (added nolint comments)
- Phase 3: Fixed 6 error builder errcheck errors (investigated & fixed)

Verification:
- 0 lint errors from golangci-lint
- All tests passing
- No regressions
- Follows Go best practices and idiomatic patterns"
```

### Success Criteria

✅ All 29 lint errors resolved (0 issues from golangci-lint)
✅ All tests pass (`go test ./...`)
✅ No regressions in existing functionality
✅ Test coverage maintained or improved
✅ Code follows Go best practices and idiomatic patterns

---

## Risk Assessment

### Low Risk
- **Phase 2** (deferred close nolint): Standard Go pattern, no functional changes

### Medium Risk
- **Phase 3** (error builder): Requires investigation, behavior may vary

### High Risk
- **Phase 1** (context propagation): Requires careful context passing through call chain

### Mitigation Strategies

1. **Test Coverage**: Write comprehensive tests before fixing
2. **Incremental Changes**: Fix one file at a time, verify after each
3. **Rollback Plan**: Keep git commit history clean, easy to revert
4. **Peer Review**: All changes reviewed before merging

---

## Estimated Timeline

| Phase | Time Estimate | Dependencies |
|--------|---------------|--------------|
| Phase 1 (noctx) | 2-3 hours | Test context propagation |
| Phase 2 (defer nolint) | 30 minutes | None |
| Phase 3 (error builder) | 1-2 hours | Investigation of errors.go |
| **Total** | **3.5-5.5 hours** | - |

---

## Rollback Plan

If issues arise during implementation:

1. **Phase Rollback**: Revert specific phase changes using git
   ```bash
   git revert <commit-hash>
   ```

2. **File Rollback**: Revert individual files if needed
   ```bash
   git checkout HEAD -- <file-path>
   ```

3. **Full Rollback**: Revert all changes to pre-fix state
   ```bash
   git reset --hard HEAD~1
   ```

---

## Compounded Approval Request

**This plan requests a single compounded approval to fix all 29 lint errors using a TDD approach.**

**Approval covers:**
1. Phase 1: Fix 11 noctx errors (add context parameters to database operations)
2. Phase 2: Add nolint comments to 12 deferred close operations (idiomatic Go pattern)
3. Phase 3: Investigate and fix 6 error builder errcheck errors in errors.go
4. Write tests for all changes (TDD approach)
5. Verify fixes with golangci-lint, go test, go vet, gofmt

**Expected Outcome:**
- ✅ 0 lint errors from golangci-lint
- ✅ All tests passing
- ✅ Context-safe database operations
- ✅ Idiomatic Go error handling patterns
- ✅ No regressions in existing functionality

**Do you approve this plan to fix all 29 lint errors?**

---

## References

1. **Go Official Documentation** - [Executing Transactions](https://go.dev/doc/database/execute-transactions)
2. **Community Best Practices** - [Context in Database Methods](https://medium.com/@phu09032000/should-i-pass-context-context-to-database-methods-in-go-cdace499d594)
3. **Error Handling** - [Handling Deferred Function Errors](https://trstringer.com/golang-deferred-function-error-handling/)
4. **errcheck Issues** - [Issue #101](https://github.com/kisielk/errcheck/issues/101), [Issue #105](https://github.com/kisielk/errcheck/issues/105)
5. **Project Standards** - [AGENTS.md](/media/eddy/hdd/Project/Database_MCP_Server/AGENTS.md)
6. **Testing Standards** - [Standards: Code & Tests](/home/eddy/distrobox/box-go-debian-home/.config/opencode/context/core/standaries/code.md)

---

**Document Version:** 1.1
**Last Updated:** 2026-02-06
**Status:** ⏳ Pending Approval

**Change Log:**
- v1.1 (2026-02-06): Added /compact command for auto cleanup context and session management
- v1.0 (2026-02-06): Initial TDD-based lint fix plan
