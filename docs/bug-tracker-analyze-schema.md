# Bug Tracker — `analyze-schema` Accuracy Issues

> **Parent**: [bug-tracker.md](./bug-tracker.md)
> **Discovered**: 2026-04-16
> **Test Database**: CSS (MariaDB) — 114 tables, 904 columns, 96 FK relationships

Bugs discovered during comprehensive `analyze-schema` run against the CSS database. The tool produces output but with significant accuracy problems that make automated analysis unreliable.

---

## BUG-004: `analyze-schema` — Column Count Massively Underreported

| Field | Details |
|-------|---------|
| **Tool** | `analyze-schema` |
| **Severity** | High — fundamental metadata inaccuracy |
| **Status** | Open |
| **Date Reported** | 2026-04-16 |
| **Reproducibility** | 100% (MariaDB, profile `css-mariadb`) |

### Problem

Reports **362 columns** when the actual count is **904 columns** (60% underreporting).

### Evidence

```
Expected: 904 columns (verified via SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA='css')
Reported: 362 columns (analyze-schema output: "total_columns": 362)
```

### Root Cause

The tool relies on `describe-table` for column enumeration. Tables with 0 sample rows (54 of 114 tables) show **0 columns** in the output:

```
activation_link_mapping: 0 columns  (actual: multiple)
adm_account_role_groups: 0 columns  (actual: multiple)
broadcast_customer_room_links: 0 columns  (actual: multiple)
...
```

The `describe-table` step appears to skip or fail silently on empty tables, and `analyze-schema` does not fall back to `information_schema.COLUMNS`.

### Expected Behavior

Column count should match `SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = ?` regardless of whether tables contain data.

### Suggested Fix

1. **Primary**: Query `information_schema.COLUMNS` directly for column counts (independent of sample data)
2. **Fallback**: If `describe-table` returns 0 columns, cross-check against `information_schema.COLUMNS`
3. **Validation**: Add assertion: if `list-tables` returns N tables but column total is suspiciously low, log a warning

---

## BUG-005: `analyze-schema` — Row Counts All Zero

| Field | Details |
|-------|---------|
| **Tool** | `analyze-schema` |
| **Severity** | High — table size analysis completely broken |
| **Status** | Open |
| **Date Reported** | 2026-04-16 |
| **Reproducibility** | 100% (MariaDB, profile `css-mariadb`) |

### Problem

All 114 tables report **0 rows** in `table_schemas`, even though many tables have data (60 tables returned sample data, largest has 310K rows).

### Evidence

```
table_schemas output:
  user_id_hashes: 0 rows   (actual: 310,200)
  adm_audit_log: 0 rows    (actual: 8,532)
  customers: 0 rows        (actual: 158)
  user_ids: 0 rows         (actual: 176)
```

### Root Cause

The tool populates `row_count` from sample data retrieval only. It does NOT query:
- `information_schema.TABLES.TABLE_ROWS` (fast, estimated)
- `SELECT COUNT(*) FROM table` (accurate, slower)

The `row_count` field in `table_schemas` is never populated from metadata.

### Expected Behavior

Row counts should be populated from `information_schema.TABLES.TABLE_ROWS` at minimum, with optional `SELECT COUNT(*)` for accurate counts at higher analysis levels.

### Suggested Fix

1. **Basic analysis**: Query `SELECT TABLE_NAME, TABLE_ROWS FROM information_schema.TABLES WHERE TABLE_SCHEMA = ?`
2. **Detailed/Comprehensive**: Run `SELECT COUNT(*)` for accurate counts (with timeout protection)
3. **Profiled tables**: Use `sample_row_count` from column profiling as a secondary indicator

---

## BUG-006: `analyze-schema` — Foreign Key Relationships Not Detected

| Field | Details |
|-------|---------|
| **Tool** | `analyze-schema` (via `discover-joins`) |
| **Severity** | High — relationship analysis produces false positives, zero true positives |
| **Status** | Open |
| **Date Reported** | 2026-04-16 |
| **Reproducibility** | 100% (MariaDB, profile `css-mariadb`) |

### Problem

The tool detected **2,336 relationships** — all classified as `shared_column` with 0.6 confidence. **Zero** actual foreign key relationships were detected, despite 96 real FKs existing in the database.

### Evidence

```
Detected:  2,336 relationships, ALL "shared_column" type, ALL 0.6 confidence
Actual FKs: 96 foreign keys (verified via information_schema.KEY_COLUMN_USAGE)

Example false positive:
  customers JOIN user_id_hashes ON customers.created = user_id_hashes.created
  (WRONG — should be ON user_id_hashes.i_customer = customers.id)
```

The tool suggests joining tables on `created` timestamps — a column present in nearly every table — producing thousands of meaningless relationships.

### Root Cause

The `discover-joins` / relationship detection logic uses column-name matching to infer relationships. It does NOT query actual FK metadata from:
- `information_schema.KEY_COLUMN_USAGE` (FK definitions)
- `information_schema.REFERENTIAL_CONSTRAINTS` (constraint details)

Column-name matching on common names like `created`, `i_realm`, `id` produces massive false positives.

### Expected Behavior

1. **Primary**: Query `information_schema.KEY_COLUMN_USAGE` for real FK relationships (100% accurate)
2. **Secondary**: Use column-name matching only as "suggested" relationships with lower confidence
3. **Separate**: Clearly distinguish between "foreign key" (verified) and "shared column" (inferred)

### Suggested Fix

1. Add FK discovery via `information_schema`:
   ```sql
   SELECT 
     TABLE_NAME, COLUMN_NAME,
     REFERENCED_TABLE_NAME, REFERENCED_COLUMN_NAME
   FROM information_schema.KEY_COLUMN_USAGE
   WHERE TABLE_SCHEMA = ? AND REFERENCED_TABLE_NAME IS NOT NULL
   ```
2. Label FK relationships as `foreign_key` with 1.0 confidence
3. Label column-name matches as `shared_column` with 0.3 confidence (not 0.6)
4. **Never** suggest joins on `created`/`modified` timestamp columns
5. Deprioritize joins on generic columns (`id`, `i_realm`) unless FK-verified

---

## BUG-007: `analyze-schema` — Domain Misclassification

| Field | Details |
|-------|---------|
| **Tool** | `analyze-schema` |
| **Severity** | Medium — business context analysis misleading |
| **Status** | Open |
| **Date Reported** | 2026-04-16 |
| **Reproducibility** | 100% (MariaDB, profile `css-mariadb`) |

### Problem

Classifies the CSS database as **"CRM"** with confidence 3/5. The actual domain is a **SIP/VoIP communication platform** (multi-tenant PBX with messaging, push notifications, device provisioning).

### Evidence

```
Detected:  "crm" (confidence: 3.00)
Actual:    SIP/VoIP multi-tenant communication platform

Strong domain indicators present but missed:
  - sipuser, sip_password, sip_use_anonymous_registration
  - call_destinations, call_rates, call_recordings
  - pushtokens, push_url
  - broadcast_channels, broadcast_messages
  - turnservers, verto_secret
  - xmpp (cross-database references)
  - device provisioning (autoconfig, bundle_id_autoconfig_mapping)
```

### Root Cause

The domain classifier uses table name pattern matching. It correctly identifies `snake_case` naming but doesn't have SIP/VoIP domain patterns in its dictionary.

### Suggested Fix

1. Add SIP/VoIP domain patterns: `sip*`, `call_*`, `push*`, `broadcast*`, `turn*`, `xmpp*`, `autoconfig*`, `voip*`
2. Add communication platform indicators: `devices`, `realms`, `feature_groups`, `whitelist*`
3. Weight table count by domain: 6 call tables + 9 broadcast tables + 7 device tables should strongly indicate VoIP, not CRM

---

## BUG-008: `analyze-schema` — Entity Classification Too Generic

| Field | Details |
|-------|---------|
| **Tool** | `analyze-schema` |
| **Severity** | Medium — table catalog not useful |
| **Status** | Open |
| **Date Reported** | 2026-04-16 |
| **Reproducibility** | 100% |

### Problem

Table catalog classifies **86 of 114 tables (75%)** as `"other"`, making the categorization useless:

```
core_entities: 106 tables
lookup_tables:  2 tables
audit_tables:   6 tables

Entity roles within core_entities:
  "other":         86 tables (75%)
  "master_data":  20 tables (18%)
  "log":           6 tables (5%)
  "lookup":        2 tables (2%)
```

### Expected Behavior

Tables should be classified into meaningful categories:
- `customer_management`: customers, user_ids, devices, contacts
- `telephony`: call_destinations, call_rates, call_recordings, e164
- `messaging`: broadcast_channels, broadcast_messages, metadata_group
- `administration`: adm_accounts, adm_role, dashboard_users
- `provisioning`: autoconfig, bundle_id_autoconfig_mapping, turnservers
- `security`: 2fa_checks, brute_force_lock, auth_tokens, preauthkeys

### Suggested Fix

1. Expand entity role taxonomy beyond 4 categories
2. Use table name prefixes (`adm_*`, `broadcast_*`, `address_book_*`, `realm_*`) for classification
3. Use FK relationships (once BUG-006 is fixed) to infer entity roles from connectivity

---

## BUG-009: `analyze-schema` — `performance_optimization` Returns Empty

| Field | Details |
|-------|---------|
| **Tool** | `analyze-schema` |
| **Severity** | Low — advertised feature produces no output |
| **Status** | Open |
| **Date Reported** | 2026-04-16 |
| **Reproducibility** | 100% |

### Problem

The `performance_optimization` section returns `{"query_patterns": {}}` — completely empty despite running at `comprehensive` level.

### Expected Behavior

Should include:
- Missing index recommendations (tables with no indexes on FK columns)
- Composite index suggestions (multi-column WHERE clauses)
- Table scan warnings (large tables without indexes)
- Query pattern analysis from stored procedures (if accessible)

### Suggested Fix

1. Query `information_schema.STATISTICS` for existing indexes
2. Cross-reference with `information_schema.KEY_COLUMN_USAGE` for FK columns lacking indexes
3. For `comprehensive` level, run `EXPLAIN` on sample queries for large tables
4. Check for tables > 10K rows without indexes on frequently-joined columns

---

## Summary

| Bug | Severity | Area | Impact |
|-----|----------|------|--------|
| BUG-004 | High | Column count | 60% underreporting (362 vs 904) |
| BUG-005 | High | Row counts | All zero — size analysis broken |
| BUG-006 | High | FK detection | 0 real FKs found, 2336 false positives |
| BUG-007 | Medium | Domain | Misclassifies VoIP as CRM |
| BUG-008 | Medium | Classification | 75% tables labeled "other" |
| BUG-009 | Low | Performance | Empty output |

### Fix Priority

| Priority | Bugs | Effort | Dependencies |
|----------|------|--------|-------------|
| **P0** | BUG-006 (FK detection) | Medium | None — enables BUG-008 |
| **P0** | BUG-004 (Column count) | Small | None |
| **P1** | BUG-005 (Row counts) | Small | None |
| **P1** | BUG-007 (Domain) | Small | None |
| **P2** | BUG-008 (Classification) | Medium | Depends on BUG-006 |
| **P3** | BUG-009 (Performance) | Medium | Depends on BUG-004, BUG-005 |

### Key Insight

All high-severity bugs share a common root cause: **the tool relies on procedural data retrieval (describe-table, sample-data) instead of metadata queries (information_schema)**. A unified metadata-first approach would fix BUG-004, BUG-005, and BUG-006 simultaneously.
