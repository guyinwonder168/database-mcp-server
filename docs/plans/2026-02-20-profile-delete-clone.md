# Profile Delete & Clone Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add `delete` and `clone` actions to the existing `configure-profile` MCP tool, keeping full backward compatibility with the current create/update behavior.

**Architecture:** Add an optional `action` field (string, `"delete"` | `"clone"` | empty) and an optional `source_profile` field (string) to `ConfigureProfileParams`. The handler dispatches on `action`: empty → existing upsert path; `"delete"` → remove profile from config + save; `"clone"` → copy source profile fields into a new profile name with optional overrides. Validation is action-aware — each action has its own required fields.

**Tech Stack:** Go 1.25.7, `internal/mcp` package, `internal/config` package, `go test` with `cgo` build tag.

**Context-safe note:** Both new fields (`Action`, `SourceProfile`) are scalar `string` types with `omitempty`. The `inputSchemaFor[T]()` reflector + `sanitizeSchemaForGemini()` pipeline handles these cleanly — no `[]T` arrays, no `["null","array"]` type issues. This is Gemini-safe by design.

---

## Files Overview

| File | Role |
|------|------|
| `internal/mcp/server.go` | `ConfigureProfileParams` struct, `handleConfigureProfile`, `validateConfigureProfileParams`, `normalizeConfigureProfileParams`, `upsertConfigProfile`, tool description, constants |
| `internal/mcp/tool_help.go` | Help entry for `configure-profile` |
| `internal/mcp/server_test.go` | Tests for `handleConfigureProfile` |
| `internal/mcp/server_analyze_schema_helpers_test.go` | `TestValidateConfigureProfileParamsErrorBranch` |

## Action Dispatch Design

```
action=""       → existing create/update (upsert) — UNCHANGED
action="delete" → remove profile by name, save config
action="clone"  → copy source_profile → profile_name, apply overrides, save config
```

### Validation Rules per Action

| Action | Required | Optional | Ignored |
|--------|----------|----------|---------|
| `""` (create/update) | `profile_name`, `db_type`, `database_name` | all others | `source_profile` |
| `"delete"` | `profile_name` | — | all others |
| `"clone"` | `profile_name`, `source_profile` | `db_type`, `host`, `port`, `username`, `password`, `database_name`, `readonly`, `sslmode` | — |

### Clone Override Behavior

When `action="clone"`, the handler:
1. Finds the `source_profile` in config
2. Copies all fields from source into a new `config.Profile`
3. Sets the new profile's `ProfileName` to the requested `profile_name`
4. Applies overrides: any non-zero field in the request replaces the source value
5. Zero-value fields (`""` for strings, `0` for int, `false` for bool) are kept from source — this means you cannot override `readonly` to `false` or `port` to `0` via clone (acceptable limitation)

### Error Cases

| Scenario | Error Code | Message |
|----------|-----------|---------|
| `action="delete"`, `profile_name` empty | `MISSING_PARAMETER` | "profile_name is required for delete" |
| `action="delete"`, profile not found | `PROFILE_NOT_FOUND` | "Profile 'X' not found" |
| `action="clone"`, `profile_name` empty | `MISSING_PARAMETER` | "profile_name is required for clone" |
| `action="clone"`, `source_profile` empty | `MISSING_PARAMETER` | "source_profile is required for clone" |
| `action="clone"`, source not found | `PROFILE_NOT_FOUND` | "Source profile 'X' not found" |
| `action="clone"`, target already exists | `INVALID_INPUT` | "Profile 'X' already exists" |
| `action` is unknown value | `INVALID_INPUT` | "Unknown action 'X'" |

---

## Task 1: Add `Action` field + validate unknown action

### Files
- Modify: `internal/mcp/server.go:1067-1077` (ConfigureProfileParams struct)
- Modify: `internal/mcp/server.go:2154-2176` (validateConfigureProfileParams)
- Test: `internal/mcp/server_test.go`

### Step 1: Write the failing test

Add to `internal/mcp/server_test.go`:

```go
func TestHandleConfigureProfile_InvalidAction(t *testing.T) {
	testConfig := setupTestConfig(t)
	defer os.Remove(testConfig)

	server := NewMCPServerWithConfig(testConfig)
	ctx := context.Background()

	res, _, err := server.handleConfigureProfile(ctx, nil, ConfigureProfileParams{
		Action:      "invalid",
		ProfileName: "test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || res.Content == nil {
		t.Fatal("expected error result, got nil")
	}
	text := res.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "INVALID_INPUT") {
		t.Fatalf("expected INVALID_INPUT error, got: %s", text)
	}
}
```

### Step 2: Run test to verify it fails

```bash
source ~/.gvm/scripts/gvm && go test ./internal/mcp -run TestHandleConfigureProfile_InvalidAction -count=1
```
Expected: FAIL — `action` field doesn't exist yet on `ConfigureProfileParams`, compile error.

### Step 3: Write minimal implementation

In `internal/mcp/server.go`, add `Action` and `SourceProfile` to the struct:

```go
type ConfigureProfileParams struct {
	Action       string `json:"action,omitempty"`
	ProfileName  string `json:"profile_name"`
	DBType       string `json:"db_type,omitempty"`
	Host         string `json:"host,omitempty"`
	Port         int    `json:"port,omitempty"`
	Username     string `json:"username,omitempty"`
	Password     string `json:"password,omitempty"`
	DatabaseName string `json:"database_name,omitempty"`
	Readonly     bool   `json:"readonly"`
	SSLMode      string `json:"sslmode,omitempty"`
	SourceProfile string `json:"source_profile,omitempty"`
}
```

**Important changes to existing fields:**
- `db_type` tag changes from `json:"db_type"` → `json:"db_type,omitempty"` (no longer always required)
- `database_name` tag changes from `json:"database_name"` → `json:"database_name,omitempty"` (no longer always required)

Update `validateConfigureProfileParams` to handle action dispatch:

```go
func validateConfigureProfileParams(p ConfigureProfileParams) *mcp.CallToolResult {
	switch p.Action {
	case "":
		// Existing create/update validation — unchanged
		if p.ProfileName != "" && p.DBType != "" && p.DatabaseName != "" {
			return nil
		}
		structErr := NewStructuredError(
			ErrorCodeMissingParameter,
			messageMissingRequiredParameters,
			"All of profile_name, db_type, and database_name are required",
		).WithSuggestions(
			ErrorSuggestion{
				Action:      actionProvideAllRequiredParameters,
				Description: "Ensure profile_name, db_type, and database_name are included",
				Example:     `{"profile_name": "mydb", "db_type": "mysql", "database_name": "mydb"}`,
			},
		)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: structErr.ToJSON()}},
		}

	case "delete":
		if p.ProfileName == "" {
			structErr := NewStructuredError(
				ErrorCodeMissingParameter,
				messageMissingRequiredParameters,
				"profile_name is required for delete",
			).WithSuggestions(ErrorSuggestion{
				Action:  actionProvideAllRequiredParameters,
				Example: `{"action": "delete", "profile_name": "mydb"}`,
			})
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: structErr.ToJSON()}},
			}
		}
		return nil

	case "clone":
		if p.ProfileName == "" || p.SourceProfile == "" {
			structErr := NewStructuredError(
				ErrorCodeMissingParameter,
				messageMissingRequiredParameters,
				"profile_name and source_profile are required for clone",
			).WithSuggestions(ErrorSuggestion{
				Action:  actionProvideAllRequiredParameters,
				Example: `{"action": "clone", "profile_name": "new-profile", "source_profile": "existing-profile"}`,
			})
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: structErr.ToJSON()}},
			}
		}
		return nil

	default:
		structErr := NewStructuredError(
			ErrorCodeInvalidInput,
			"Unknown action",
			fmt.Sprintf("Unknown action '%s'; valid actions are: delete, clone (or omit for create/update)", p.Action),
		)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: structErr.ToJSON()}},
		}
	}
}
```

### Step 4: Run test to verify it passes

```bash
source ~/.gvm/scripts/gvm && go test ./internal/mcp -run TestHandleConfigureProfile_InvalidAction -count=1
```
Expected: PASS

### Step 5: Run full test suite to verify backward compatibility

```bash
source ~/.gvm/scripts/gvm && go test ./internal/mcp -run TestHandleConfigureProfile -count=1
source ~/.gvm/scripts/gvm && go test ./internal/mcp -run TestValidateConfigureProfileParamsErrorBranch -count=1
```
Expected: All PASS — existing tests use empty `Action` → hits `case ""` → same validation as before.

### Step 6: Commit

```bash
git add internal/mcp/server.go internal/mcp/server_test.go
git commit -m "feat: add action field to ConfigureProfileParams with validation dispatch"
```

---

## Task 2: Delete profile — happy path

### Files
- Modify: `internal/mcp/server.go:2107-2141` (handleConfigureProfile)
- Test: `internal/mcp/server_test.go`

### Step 1: Write the failing test

```go
func TestHandleConfigureProfile_Delete(t *testing.T) {
	testConfig := setupTestConfig(t)
	defer os.Remove(testConfig)

	server := NewMCPServerWithConfig(testConfig)
	ctx := context.Background()

	// First, create a profile to delete
	res, _, err := server.handleConfigureProfile(ctx, nil, ConfigureProfileParams{
		ProfileName:  "todelete",
		DBType:       "sqlite",
		DatabaseName: testSQLiteDBPath,
	})
	if err != nil {
		t.Fatalf("create error: %v", err)
	}
	text := res.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "successfully") {
		t.Fatalf("expected success, got: %s", text)
	}

	// Delete it
	res, _, err = server.handleConfigureProfile(ctx, nil, ConfigureProfileParams{
		Action:      "delete",
		ProfileName: "todelete",
	})
	if err != nil {
		t.Fatalf("delete error: %v", err)
	}
	text = res.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "deleted") {
		t.Fatalf("expected deleted confirmation, got: %s", text)
	}

	// Verify it's gone — findProfile should fail
	_, _, err = server.findProfile("todelete")
	if err == nil {
		t.Fatal("expected profile to be gone after delete")
	}
}
```

### Step 2: Run test to verify it fails

```bash
source ~/.gvm/scripts/gvm && go test ./internal/mcp -run TestHandleConfigureProfile_Delete -count=1
```
Expected: FAIL — handler still calls upsert for all actions.

### Step 3: Write minimal implementation

Update `handleConfigureProfile` to dispatch on `Action`:

```go
func (s *MCPServer) handleConfigureProfile(ctx context.Context, _ *mcp.CallToolRequest, input ConfigureProfileParams) (*mcp.CallToolResult, any, error) {
	cfg, err := config.LoadConfig(s.ConfigPath)
	log.JSONLog("debug", "Loaded config for configure-profile", map[string]interface{}{"configPath": s.ConfigPath, "error": err})
	if err != nil {
		log.JSONLog("warn", "Failed to load config, creating new config", map[string]interface{}{"error": err.Error()})
		cfg = &config.Config{}
	}
	p := normalizeConfigureProfileParams(input)
	if errResult := validateConfigureProfileParams(p); errResult != nil {
		return errResult, nil, nil
	}

	switch p.Action {
	case "delete":
		return s.handleDeleteProfile(cfg, p)
	case "clone":
		return s.handleCloneProfile(cfg, p)
	default:
		return s.handleUpsertProfile(cfg, p)
	}
}
```

Extract the existing upsert logic into `handleUpsertProfile`:

```go
func (s *MCPServer) handleUpsertProfile(cfg *config.Config, p ConfigureProfileParams) (*mcp.CallToolResult, any, error) {
	ensureConfigAESKey(cfg, s.ConfigPath)
	upsertConfigProfile(cfg, p)
	if err := config.SaveConfig(s.ConfigPath, cfg); err != nil {
		structErr := s.errorAnalyzer.AnalyzeError(err, map[string]interface{}{
			"profile_name": p.ProfileName,
			"operation":    "save_config",
		})
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: structErr.ToJSON()}},
		}, nil, nil
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: "Profile configured successfully."}},
	}, nil, nil
}
```

Add `handleDeleteProfile`:

```go
func (s *MCPServer) handleDeleteProfile(cfg *config.Config, p ConfigureProfileParams) (*mcp.CallToolResult, any, error) {
	found := false
	for i := range cfg.Profiles {
		if cfg.Profiles[i].ProfileName == p.ProfileName {
			cfg.Profiles = append(cfg.Profiles[:i], cfg.Profiles[i+1:]...)
			found = true
			break
		}
	}
	if !found {
		structErr := NewStructuredError(
			ErrorCodeProfileNotFound,
			messageProfileNotFound,
			fmt.Sprintf(messageProfileNotFoundFormat, p.ProfileName),
		).WithContext("profile_name", p.ProfileName)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: structErr.ToJSON()}},
		}, nil, nil
	}
	if err := config.SaveConfig(s.ConfigPath, cfg); err != nil {
		structErr := s.errorAnalyzer.AnalyzeError(err, map[string]interface{}{
			"profile_name": p.ProfileName,
			"operation":    "save_config",
		})
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: structErr.ToJSON()}},
		}, nil, nil
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{
			Text: fmt.Sprintf("Profile '%s' deleted successfully.", p.ProfileName),
		}},
	}, nil, nil
}
```

### Step 4: Run test to verify it passes

```bash
source ~/.gvm/scripts/gvm && go test ./internal/mcp -run TestHandleConfigureProfile_Delete -count=1
```
Expected: PASS

### Step 5: Commit

```bash
git add internal/mcp/server.go internal/mcp/server_test.go
git commit -m "feat: implement delete action for configure-profile"
```

---

## Task 3: Delete profile — error case (profile not found)

### Files
- Test: `internal/mcp/server_test.go`

### Step 1: Write the failing test

```go
func TestHandleConfigureProfile_DeleteNotFound(t *testing.T) {
	testConfig := setupTestConfig(t)
	defer os.Remove(testConfig)

	server := NewMCPServerWithConfig(testConfig)
	ctx := context.Background()

	res, _, err := server.handleConfigureProfile(ctx, nil, ConfigureProfileParams{
		Action:      "delete",
		ProfileName: "nonexistent",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := res.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "PROFILE_NOT_FOUND") {
		t.Fatalf("expected PROFILE_NOT_FOUND error, got: %s", text)
	}
}
```

### Step 2: Run test to verify it passes (already implemented in Task 2)

```bash
source ~/.gvm/scripts/gvm && go test ./internal/mcp -run TestHandleConfigureProfile_DeleteNotFound -count=1
```
Expected: PASS — the not-found check was implemented in Task 2's `handleDeleteProfile`.

### Step 3: Commit (if test was written separately)

```bash
git add internal/mcp/server_test.go
git commit -m "test: add delete-not-found error case for configure-profile"
```

---

## Task 4: Delete profile — validation (missing profile_name)

### Files
- Test: `internal/mcp/server_test.go`

### Step 1: Write the failing test

```go
func TestHandleConfigureProfile_DeleteMissingName(t *testing.T) {
	testConfig := setupTestConfig(t)
	defer os.Remove(testConfig)

	server := NewMCPServerWithConfig(testConfig)
	ctx := context.Background()

	res, _, err := server.handleConfigureProfile(ctx, nil, ConfigureProfileParams{
		Action: "delete",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := res.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "MISSING_PARAMETER") {
		t.Fatalf("expected MISSING_PARAMETER error, got: %s", text)
	}
}
```

### Step 2: Run test to verify it passes (already implemented in Task 1)

```bash
source ~/.gvm/scripts/gvm && go test ./internal/mcp -run TestHandleConfigureProfile_DeleteMissingName -count=1
```
Expected: PASS — validation for `case "delete"` was implemented in Task 1.

### Step 3: Commit

```bash
git add internal/mcp/server_test.go
git commit -m "test: add delete-missing-name validation test for configure-profile"
```

---

## Task 5: Clone profile — happy path

### Files
- Modify: `internal/mcp/server.go` (add `handleCloneProfile`)
- Test: `internal/mcp/server_test.go`

### Step 1: Write the failing test

```go
func TestHandleConfigureProfile_Clone(t *testing.T) {
	testConfig := setupTestConfig(t)
	defer os.Remove(testConfig)

	server := NewMCPServerWithConfig(testConfig)
	ctx := context.Background()

	// Clone the existing "testsqlite" profile to "cloned-sqlite"
	res, _, err := server.handleConfigureProfile(ctx, nil, ConfigureProfileParams{
		Action:        "clone",
		ProfileName:   "cloned-sqlite",
		SourceProfile: "testsqlite",
	})
	if err != nil {
		t.Fatalf("clone error: %v", err)
	}
	text := res.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "cloned") {
		t.Fatalf("expected cloned confirmation, got: %s", text)
	}

	// Verify the cloned profile exists and has the same fields
	cfg, err := config.LoadConfig(testConfig)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	var cloned *config.Profile
	for i := range cfg.Profiles {
		if cfg.Profiles[i].ProfileName == "cloned-sqlite" {
			cloned = &cfg.Profiles[i]
			break
		}
	}
	if cloned == nil {
		t.Fatal("cloned profile not found in config")
	}
	if cloned.DBType != "sqlite" {
		t.Fatalf("expected db_type 'sqlite', got '%s'", cloned.DBType)
	}
	if cloned.DatabaseName != testSQLiteDBPath {
		t.Fatalf("expected database_name '%s', got '%s'", testSQLiteDBPath, cloned.DatabaseName)
	}
}
```

### Step 2: Run test to verify it fails

```bash
source ~/.gvm/scripts/gvm && go test ./internal/mcp -run TestHandleConfigureProfile_Clone -count=1
```
Expected: FAIL — `handleCloneProfile` is not yet implemented (handler dispatches to it but it doesn't exist).

### Step 3: Write minimal implementation

Add `handleCloneProfile` to `internal/mcp/server.go`:

```go
func (s *MCPServer) handleCloneProfile(cfg *config.Config, p ConfigureProfileParams) (*mcp.CallToolResult, any, error) {
	// Find source profile
	var source *config.Profile
	for i := range cfg.Profiles {
		if cfg.Profiles[i].ProfileName == p.SourceProfile {
			source = &cfg.Profiles[i]
			break
		}
	}
	if source == nil {
		structErr := NewStructuredError(
			ErrorCodeProfileNotFound,
			messageProfileNotFound,
			fmt.Sprintf("Source profile '%s' not found", p.SourceProfile),
		).WithContext("source_profile", p.SourceProfile)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: structErr.ToJSON()}},
		}, nil, nil
	}

	// Check target doesn't already exist
	for i := range cfg.Profiles {
		if cfg.Profiles[i].ProfileName == p.ProfileName {
			structErr := NewStructuredError(
				ErrorCodeInvalidInput,
				"Profile already exists",
				fmt.Sprintf("Profile '%s' already exists; use a different name or delete it first", p.ProfileName),
			).WithContext("profile_name", p.ProfileName)
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: structErr.ToJSON()}},
			}, nil, nil
		}
	}

	// Copy source and apply overrides
	cloned := *source
	cloned.ProfileName = p.ProfileName
	if p.DBType != "" {
		cloned.DBType = p.DBType
	}
	if p.Host != "" {
		cloned.Host = p.Host
	}
	if p.Port != 0 {
		cloned.Port = p.Port
	}
	if p.Username != "" {
		cloned.Username = p.Username
	}
	if p.Password != "" {
		cloned.Password = p.Password
	}
	if p.DatabaseName != "" {
		cloned.DatabaseName = p.DatabaseName
	}
	if p.Readonly {
		cloned.Readonly = p.Readonly
	}
	if p.SSLMode != "" {
		cloned.SSLMode = p.SSLMode
	}

	ensureConfigAESKey(cfg, s.ConfigPath)
	cfg.Profiles = append(cfg.Profiles, cloned)

	if err := config.SaveConfig(s.ConfigPath, cfg); err != nil {
		structErr := s.errorAnalyzer.AnalyzeError(err, map[string]interface{}{
			"profile_name": p.ProfileName,
			"operation":    "save_config",
		})
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: structErr.ToJSON()}},
		}, nil, nil
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{
			Text: fmt.Sprintf("Profile '%s' cloned from '%s' successfully.", p.ProfileName, p.SourceProfile),
		}},
	}, nil, nil
}
```

### Step 4: Run test to verify it passes

```bash
source ~/.gvm/scripts/gvm && go test ./internal/mcp -run TestHandleConfigureProfile_Clone -count=1
```
Expected: PASS

### Step 5: Commit

```bash
git add internal/mcp/server.go internal/mcp/server_test.go
git commit -m "feat: implement clone action for configure-profile"
```

---

## Task 6: Clone profile — with overrides

### Files
- Test: `internal/mcp/server_test.go`

### Step 1: Write the failing test

```go
func TestHandleConfigureProfile_CloneWithOverrides(t *testing.T) {
	testConfig := setupTestConfig(t)
	defer os.Remove(testConfig)

	server := NewMCPServerWithConfig(testConfig)
	ctx := context.Background()

	// Clone "testsqlite" but override readonly to true
	res, _, err := server.handleConfigureProfile(ctx, nil, ConfigureProfileParams{
		Action:        "clone",
		ProfileName:   "readonly-clone",
		SourceProfile: "testsqlite",
		Readonly:      true,
	})
	if err != nil {
		t.Fatalf("clone error: %v", err)
	}
	text := res.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "cloned") {
		t.Fatalf("expected cloned confirmation, got: %s", text)
	}

	cfg, err := config.LoadConfig(testConfig)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	for i := range cfg.Profiles {
		if cfg.Profiles[i].ProfileName == "readonly-clone" {
			if !cfg.Profiles[i].Readonly {
				t.Fatal("expected readonly=true override, got false")
			}
			if cfg.Profiles[i].DBType != "sqlite" {
				t.Fatalf("expected db_type 'sqlite' from source, got '%s'", cfg.Profiles[i].DBType)
			}
			return
		}
	}
	t.Fatal("cloned profile not found")
}
```

### Step 2: Run test to verify it passes (already implemented in Task 5)

```bash
source ~/.gvm/scripts/gvm && go test ./internal/mcp -run TestHandleConfigureProfile_CloneWithOverrides -count=1
```
Expected: PASS — override logic is already in `handleCloneProfile`.

### Step 3: Commit

```bash
git add internal/mcp/server_test.go
git commit -m "test: add clone-with-overrides test for configure-profile"
```

---

## Task 7: Clone profile — error cases

### Files
- Test: `internal/mcp/server_test.go`

### Step 1: Write the failing tests

```go
func TestHandleConfigureProfile_CloneErrors(t *testing.T) {
	testConfig := setupTestConfig(t)
	defer os.Remove(testConfig)

	server := NewMCPServerWithConfig(testConfig)
	ctx := context.Background()

	t.Run("missing source_profile", func(t *testing.T) {
		res, _, err := server.handleConfigureProfile(ctx, nil, ConfigureProfileParams{
			Action:      "clone",
			ProfileName: "newname",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		text := res.Content[0].(*mcp.TextContent).Text
		if !strings.Contains(text, "MISSING_PARAMETER") {
			t.Fatalf("expected MISSING_PARAMETER, got: %s", text)
		}
	})

	t.Run("source not found", func(t *testing.T) {
		res, _, err := server.handleConfigureProfile(ctx, nil, ConfigureProfileParams{
			Action:        "clone",
			ProfileName:   "newname",
			SourceProfile: "nonexistent",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		text := res.Content[0].(*mcp.TextContent).Text
		if !strings.Contains(text, "PROFILE_NOT_FOUND") {
			t.Fatalf("expected PROFILE_NOT_FOUND, got: %s", text)
		}
	})

	t.Run("target already exists", func(t *testing.T) {
		res, _, err := server.handleConfigureProfile(ctx, nil, ConfigureProfileParams{
			Action:        "clone",
			ProfileName:   "testsqlite",
			SourceProfile: "testpg",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		text := res.Content[0].(*mcp.TextContent).Text
		if !strings.Contains(text, "INVALID_INPUT") {
			t.Fatalf("expected INVALID_INPUT (already exists), got: %s", text)
		}
	})
}
```

### Step 2: Run tests to verify they pass (already implemented in Tasks 1+5)

```bash
source ~/.gvm/scripts/gvm && go test ./internal/mcp -run TestHandleConfigureProfile_CloneErrors -count=1
```
Expected: PASS

### Step 3: Commit

```bash
git add internal/mcp/server_test.go
git commit -m "test: add clone error case tests for configure-profile"
```

---

## Task 8: Update tool description and help

### Files
- Modify: `internal/mcp/server.go:504-521` (tool registration description)
- Modify: `internal/mcp/tool_help.go:44-48` (help entry)

### Step 1: No failing test needed — this is documentation/metadata, not behavior

### Step 2: Update the tool description

In `internal/mcp/server.go`, update the tool description around line 508:

```go
Description: descriptionFormatter(`Create, update, delete, or clone a database connection profile. Required for all database actions.
  Fields:
    profile_name (required)
    db_type (mysql|mariadb|postgres|sqlite)
    host / port / username / password (required except sqlite)
    database_name
    readonly (boolean)
    sslmode (Postgres only, optional: disable|require|verify-ca|verify-full; defaults to require)
    action (optional: "delete" or "clone"; omit for create/update)
    source_profile (required for clone: name of profile to copy from)
  Examples:
  Create/update: {"profile_name":"mydb","db_type":"postgres","host":"localhost","port":5432,"username":"app","password":"secret","database_name":"appdb","readonly":false}
  Delete: {"action":"delete","profile_name":"mydb"}
  Clone: {"action":"clone","profile_name":"mydb-readonly","source_profile":"mydb","readonly":true}`),
```

### Step 3: Update tool help

In `internal/mcp/tool_help.go`, update the `configure-profile` entry:

```go
"configure-profile": {
	Summary:         "Create, update, delete, or clone a database profile used by all DB tools.",
	MinimalExample:  map[string]any{"profile_name": "analytics_db", "db_type": "sqlite", "database_name": "/tmp/demo.sqlite", "readonly": true},
	AdvancedExample: map[string]any{"action": "clone", "profile_name": "analytics_ro", "source_profile": "analytics_db", "readonly": true},
	CommonErrors: []ToolHelpError{
		{Error: "Profile not found", Cause: "Using unknown profile_name", Fix: "Create profile first with configure-profile"},
		{Error: "Profile not found (delete)", Cause: "Deleting non-existent profile", Fix: "Check profile name with list-profiles"},
		{Error: "Source profile not found (clone)", Cause: "source_profile does not exist", Fix: "Verify source profile name with list-profiles"},
	},
},
```

### Step 4: Run full test suite

```bash
source ~/.gvm/scripts/gvm && go test ./internal/mcp -count=1
```
Expected: All PASS

### Step 5: Commit

```bash
git add internal/mcp/server.go internal/mcp/tool_help.go
git commit -m "docs: update configure-profile description and help for delete/clone actions"
```

---

## Task 9: Full regression + vet + fmt

### Step 1: Run full validation (sequentially per AGENTS.md Mistake #5)

```bash
source ~/.gvm/scripts/gvm && go test ./internal/mcp -count=1
```

```bash
source ~/.gvm/scripts/gvm && go vet ./...
```

```bash
source ~/.gvm/scripts/gvm && gofmt -l .
```

Expected: All clean, no failures.

### Step 2: Fix any issues found

If `gofmt` lists files, run `gofmt -w .` and re-commit.

### Step 3: Final commit if needed

```bash
git add -A
git commit -m "chore: fmt/vet cleanup"
```

---

## Verification Checklist

- [ ] `action=""` (omitted) → existing create/update works identically (backward compatible)
- [ ] `action="delete"` + valid `profile_name` → profile removed from config.yaml
- [ ] `action="delete"` + missing `profile_name` → `MISSING_PARAMETER` error
- [ ] `action="delete"` + non-existent profile → `PROFILE_NOT_FOUND` error
- [ ] `action="clone"` + valid `profile_name` + valid `source_profile` → new profile created with source fields
- [ ] `action="clone"` + overrides → override fields applied on top of source
- [ ] `action="clone"` + missing `source_profile` → `MISSING_PARAMETER` error
- [ ] `action="clone"` + non-existent source → `PROFILE_NOT_FOUND` error
- [ ] `action="clone"` + target already exists → `INVALID_INPUT` error
- [ ] `action="unknown"` → `INVALID_INPUT` error
- [ ] Schema passes `sanitizeSchemaForGemini()` — no `["null","array"]` issues (scalar fields only)
- [ ] `go test ./internal/mcp -count=1` → all pass
- [ ] `go vet ./...` → clean
- [ ] `gofmt -l .` → no output
- [ ] Tool description updated
- [ ] Tool help updated

## Known Limitations

1. **Clone cannot set `readonly` to `false`** — Go zero-value for `bool` is `false`, so there's no way to distinguish "user wants false" from "user didn't specify". Cloning always inherits the source value for `false`-valued bools and `0`-valued ints. This is acceptable for the initial implementation.

2. **No connection pool cleanup on delete** — The current architecture creates connections per-request via `OpenConnectionWithPool()` with no server-level connection cache. There is nothing to clean up. If a connection pool cache is added later, delete should be updated to close cached connections for the deleted profile.
