# Database MCP Server - Technical Specifications

## Overview

This document provides detailed technical specifications for the Database MCP Server implementation, including architecture, data models, algorithms, and technical design decisions. Toolchain: Go 1.25.5.

**Transports**
- Stdio (default)
- HTTP/SSE (optional) via `MCP_SSE_ADDR` (e.g., `:8080`; start server with `MCP_SSE_ADDR=":8080" ./mcp-server`)
  - Claude Desktop: add an MCP provider pointing to `http://localhost:8080` (SSE auto-detected).
  - Codex CLI: if SSE supported, target the HTTP endpoint; otherwise use stdio (default).
  - Kilocode: stdio by default; leave `MCP_SSE_ADDR` unset unless testing SSE.

## System Architecture

### High-Level Architecture

```
┌─────────────────────────────────────┐
│         MCP Client (AI Agent)       │
└─────────────────┬───────────────────┘
                  │ stdio
┌─────────────────┴───────────────────┐
│          MCP Server Layer           │
│    (internal/mcp/server.go)         │
├─────────────────────────────────────┤
│         Business Logic              │
│  ┌─────────────┬─────────────────┐  │
│  │   Config    │   Database      │  │
│  │  Manager    │   Connector     │  │
│  └─────────────┴─────────────────┘  │
├─────────────────────────────────────┤
│         Infrastructure              │
│  ┌──────┬──────┬────────────────┐   │
│  │ Log  │ AES  │ DB Drivers     │   │
│  └──────┴──────┴────────────────┘   │
└─────────────────────────────────────┘
```

### Component Architecture

#### 1. MCP Server Layer (`internal/mcp/`)
- **server.go**: Main MCP server implementation
- **analyze_schema_types.go**: Type system for schema analysis
- **errors.go**: Error handling and response formatting
- **server_test.go**: Unit tests for MCP server

#### 2. Configuration Management (`internal/config/`)
- **config.go**: Profile and configuration management
- **config_template.yaml**: Configuration template

#### 3. Database Abstraction (`internal/db/`)
- **driver.go**: Database connection management and abstraction

#### 4. Logging Infrastructure (`internal/log/`)
- **logger.go**: Structured JSON logging with rotation

## Data Models

### Configuration Data Models

#### Profile Structure
```go
type Profile struct {
    ProfileName string `yaml:"profile_name"`
    DBType     string `yaml:"db_type"`     // mysql, postgresql, sqlite
    Host       string `yaml:"host"`
    Port       int    `yaml:"port"`
    Username   string `yaml:"username"`
    Password   string `yaml:"password"`     // encrypted
    DatabaseName string `yaml:"database_name"`
    ReadOnly   bool   `yaml:"readonly"`
}
```

#### Configuration Structure
```go
type Config struct {
    MaxPoolSize int       `yaml:"max_pool_size"`
    AESKey      string    `yaml:"aes_key"`      // 32-character key
    Profiles    []Profile `yaml:"profiles"`
}
```

### MCP Action Data Models

#### Standard Request Structure
```go
type MCPRequest struct {
    Method string                 `json:"method"`
    Params map[string]interface{} `json:"params"`
    ID     interface{}           `json:"id"`
}
```

#### Standard Response Structure
```go
type MCPResponse struct {
    Result interface{} `json:"result,omitempty"`
    Error  *MCPError   `json:"error,omitempty"`
    ID     interface{} `json:"id"`
}
```

#### Error Structure
```go
type MCPError struct {
    Code        string                 `json:"code"`
    Message     string                 `json:"message"`
    Details     map[string]interface{} `json:"details,omitempty"`
    Suggestions []string               `json:"suggestions,omitempty"`
}
```

### Schema Analysis Data Models

#### Column Information
```go
type ColumnInfo struct {
    Name         string `json:"name"`
    Type         string `json:"type"`
    Nullable     bool   `json:"nullable"`
    Key         string `json:"key"`
    Default      *string `json:"default,omitempty"`
    Extra        string `json:"extra,omitempty"`
    Comment      string `json:"comment,omitempty"`
    MaxLength    *int   `json:"max_length,omitempty"`
    Precision    *int   `json:"precision,omitempty"`
    Scale        *int   `json:"scale,omitempty"`
    Charset      string `json:"charset,omitempty"`
    Collation    string `json:"collation,omitempty"`
}
```

#### Table Information
```go
type TableInfo struct {
    Name        string       `json:"name"`
    Type        string       `json:"type"`        // TABLE, VIEW
    Columns     []ColumnInfo `json:"columns"`
    RowCount    *int64      `json:"row_count,omitempty"`
    Size        *int64       `json:"size,omitempty"`
    Comment     string       `json:"comment,omitempty"`
}
```

#### Schema Analysis Result
```go
type SchemaAnalysisResult struct {
    Level              string                    `json:"level"`              // BASIC, DETAILED, COMPREHENSIVE
    DatabaseName       string                    `json:"database_name"`
    Tables            []TableInfo               `json:"tables"`
    BusinessContext    *BusinessContext          `json:"business_context,omitempty"`
    DataQuality       *DataQualityMetrics       `json:"data_quality,omitempty"`
    Relationships     []RelationshipInfo        `json:"relationships,omitempty"`
    QuerySuggestions  []QuerySuggestion         `json:"query_suggestions,omitempty"`
}
```

## Algorithm Specifications

### Connection Pooling Algorithm

#### Pool Configuration
```go
func OpenConnectionWithPool(profile Profile) (*sql.DB, error) {
    db, err := sql.Open(driverName, dsn)
    if err != nil {
        return nil, err
    }
    
    // Configure pool based on config
    db.SetMaxOpenConns(config.MaxPoolSize)
    db.SetMaxIdleConns(config.MaxPoolSize / 2)
    db.SetConnMaxLifetime(time.Hour)
    db.SetConnMaxIdleTime(time.Minute * 30)
    
    return db, nil
}
```

#### Connection Lifecycle
1. **Connection Request**: Check pool for available connection
2. **Pool Exhaustion**: Create new connection if under max limit
3. **Connection Validation**: Ping before returning to client
4. **Connection Return**: Return to pool after operation completion
5. **Idle Cleanup**: Close connections idle for >30 minutes
6. **Max Lifetime**: Close connections older than 1 hour

### Encryption Algorithm

#### AES-256-GCM Implementation
```go
func EncryptPassword(plaintext, key string) (string, error) {
    block, err := aes.NewCipher([]byte(key))
    if err != nil {
        return "", err
    }
    
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return "", err
    }
    
    nonce := make([]byte, gcm.NonceSize())
    if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
        return "", err
    }
    
    ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
    return base64.StdEncoding.EncodeToString(ciphertext), nil
}
```

#### Key Management
- **Key Generation**: Cryptographically secure random 32-byte key
- **Key Storage**: Environment variable `DB_MCP_AES_KEY` or config file
- **Key Rotation**: Manual process requiring re-encryption of all passwords

### Schema Analysis Algorithm

#### Multi-Level Analysis Process

##### BASIC Level
1. **Table Discovery**: Query information_schema.tables
2. **Column Enumeration**: Query information_schema.columns
3. **Basic Metadata**: Extract name, type, nullability
4. **Result Format**: Simple table/column structure

##### DETAILED Level
1. **BASIC Analysis**: Include all BASIC level information
2. **Extended Metadata**: Add constraints, defaults, comments
3. **Data Statistics**: Row counts, table sizes
4. **Index Information**: Primary and foreign key information
5. **Result Format**: Enhanced metadata with statistics

##### COMPREHENSIVE Level
1. **DETAILED Analysis**: Include all DETAILED level information
2. **Business Context Inference**: Analyze naming patterns and relationships
3. **Data Quality Assessment**: Check for null ratios, data patterns
4. **Relationship Discovery**: Map foreign key relationships
5. **Query Optimization**: Suggest indexes and query improvements
6. **Result Format**: Complete analysis with actionable insights

#### Business Context Inference Algorithm
```go
func InferBusinessContext(tables []TableInfo) *BusinessContext {
    ctx := &BusinessContext{
        Domain:        inferDomain(tables),
        EntityTypes:   inferEntityTypes(tables),
        Relationships: inferRelationships(tables),
        DataPatterns:  inferDataPatterns(tables),
    }
    return ctx
}

func inferDomain(tables []TableInfo) string {
    // Analyze table names, column names, and patterns
    // Common domains: ecommerce, crm, analytics, etc.
}
```

### SQL Parsing Algorithm

#### Read-Only Enforcement
```go
func EnforceReadOnly(sql string) error {
    // Parse SQL using simplified parser
    tokens := tokenizeSQL(sql)
    
    for _, token := range tokens {
        if isWriteOperation(token) {
            return &MCPError{
                Code:    "READONLY_VIOLATION",
                Message: "Write operation not allowed in read-only profile",
            }
        }
    }
    return nil
}

func isWriteOperation(token string) bool {
    writeOps := []string{"INSERT", "UPDATE", "DELETE", "DROP", "CREATE", "ALTER"}
    upperToken := strings.ToUpper(token)
    
    for _, op := range writeOps {
        if upperToken == op {
            return true
        }
    }
    return false
}
```

## Database-Specific Implementations

### MySQL/MariaDB Implementation

#### Connection String Format
```
{username}:{password}@tcp({host}:{port})/{database_name}?parseTime=true&loc=Local
```

#### Schema Queries
```sql
-- List tables
SELECT table_name, table_type, table_comment 
FROM information_schema.tables 
WHERE table_schema = ? 
ORDER BY table_name

-- Describe table
SELECT 
    column_name, 
    data_type, 
    is_nullable, 
    column_key, 
    column_default, 
    extra, 
    column_comment,
    character_maximum_length,
    numeric_precision,
    numeric_scale,
    character_set_name,
    collation_name
FROM information_schema.columns 
WHERE table_schema = ? AND table_name = ?
ORDER BY ordinal_position
```

### PostgreSQL Implementation

#### Connection String Format
```
host={host} port={port} user={username} password={password} dbname={database_name} sslmode=disable
```

#### Schema Queries
```sql
-- List tables
SELECT 
    t.table_name, 
    CASE WHEN t.table_type = 'BASE TABLE' THEN 'TABLE' ELSE 'VIEW' END as table_type,
    obj_description(c.oid) as table_comment
FROM information_schema.tables t
LEFT JOIN pg_class c ON c.relname = t.table_name
WHERE t.table_schema = 'public'
ORDER BY t.table_name

-- Describe table
SELECT 
    column_name, 
    data_type, 
    is_nullable, 
    column_default, 
    '' as column_key,
    col_description(pgc.oid, cols.ordinal_position) as column_comment,
    character_maximum_length,
    numeric_precision,
    numeric_scale
FROM information_schema.columns cols
LEFT JOIN pg_class pgc ON pgc.relname = cols.table_name
WHERE cols.table_schema = 'public' AND cols.table_name = ?
ORDER BY cols.ordinal_position
```

### SQLite Implementation

#### Connection String Format
```
file:{database_path}?cache=shared&mode=rwc
```

#### Schema Queries
```sql
-- List tables
SELECT name, type FROM sqlite_master 
WHERE type IN ('table', 'view') AND name NOT LIKE 'sqlite_%'
ORDER BY name

-- Describe table
PRAGMA table_info({table_name})
```

## Performance Optimizations

### Connection Pool Optimization

#### Dynamic Pool Sizing
```go
func OptimizePoolSize(db *sql.DB, workload WorkloadType) {
    switch workload {
    case HighConcurrency:
        db.SetMaxOpenConns(50)
        db.SetMaxIdleConns(25)
    case MemoryConstrained:
        db.SetMaxOpenConns(10)
        db.SetMaxIdleConns(5)
    case Default:
        db.SetMaxOpenConns(25)
        db.SetMaxIdleConns(12)
    }
}
```

#### Query Optimization
- **Prepared Statements**: Use prepared statements for repeated queries
- **Result Streaming**: Stream large result sets to reduce memory usage
- **Connection Reuse**: Maximize connection reuse through efficient pooling

### Memory Management

#### Result Set Handling
```go
func QueryWithLimit(db *sql.DB, query string, limit int) (*sql.Rows, error) {
    // Add LIMIT clause if not present
    if !strings.Contains(strings.ToUpper(query), "LIMIT") {
        query = fmt.Sprintf("%s LIMIT %d", query, limit)
    }
    
    return db.Query(query)
}
```

#### Garbage Collection Optimization
- **Object Pooling**: Reuse objects for frequent allocations
- **Buffer Management**: Use sync.Pool for byte buffers
- **Finalizer Avoidance**: Avoid finalizers for better GC performance

## Security Implementation

### Credential Protection

#### Encryption Workflow
1. **Key Generation**: Generate cryptographically secure 32-byte key
2. **Password Encryption**: AES-256-GCM with random nonce
3. **Storage**: Base64-encoded ciphertext in YAML
4. **Decryption**: On-demand decryption for database connections
5. **Memory Cleanup**: Zero plaintext from memory after use

#### Access Control
```go
type SecurityContext struct {
    ProfileName string
    ReadOnly   bool
    AllowedIPs []string
    RateLimit  int
}

func ValidateAccess(ctx SecurityContext, request MCPRequest) error {
    // Check read-only constraints
    if ctx.ReadOnly && isWriteOperation(request) {
        return ErrReadOnlyViolation
    }
    
    // Check rate limits
    if !checkRateLimit(ctx.ProfileName, ctx.RateLimit) {
        return ErrRateLimitExceeded
    }
    
    return nil
}
```

### SQL Injection Prevention

#### Parameterized Queries
```go
func ExecuteQuery(db *sql.DB, query string, params []interface{}) (*sql.Rows, error) {
    // Always use prepared statements
    stmt, err := db.Prepare(query)
    if err != nil {
        return nil, err
    }
    defer stmt.Close()
    
    return stmt.Query(params...)
}
```

#### Query Validation
```go
func ValidateSQL(query string) error {
    // Check for dangerous patterns
    dangerousPatterns := []string{
        "DROP", "DELETE", "UPDATE", "INSERT", 
        "ALTER", "CREATE", "TRUNCATE",
    }
    
    upperQuery := strings.ToUpper(query)
    for _, pattern := range dangerousPatterns {
        if strings.Contains(upperQuery, pattern) {
            return fmt.Errorf("potentially dangerous SQL pattern detected: %s", pattern)
        }
    }
    
    return nil
}
```

## Error Handling Strategy

### Error Classification

#### System Errors
- **CONNECTION_FAILED**: Database connection failures
- **TIMEOUT**: Query execution timeouts
- **RESOURCE_EXHAUSTED**: Memory or connection pool exhaustion

#### User Errors
- **INVALID_PARAMETER**: Invalid request parameters
- **PERMISSION_DENIED**: Insufficient database permissions
- **SQL_SYNTAX_ERROR**: SQL syntax errors

#### Business Logic Errors
- **PROFILE_NOT_FOUND**: Requested profile doesn't exist
- **TABLE_NOT_FOUND**: Requested table doesn't exist
- **READONLY_VIOLATION**: Write operation on read-only profile

### Error Response Format
```go
type ErrorResponse struct {
    Status      string                 `json:"status"`
    ErrorCode   string                 `json:"error_code"`
    Message     string                 `json:"message"`
    Details     map[string]interface{} `json:"details,omitempty"`
    Suggestions []string               `json:"suggestions,omitempty"`
    Context     ErrorContext           `json:"context,omitempty"`
}
```

## Monitoring and Observability

### Logging Strategy

#### Log Levels
- **ERROR**: System errors and failures
- **WARN**: Warning conditions and recoverable errors
- **INFO**: Important operational events
- **DEBUG**: Detailed debugging information

#### Log Format
```json
{
  "timestamp": "2024-12-09T15:09:42Z",
  "level": "INFO",
  "component": "mcp-server",
  "action": "execute-sql",
  "profile_name": "production_db",
  "duration_ms": 150,
  "rows_affected": 42,
  "error": null
}
```

### Metrics Collection

#### Performance Metrics
- **Query Execution Time**: Histogram of query durations
- **Connection Pool Usage**: Active/idle connection counts
- **Error Rates**: Error counts by type and frequency
- **Memory Usage**: Heap and GC statistics

#### Business Metrics
- **API Usage**: Request counts by endpoint
- **User Activity**: Active profiles and usage patterns
- **Database Access**: Most accessed tables and queries

## Deployment Architecture

### Container Deployment

#### Docker Configuration
```dockerfile
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o mcp-server ./cmd/server/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/mcp-server .
COPY --from=builder /app/config.yaml .
CMD ["./mcp-server"]
```

#### Kubernetes Deployment
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: database-mcp-server
spec:
  replicas: 3
  selector:
    matchLabels:
      app: database-mcp-server
  template:
    metadata:
      labels:
        app: database-mcp-server
    spec:
      containers:
      - name: mcp-server
        image: database-mcp-server:latest
        env:
        - name: DB_MCP_AES_KEY
          valueFrom:
            secretKeyRef:
              name: mcp-secrets
              key: aes-key
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"
```

### Configuration Management

#### Environment-Specific Configs
- **Development**: Local databases, verbose logging
- **Staging**: Production-like environment, reduced logging
- **Production**: Optimized settings, security hardening

#### Secret Management
- **Encryption Keys**: Stored in Kubernetes secrets or AWS KMS
- **Database Credentials**: Encrypted at rest, rotated regularly
- **Access Control**: RBAC for secret access

## Testing Strategy

### Unit Testing

#### Test Coverage Requirements
- **Minimum Coverage**: 90% line coverage
- **Critical Path Coverage**: 100% for security-critical components
- **Error Path Testing**: All error conditions must be tested

#### Test Structure
```go
func TestExecuteSQL_Success(t *testing.T) {
    // Setup
    db := setupTestDB(t)
    defer db.Close()
    
    // Test
    result, err := ExecuteSQL(db, "SELECT 1 as test", []interface{}{})
    
    // Assert
    assert.NoError(t, err)
    assert.NotNil(t, result)
}
```

### Integration Testing

#### Database-Specific Tests
- **MySQL Integration**: Test with MySQL 5.7, 8.0
- **PostgreSQL Integration**: Test with PostgreSQL 10-16
- **SQLite Integration**: Test with SQLite 3.8.0+
- **Cross-Database Tests**: Verify consistent behavior

#### MCP Protocol Tests
- **Protocol Compliance**: Verify MCP specification compliance
- **Error Handling**: Test error response formats
- **Tool Discovery**: Verify tool listing functionality

### Performance Testing

#### Load Testing
- **Concurrent Users**: Test with 100+ concurrent connections
- **Query Performance**: Benchmark query execution times
- **Memory Usage**: Monitor memory consumption under load
- **Connection Pool**: Test pool efficiency under stress

#### Stress Testing
- **Resource Exhaustion**: Test behavior under resource constraints
- **Error Recovery**: Test recovery from database failures
- **Long-Running Tests**: Test stability over extended periods

## Version Compatibility

### Database Version Support

#### MySQL/MariaDB
- **MySQL**: 5.7+, 8.0+
- **MariaDB**: 10.2+
- **Features**: Full feature support across versions
- **Limitations**: Some newer features may not be available in older versions

#### PostgreSQL
- **Supported Versions**: 10, 11, 12, 13, 14, 15, 16
- **Feature Compatibility**: Core features available across all versions
- **Version-Specific Features**: Leverage version-specific optimizations

#### SQLite
- **Minimum Version**: 3.8.0+
- **Recommended Version**: 3.40.0+
- **Feature Support**: Full feature support with latest versions

### Go Version Compatibility

#### Minimum Requirements
- **Go Version**: 1.23.0+
- **Build Tools**: Standard Go toolchain
- **Dependencies**: Compatible with Go modules

#### Compatibility Testing
- **Multiple Go Versions**: Tested on Go 1.25.5 (current baseline)
- **Platform Testing**: Linux, macOS, Windows
- **Architecture Testing**: amd64, arm64

### Enhancement Architecture Extensions

#### New Component Requirements
- **SQL Parser Library**: For query validation and optimization
- **Statistical Analysis**: For business intelligence features
- **Graph Processing**: For data lineage analysis
- **NLP Integration**: For enhanced natural language processing
- **Advanced Encryption**: For context and session data protection

#### Performance Considerations
- **Memory Usage**: Additional components will increase memory footprint
- **CPU Usage**: Analytics and optimization features require more processing
- **Storage**: Schema lineage and change tracking require metadata storage

## Enhancement Architecture Evolution

### Phase 1 Architecture Changes
- **Query Optimization Engine**: New `internal/mcp/query_optimizer.go` component
- **Validation Framework**: Extended `internal/mcp/query_validator.go` with syntax checking
- **Context Management**: New `internal/mcp/context/` package for session state
- **NLP Enhancement**: Extended `internal/mcp/nlp/` for intent classification

### Phase 2 Architecture Changes
- **Data Lineage Engine**: New `internal/mcp/lineage/` package for dependency analysis
- **Analytics Framework**: New `internal/mcp/analytics/` package for statistical processing
- **Business Intelligence**: KPI discovery and trend analysis capabilities

### Phase 3 Architecture Changes
- **Schema Evolution**: New `internal/mcp/schema/` package for change tracking
- **Advanced Profiling**: Enhanced `internal/mcp/profiling/` for statistical analysis
- **Federation Engine**: New `internal/mcp/federation/` package for cross-database operations

### Backward Compatibility
- **Existing Tools**: All 14 current MCP tools remain unchanged
- **Configuration**: Extended YAML structure with feature flags
- **API Protocol**: No changes to MCP JSON-RPC specification
- **Database Drivers**: Existing driver abstraction supports all enhancements

## Conclusion

This technical specification provides a comprehensive overview of the Database MCP Server implementation and its planned evolution. The current architecture is production-ready and well-designed, with clear extension paths for AI-enhanced capabilities while maintaining backward compatibility.

The implementation follows best practices for Go development, database connectivity, and MCP protocol compliance. The modular design allows for easy extension and maintenance while maintaining high performance and reliability.

The enhancement roadmap maintains architectural integrity while adding sophisticated AI capabilities that will transform the Database MCP Server into an intelligent database interaction platform.
