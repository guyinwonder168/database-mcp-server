# MCP Tool Usage Examples

This document provides example workflows for using the Database MCP Provider tools programmatically.

## 1. List All Databases

**Request:**
```json
{
  "profile_name": "local-mariadb"
}
```
**Tool:** `list-databases`

---

## 2. List All Tables in a Database

**Request:**
```json
{
  "profile_name": "local-mariadb",
  "database_name": "sampledb"
}
```
**Tool:** `list-tables`

---

## 3. Describe a Table

**Request:**
```json
{
  "profile_name": "local-mariadb",
  "table_name": "example_table"
}
```
**Tool:** `describe-table`

---

## 4. Execute a SQL Query

**Request:**
```json
{
  "profile_name": "local-mariadb",
  "sql": "SELECT * FROM sampledb.example_table WHERE user_id=34 AND DATE(event_time)='2025-06-08';"
}
```
**Tool:** `execute-sql`

---

## 5. Workflow: Discover Schema and Query Attendance

1. Call `list-databases` to find available databases.
2. Call `list-tables` with the target database to list tables.
3. Call `describe-table` for the attendance table to get columns and types.
4. Call `execute-sql` with a constructed query using the discovered schema.

---

These examples enable LLMs and clients to discover schema and construct valid queries automatically.