# Database MCP Server

[![Go](https://img.shields.io/badge/Go-1.25.5%2B-00ADD8?logo=Go&logoColor=white)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](https://opensource.org/licenses/MIT)
[![Version](https://img.shields.io/badge/Version-v1.0.2-blue.svg)](https://github.com/guyinwonder168/database-mcp-server/releases/tag/v1.0.2)

A production-ready Model Context Protocol (MCP) provider for SQL databases, written in Go by guyinwonder. Supports MySQL, MariaDB, PostgreSQL, and SQLite. Features robust connection pooling, secure AES-GCM credential storage, structured JSON logging, comprehensive schema introspection, and a full suite of 12 MCP tools. Built and tested with Go 1.25.5.

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
- 🧭 **Data Lineage** - Analyze upstream/downstream dependencies via `analyze-data-lineage`

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
| `list-tools` | List all available MCP tools and descriptions |
| `analyze-schema` | Comprehensive schema analysis with AI query suggestions |
| `mcp-info` | Show provider version and author |

## 📖 Documentation

### Core Documentation
- 📋 [**API Documentation**](docs/api-documentation.md) - Detailed API specifications and examples
- 📊 [**Implementation Status**](docs/implementation-status.md) - Current implementation tracking
- 🏗️ [**Technical Specifications**](docs/technical-specifications.md) - Architecture and design details
- 📝 [**PRD Analysis**](docs/prd.md) - Product requirements analysis with AI perspective
- 🔍 [**Schema Introspection Queries**](docs/schema-introspection-queries.md) - Database-specific queries
- 🧪 [**Test Enhanced Schema**](docs/test-enhanced-schema.md) - Test schema documentation
- 🗺️ [**Implementation Roadmap**](docs/implementation-roadmap.md) - Strategic development planning

### Memory Bank Documentation
The project includes a comprehensive memory bank system for AI assistants, located in `.kilocode/rules/memory-bank/`:

- 🏗️ [**Architecture**](.kilocode/rules/memory-bank/architecture.md) - System architecture and component relationships
- 📋 [**Brief**](.kilocode/rules/memory-bank/brief.md) - Project overview and requirements
- 📊 [**Context**](.kilocode/rules/memory-bank/context.md) - Current state and recent changes
- 🎯 [**Product**](.kilocode/rules/memory-bank/product.md) - Problem statement and solution overview
- 💻 [**Tech**](.kilocode/rules/memory-bank/tech.md) - Technology stack and development setup

### Project Planning
- 🗺️ [**Roadmap**](project-plan/roadmap.md) - Comprehensive implementation strategy
- 📊 [**Vertical Slices**](project-plan/vertical-slices.md) - Phase-by-phase development breakdown
- 🔍 [**Architecture Validation**](project-plan/architecture-validation.md) - Technical compatibility analysis
- 📝 [**Implementation Tasks**](project-plan/implementation-tasks.md) - Detailed task tracking
- 🐛 [**MCP Tool Detection Fix**](project-plan/mcp-tool-detection-fix.md) - Critical bug fix documentation

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

- **Version:** v1.0.2
- **Author:** guyinwonder
- **Status:** Production Ready ✅
- All 12 MCP tools are fully implemented and OpenAPI-aligned.
- Enhanced schema introspection and sample data features.
- AES-GCM encryption, connection pooling, and structured error handling are enforced.
- Comprehensive unit and integration tests included.
- Ready for production use.

### Recent Enhancements (v1.0.2)
- **Memory Bank System**: Added comprehensive AI assistant memory bank for project context preservation
- **Documentation Suite**: Complete documentation overhaul with API specs, implementation status, and technical specifications
- **Project Planning**: Detailed roadmap and implementation tracking documents
- **Enhanced Testing**: Added integration tests and MCP tool discovery regression tests
- **Improved Logging**: Better credential redaction and structured error handling
- **MCP Resources**: Added `tools://list` and `profile://{profile}` resources for read-only inspection
- **SSE Transport**: Optional HTTP/SSE transport support for additional client compatibility
- **Git Configuration**: Improved .gitignore to exclude logs and build artifacts

---

## License

MIT

## Enhancement Planning

**Current Development Status**: The Database MCP Server is production-ready with a comprehensive enhancement roadmap in progress.

**Implementation Phases**:
- **Phase 1** (Next 60 Days): Query optimization, validation, and enhanced NLP
- **Phase 2** (60-90 Days): Data lineage and business intelligence
- **Phase 3** (90+ Days): Schema evolution, advanced profiling, and multi-database federation

**Planning Documents**:
- [Implementation Roadmap](docs/implementation-roadmap.md) - Strategic overview
- [Project Plan](../project-plan/roadmap.md) - Comprehensive implementation strategy
- [Vertical Slices](../project-plan/vertical-slices.md) - Detailed phase breakdowns
- [Architecture Validation](../project-plan/architecture-validation.md) - Technical compatibility analysis

**Ready for immediate enhancement development while maintaining production stability.**
