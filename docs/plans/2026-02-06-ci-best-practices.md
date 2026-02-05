# CI Best Practices Fix Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix all CI configuration issues to align with Go best practices — golangci-lint v2 config migration, security scanning improvements, duplicate test elimination, and missing checks.

**Architecture:** Three files are modified: `.golangci.yml` (v1→v2 migration), `.github/workflows/ci.yml` (security, coverage, permissions fixes), and `Makefile` (align local targets with CI changes).

**Tech Stack:** GitHub Actions, golangci-lint v2, govulncheck, gosec, Go 1.25

---

### Task 1: Migrate `.golangci.yml` from v1 to v2 format

**Files:**
- Modify: `.golangci.yml` (entire file)

**Step 1: Rewrite `.golangci.yml` to v2 format**

Replace the entire file with v2 syntax. Key changes:
- `linters.disable-all: true` → `linters.default: none`
- `linters-settings:` → `linters.settings:`
- `issues.exclude-dirs/exclude-files/exclude-rules` → `linters.exclusions`
- `run.timeout` → removed (passed via CLI `--timeout` flag)
- Add `errcheck` to enabled linters

```yaml
version: "2"

linters:
  default: none
  enable:
    - bodyclose
    - dogsled
    - errcheck
    - exhaustive
    - gochecknoinits
    - gofmt
    - goimports
    - goprintffuncname
    - gosec
    - gosimple
    - govet
    - ineffassign
    - misspell
    - nakedret
    - noctx
    - nolintlint
    - staticcheck
    - typecheck
    - unconvert
    - unparam
    - unused
    - whitespace

  settings:
    misspell:
      locale: US
    nolintlint:
      allow-leading-space: true
      allow-unused: false
      require-explanation: false
      require-specific: false

  exclusions:
    generated: lax
    paths:
      - vendor
      - \.git
      - ".*\\.pb\\.go$"
    rules:
      - path: _test\.go
        linters:
          - gocyclo
          - errcheck
          - dupl
          - gosec
      - text: "weak cryptographic primitive"
        linters:
          - gosec
      - path: _test\.go
        linters:
          - shadow
    max-issues-per-linter: 0
    max-same-issues: 0
```

**Step 2: Verify the config parses correctly**

Run: `cd /media/eddy/hdd/Project/Database_MCP_Server && golangci-lint config verify 2>&1 || true`
Expected: No config parse errors. Lint warnings about code are OK.

**Step 3: Commit**

```bash
git add .golangci.yml
git commit -m "ci: migrate golangci-lint config to v2 format

Migrate from v1 to v2 config syntax as required by golangci-lint v2.8.0.
Add errcheck linter. Remove run.timeout (passed via CLI flag)."
```

---

### Task 2: Fix security job — pin gosec, replace Nancy with govulncheck, add permissions, remove SSH

**Files:**
- Modify: `.github/workflows/ci.yml:92-129` (security job)

**Step 1: Rewrite the security job**

Replace lines 92-129 with:

```yaml
  # Job 3: Security Scanning
  security:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      security-events: write
    env:
      GOMODCACHE: ${{ github.workspace }}/.gomodcache
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version-file: go.mod

      - name: Run Gosec Security Scanner
        uses: securego/gosec@v2.22.4
        with:
          args: '-fmt sarif -out results.sarif ./...'

      - name: Upload SARIF file
        uses: github/codeql-action/upload-sarif@v3
        with:
          sarif_file: results.sarif

      - name: Run govulncheck
        uses: golang/govulncheck-action@v1
        with:
          go-version-file: go.mod
```

Changes:
- Pin gosec to `v2.22.4` (not `@master`)
- Add `permissions: security-events: write` at job level
- Replace Nancy + SSH block with `govulncheck`

**Step 2: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: pin gosec version, replace Nancy with govulncheck

Pin securego/gosec to v2.22.4 instead of @master.
Replace legacy Nancy dependency scanner with official govulncheck.
Add security-events:write permission for SARIF upload.
Remove unnecessary SSH setup for private modules."
```

---

### Task 3: Eliminate duplicate test run — make coverage job reuse build-and-test artifacts

**Files:**
- Modify: `.github/workflows/ci.yml:42-58` (build-and-test coverage steps)
- Modify: `.github/workflows/ci.yml:190-217` (coverage job)

**Step 1: Update build-and-test to upload coverage.out artifact**

In the build-and-test job, add `-covermode=atomic` to the test command (line 43) and upload `coverage.out` as an artifact:

Replace line 43:
```yaml
        run: go test -v -race -coverprofile=coverage.out -covermode=atomic -json ./... > report.json
```

Add a new step to upload coverage.out (after the existing coverage report upload, line 55-58):
```yaml
      - name: Upload coverage data
        uses: actions/upload-artifact@v4
        with:
          name: coverage-data
          path: coverage.out
```

**Step 2: Replace the coverage job to download artifact instead of re-running tests**

Replace lines 190-217 with:

```yaml
  # Job 5: Code Coverage Upload to Codecov
  coverage:
    runs-on: ubuntu-latest
    needs: build-and-test
    steps:
      - name: Download coverage data
        uses: actions/download-artifact@v4
        with:
          name: coverage-data

      - name: Upload coverage to Codecov
        uses: codecov/codecov-action@v4
        with:
          files: ./coverage.out
          flags: unittests
          name: codecov-umbrella
          fail_ci_if_error: false
          verbose: true
        env:
          CODECOV_TOKEN: ${{ secrets.CODECOV_TOKEN }}
```

**Step 3: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: eliminate duplicate test run in coverage job

Coverage job now downloads coverage.out from build-and-test instead
of re-running the entire test suite. Saves CI minutes."
```

---

### Task 4: Add `go mod tidy` verification and remove redundant gofmt check

**Files:**
- Modify: `.github/workflows/ci.yml:60-90` (lint job)

**Step 1: Update the lint job**

Replace lines 60-90 with:

```yaml
  # Job 2: Lint and Format Check
  lint:
    runs-on: ubuntu-latest
    env:
      GOMODCACHE: ${{ github.workspace }}/.gomodcache
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          cache: true

      - name: Run go vet
        run: go vet ./...

      - name: Verify go.mod is tidy
        run: |
          go mod tidy
          git diff --exit-code go.mod go.sum

      - name: Run golangci-lint
        uses: golangci/golangci-lint-action@v9
        with:
          version: v2.8.0
          args: --timeout=5m
```

Changes:
- Remove standalone gofmt check (already covered by golangci-lint `gofmt` linter)
- Add `go mod tidy` verification step

**Step 2: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: add go mod tidy check, remove redundant gofmt step

go mod tidy verification ensures module files stay clean.
Standalone gofmt check removed since golangci-lint gofmt linter covers it."
```

---

### Task 5: Remove single-entry matrix from build-and-test

**Files:**
- Modify: `.github/workflows/ci.yml:15-31` (build-and-test setup)

**Step 1: Replace matrix strategy with direct go-version-file**

Replace lines 15-31 with:

```yaml
  build-and-test:
    runs-on: ubuntu-latest
    env:
      GOMODCACHE: ${{ github.workspace }}/.gomodcache

    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          cache: true
```

**Step 2: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: use go-version-file instead of single-entry matrix

Replace matrix with go-version: ['1.25'] (single entry adds complexity
without benefit) with go-version-file: go.mod for consistency."
```

---

### Task 6: Update Makefile to align with CI changes

**Files:**
- Modify: `Makefile:44-47` (audit target)

**Step 1: Replace nancy audit target with govulncheck**

Replace lines 44-47:

```makefile
# Check dependencies for vulnerabilities
audit:
	@echo "Checking for vulnerabilities..."
	govulncheck ./...
```

**Step 2: Commit**

```bash
git add Makefile
git commit -m "ci: replace nancy with govulncheck in Makefile audit target"
```

---

### Task 7: Verify the full CI config is valid YAML and internally consistent

**Step 1: Validate YAML syntax**

Run: `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/ci.yml'))" && echo "YAML OK"`
Expected: `YAML OK`

**Step 2: Verify golangci-lint config**

Run: `golangci-lint config verify 2>&1 || true`
Expected: No config parse errors.

**Step 3: Run local lint to confirm no config regression**

Run: `golangci-lint run --timeout=5m 2>&1 | head -30`
Expected: Either clean output or real lint warnings (not config errors).

**Step 4: Squash or keep commits as-is depending on preference**

All changes are on the `fix/jsonschema-items` branch. Each commit is atomic and can be reviewed independently.
