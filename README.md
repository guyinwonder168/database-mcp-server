# Database MCP Server

[![Go](https://img.shields.io/badge/Go-1.25.7%2B-00ADD8?logo=Go&logoColor=white)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](https://opensource.org/licenses/MIT)
[![Version](https://img.shields.io/badge/Version-v1.0.7-blue.svg)](https://github.com/guyinwonder168/database-mcp-server/releases/tag/v1.0.7)
[![Reliability Rating](https://sonarcloud.io/api/project_badges/measure?project=guyinwonder168_database-mcp-server&metric=reliability_rating)](https://sonarcloud.io/summary/new_code?id=guyinwonder168_database-mcp-server)
[![Security Rating](https://sonarcloud.io/api/project_badges/measure?project=guyinwonder168_database-mcp-server&metric=security_rating)](https://sonarcloud.io/summary/new_code?id=guyinwonder168_database-mcp-server)
[![Maintainability Rating](https://sonarcloud.io/api/project_badges/measure?project=guyinwonder168_database-mcp-server&metric=sqale_rating)](https://sonarcloud.io/summary/new_code?id=guyinwonder168_database-mcp-server)
[![Quality Gate Status](https://sonarcloud.io/api/project_badges/measure?project=guyinwonder168_database-mcp-server&metric=alert_status)](https://sonarcloud.io/summary/new_code?id=guyinwonder168_database-mcp-server)

A production-ready Model Context Protocol (MCP) provider for SQL databases, built using various vibe coding tools. Supports MySQL, MariaDB, PostgreSQL, and SQLite. Features robust connection pooling, secure AES-GCM credential storage, structured JSON logging, comprehensive schema introspection, and a full suite of 18 MCP tools. Built and tested with Go 1.25.7.

## 🚀 Quick Start

```bash
# Clone the repository
git clone https://github.com/guyinwonder168/database-mcp-server.git
cd database-mcp-server

# Build the server
go build -o mcp-server ./cmd/server/main.go

# Run the server
./mcp-server
```

## 📦 Container Package (GHCR)

```bash
# Pull the release image
docker pull ghcr.io/guyinwonder168/database-mcp-server:v1.0.7

# Run with stdio transport
docker run --rm -i ghcr.io/guyinwonder168/database-mcp-server:v1.0.7
```

```bash
# Persist config.yaml and logs on host
mkdir -p ./.mcp-data
docker run --rm -i \
  -v "$(pwd)/.mcp-data:/app" \
  ghcr.io/guyinwonder168/database-mcp-server:v1.0.7
```

Package registry: `https://github.com/guyinwonder168/database-mcp-server/pkgs/container/database-mcp-server`

## 📋 Features

- 🔧 **Interactive Setup** - Auto-creates `config.yaml` if missing; all configuration is managed via MCP actions
- 👥 **Profile Management** - Add, update, and list database profiles via MCP
- ⚡ **SQL Execution** - Run arbitrary SQL queries (with read-only enforcement)
- 🔍 **Schema Introspection** - List tables/views, describe table schemas, list databases, and discover joins
- 📊 **Sample Data Fetching** - Fetch sample rows to infer data formats and value ranges
- 🔗 **Automated Join Discovery** - Suggest JOIN SQL for building complex queries
- 🚦 **Query Optimization** - EXPLAIN-based analysis with findings and performance estimates
- 🛡️ **Query Validation** - Syntax, logic, and security checks before execution
- 🤖 **Smart Query Builder** - Generate SQL queries programmatically (integrated into analyze-schema AIQuerySuggestions)
- 🔒 **Read-only Profiles** - Prevent write operations on selected profiles
- 🔐 **Secure Credentials** - Passwords are encrypted at rest using AES-GCM (256-bit)
- 🏊 **Connection Pooling** - Efficient, configurable pooling with max pool size
- 📝 **Structured Logging & Error Handling** - All actions and errors are logged as structured JSON; actionable error responses
- 🛠️ **Tool Discovery** - `list-tools` MCP action returns a machine-readable list of all available tools/actions
- 🔌 **Official MCP Protocol** - Communication via stdio (not HTTP server; JSON is exchanged over stdio via official Go MCP SDK)
- 📈 **Business Intelligence** - Discover KPIs, trends, anomalies, and distribution patterns via `discover-insights`
- 🧭 **Data Lineage** - Analyze upstream/downstream dependencies via `analyze-data-lineage`
- 🧱 **Schema Evolution Tracking** - Track schema snapshots, detect drift, and generate migration scripts via `track-schema-changes`
- 🧬 **Advanced Data Profiling** - Optional statistical/pattern profiling for `analyze-schema` via `profiling: true`
- 🌐 **Multi-Database Federation** - Execute federated subqueries with cross-profile joins via `federated-query`

## 🛠️ Supported MCP Tools

| Tool | Description |
|-------|-------------|
| `configure-profile` | Create or update database connection profiles |
| `list-profiles` | List all configured database profiles |
| `execute-sql` | Execute arbitrary SQL queries with read-only enforcement |
| `list-tables` | List tables in selected database |
| `describe-table` | Describe comprehensive table schema with metadata |
| `list-databases` | List accessible databases for profile |
| `smart-query-builder` | Generate SQL from high-level intent |
| `optimize-query` | Run EXPLAIN, return plan, findings, and performance estimate |
| `validate-query` | Validate SQL syntax and flag risky patterns before execution |
| `analyze-data-lineage` | Trace FK-based upstream/downstream table dependencies |
| `discover-joins` | Discover foreign key relationships and suggest JOINs |
| `sample-data` | Fetch sample rows to infer data formats |
| `discover-insights` | Discover KPIs, trends, anomalies, and distribution patterns in database tables |
| `track-schema-changes` | Track schema snapshots/history, generate migrations, and detect schema drift |
| `federated-query` | Execute read-only cross-profile subqueries with optional JOINs, aggregation, and partial-failure metadata |
| `list-tools` | List all available MCP tools and descriptions |
| `analyze-schema` | Comprehensive schema analysis with AI query suggestions and optional advanced profiling (`profiling`) |
| `mcp-info` | Show provider version and author |

## 📖 Documentation

### Core Documentation
- 📋 [**API Documentation**](docs/api-documentation.md) - Detailed API specifications and examples
- 📊 [**Implementation Status**](docs/implementation-status.md) - Current implementation tracking
- 🏗️ [**Technical Specifications**](docs/technical-specifications.md) - Architecture and design details
- 📝 [**PRD Analysis**](docs/prd.md) - Product requirements analysis with AI perspective
- 🔍 [**Schema Introspection Queries**](docs/schema-introspection-queries.md) - Database-specific queries
- 🧪 [**Test Enhanced Schema**](docs/test-enhanced-schema.md) - Test schema documentation
- 🗺️ [**Enhancement Roadmap**](docs/roadmap.md) - Strategic development planning

### Memory Bank Documentation
The project includes a comprehensive memory bank system for AI assistants, located in `.kilocode/rules/memory-bank/`:

- 🏗️ [**Architecture**](.kilocode/rules/memory-bank/architecture.md) - System architecture and component relationships
- 📋 [**Brief**](.kilocode/rules/memory-bank/brief.md) - Project overview and requirements
- 📊 [**Context**](.kilocode/rules/memory-bank/context.md) - Current state and recent changes
- 🎯 [**Product**](.kilocode/rules/memory-bank/product.md) - Problem statement and solution overview
- 💻 [**Tech**](.kilocode/rules/memory-bank/tech.md) - Technology stack and development setup

### Project Planning
- 🗺️ [**Roadmap**](docs/roadmap.md) - Consolidated enhancement plan
- 📊 [**Vertical Slices (History)**](docs/history/vertical-slices.md) - Phase-by-phase development breakdown
- 🔍 [**Architecture Validation (History)**](docs/history/architecture-validation.md) - Technical compatibility analysis
- 🐛 [**MCP Tool Detection Fix (History)**](docs/history/mcp-tool-detection-fix.md) - Critical bug fix documentation

### Version History
- 📋 [**CHANGELOG**](CHANGELOG.md) - Detailed release notes and version history

## 🤝 Contributing

We welcome contributions! Please see our [**Contributing Guidelines**](CONTRIBUTING.md) for development setup and workflow.

## 📄 License

This project is licensed under the [**MIT License**](LICENSE).

## 🔐 Security

For security policies and vulnerability reporting, please see our [**Security Policy**](SECURITY.md).

## 📜 Code of Conduct

Please read our [**Code of Conduct**](CODE_OF_CONDUCT.md) for community guidelines.

## 🧪 Testing

```bash
go test ./...
```

## 📊 Project Status

- **Version:** v1.0.7
- **Built with:** Various vibe coding tools
- **Status:** Production Ready ✅
- All 18 MCP tools are fully implemented and OpenAPI-aligned.
- Enhanced schema introspection and sample data features.
- Optional advanced profiling in `analyze-schema` for column-level statistics, pattern detection, and quality scoring.
- AES-GCM encryption, connection pooling, and structured error handling are enforced.
- Comprehensive unit and integration tests included.
- Ready for production use.

 - **Business Intelligence Discovery**: Added `discover-insights` tool for automatic KPI, trend, anomaly, and distribution analysis

---

## License

MIT

## Enhancement Planning

**Current Development Status**: The Database MCP Server is production-ready with a comprehensive enhancement roadmap in progress.

**Implementation Phases**:
- **Phase 1** (Completed): Query optimization, validation, and enhanced NLP
- **Phase 2** (Completed): Data lineage and business intelligence
- **Phase 3** (Completed): Schema evolution, advanced profiling, and multi-database federation delivered

**Current Progress**:
- `track-schema-changes` is implemented with snapshot tracking, history, migration generation, and drift detection (F2 Phase 4 complete).
- Advanced profiling for `analyze-schema` is implemented with optional `profiling` parameter and backward-compatible response shape (F3 complete).
- `federated-query` is implemented with parser/planner/join/executor/handler modules and dedicated test coverage (F4 complete).

**Planning Documents**:
- [Enhancement Roadmap](docs/roadmap.md) - Strategic overview
- [Project Plan (History)](docs/history/project-plan-roadmap.md) - Comprehensive implementation strategy
- [Vertical Slices (History)](docs/history/vertical-slices.md) - Detailed phase breakdowns
- [Architecture Validation (History)](docs/history/architecture-validation.md) - Technical compatibility analysis

**Ready for immediate enhancement development while maintaining production stability.**
