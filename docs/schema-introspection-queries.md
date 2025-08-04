# Enhanced Schema Introspection Queries

This document contains the database-specific queries needed for comprehensive schema introspection including column comments, defaults, and constraints.

## MySQL/MariaDB

### Current Query (Basic)
```sql
DESCRIBE `database`.`table`
```

### Enhanced Query (With Comments & Metadata)
```sql
SELECT 
    COLUMN_NAME as name,
    COLUMN_TYPE as type,
    IS_NULLABLE as nullable,
    COLUMN_KEY as key_type,
    COLUMN_DEFAULT as default_value,
    COLUMN_COMMENT as comment,
    EXTRA as extra,
    CHARACTER_SET_NAME as character_set,
    COLLATION_NAME as collation
FROM INFORMATION_SCHEMA.COLUMNS 
WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
ORDER BY ORDINAL_POSITION
```

## PostgreSQL

### Current Query (Basic)
```sql
SELECT column_name, data_type, is_nullable 
FROM information_schema.columns 
WHERE table_schema='public' AND table_name=?
```

### Enhanced Query (With Comments & Metadata)
```sql
SELECT 
    c.column_name as name,
    c.data_type as type,
    c.is_nullable as nullable,
    c.column_default as default_value,
    COALESCE(pgd.description, '') as comment,
    c.character_maximum_length,
    c.numeric_precision,
    c.numeric_scale,
    CASE 
        WHEN tc.constraint_type = 'PRIMARY KEY' THEN 'PRI'
        WHEN tc.constraint_type = 'UNIQUE' THEN 'UNI'
        WHEN tc.constraint_type = 'FOREIGN KEY' THEN 'MUL'
        ELSE ''
    END as key_type
FROM information_schema.columns c
LEFT JOIN pg_class pgc ON pgc.relname = c.table_name
LEFT JOIN pg_namespace pgn ON pgn.oid = pgc.relnamespace
LEFT JOIN pg_attribute pga ON pga.attrelid = pgc.oid AND pga.attname = c.column_name
LEFT JOIN pg_description pgd ON pgd.objoid = pgc.oid AND pgd.objsubid = pga.attnum
LEFT JOIN information_schema.key_column_usage kcu ON kcu.column_name = c.column_name 
    AND kcu.table_name = c.table_name AND kcu.table_schema = c.table_schema
LEFT JOIN information_schema.table_constraints tc ON tc.constraint_name = kcu.constraint_name
    AND tc.table_name = c.table_name AND tc.table_schema = c.table_schema
WHERE c.table_schema = ? AND c.table_name = ?
ORDER BY c.ordinal_position
```

## SQLite

### Current Query (Basic)
```sql
PRAGMA database.table_info('table')
```

### Enhanced Query (With Additional Metadata)
```sql
-- SQLite doesn't support column comments natively
-- We'll use table_xinfo for extended information
PRAGMA database.table_xinfo('table')
```

### SQLite Response Structure
```
cid|name|type|notnull|dflt_value|pk|hidden
```

## Expected Enhanced Response Structure

```json
{
  "columns": [
    {
      "name": "id",
      "type": "int(11)",
      "nullable": false,
      "key": "PRI",
      "default": null,
      "comment": "Primary key identifier",
      "extra": "auto_increment",
      "character_set": "utf8mb4",
      "collation": "utf8mb4_unicode_ci",
      "auto_increment": true,
      "max_length": null,
      "precision": null,
      "scale": null
    },
    {
      "name": "email",
      "type": "varchar(255)",
      "nullable": false,
      "key": "UNI",
      "default": null,
      "comment": "User email address",
      "extra": "",
      "character_set": "utf8mb4",
      "collation": "utf8mb4_unicode_ci",
      "auto_increment": false,
      "max_length": 255,
      "precision": null,
      "scale": null
    }
  ]
}