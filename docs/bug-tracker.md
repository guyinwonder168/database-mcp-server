# Bug Tracker — MCP Database Server

Documented bugs, issues, and improvement requests discovered during usage.

---

## BUG-001: `list-tables` — SQL Scan Argument Mismatch

| Field | Details |
|-------|---------|
| **Tool** | `list-tables` |
| **Severity** | High — tool completely unusable |
| **Status** | Fixed (2026-04-16) |
| **Date Reported** | 2026-04-16 |
| **Reproducibility** | 100% (MariaDB, profile `css-mariadb`) |

### Error

```
sql: expected 2 destination arguments in Scan, not 3
```

### Context

Called with:
```json
{
  "profile_name": "css-mariadb",
  "database_name": "css"
}
```

### Root Cause (Confirmed)

The `queryTableNames()` function called `tableListQuery("mysql")` which returned `SHOW FULL TABLES` (2 columns), but then used `scanTableInfo(rows, "mysql")` which expected 3 columns (schema, name, type). Column count mismatch between query and scanner.

### Fix Applied

Created `tableInfoListQuery()` returning 3-column aligned queries per dbType:
- mysql/mariadb: `SELECT TABLE_SCHEMA, TABLE_NAME, TABLE_TYPE FROM information_schema.TABLES WHERE TABLE_SCHEMA = DATABASE() ORDER BY TABLE_NAME`
- postgres: `SELECT table_schema, table_name, table_type FROM information_schema.tables WHERE ...`
- sqlite: `SELECT '' AS schema, name, type FROM sqlite_master WHERE type IN ('table', 'view') ORDER BY name`

Simplified `scanTableInfo()` to unified 3-col signature (removed dbType branching). Updated `queryTableNames` to call `tableInfoListQuery` instead of `tableListQuery`.

**Note**: SQLite `list-tables` now includes views (was `type='table'`, now `type IN ('table', 'view')`), making it consistent with MySQL/PostgreSQL behavior.

---

## BUG-002: `analyze-schema` — Same SQL Scan Mismatch

| Field | Details |
|-------|---------|
| **Tool** | `analyze-schema` |
| **Severity** | High — tool completely unusable |
| **Status** | Fixed (2026-04-16) |
| **Date Reported** | 2026-04-16 |
| **Reproducibility** | 100% (MariaDB, profile `css-mariadb`) |

### Error

```
sql: expected 2 destination arguments in Scan, not 3
```

### Context

Called with:
```json
{
  "profile_name": "css-mariadb",
  "database_name": "css",
  "analysis_level": "basic"
}
```

### Root Cause (Confirmed)

Same underlying issue as BUG-001 — `listAnalyzeSchemaTables` called `queryTableNames` → `tableListQuery` → `scanTableInfo` with column count mismatch on mysql/mariadb. Fixed by the same `tableInfoListQuery` + unified `scanTableInfo` change.

---

## BUG-003: `get-tool-help` — Returns Minimal/Unusable Help Content

| Field | Details |
|-------|---------|
| **Tool** | `get-tool-help` |
| **Severity** | Medium — poor developer experience, blocks proper usage |
| **Status** | Fixed (2026-04-16) |
| **Date Reported** | 2026-04-16 |
| **Reproducibility** | 100% |

### Problem

The `get-tool-help` endpoint did not return meaningful or detailed parameter documentation. It only returned:

1. **Tool name and summary** — already known from tool listing
2. **Topics list** — array of available topic strings (e.g., `["summary", "minimal_example", "advanced_example", "errors", "all"]`)
3. **Minimal example** — a single trivial call

**What was missing:**
- Full parameter schema (types, required/optional, allowed values, defaults)
- `analysis_level` valid enum values (e.g., `"basic"` vs `"detailed"` vs `"comprehensive"`)
- Descriptions for each parameter
- Advanced examples with explanations
- Error codes and troubleshooting guidance
- Return value schema / response format documentation

### Fix Applied

Added `ToolParamInfo` struct and new fields (`Description`, `Parameters`, `ResponseFormat`) to both `toolHelpEntry` and `GetToolHelpResult`. Populated all 18 catalog entries with:
- Description: 1-2 sentence description of each tool
- Parameters: Full parameter info with name, type, required, description, enum values, defaults
- AdvancedExample: More complex usage examples
- CommonErrors: 2-3 common errors with cause and fix
- ResponseFormat: Brief description of response shape

Updated `topicFilteredToolHelpResult` to include Description and Parameters in all topic responses.

---

## Fix Priority Recommendation

| Bug | Priority | Effort | Status |
|-----|----------|--------|--------|
| BUG-001 (`list-tables` scan error) | P0 | Small | Fixed |
| BUG-002 (`analyze-schema` scan error) | P0 | Small | Fixed (same fix as BUG-001) |
| BUG-003 (`get-tool-help` unhelpful) | P1 | Medium | Fixed |

**Fix applied (2026-04-16):** BUG-001 and BUG-002 were resolved by unifying `scanTableInfo` to a 3-column signature and creating `tableInfoListQuery()` with column-aligned queries per dbType. BUG-003 was resolved by adding `ToolParamInfo` struct, `Description`/`Parameters`/`ResponseFormat` fields, and populating all 18 tool help entries.