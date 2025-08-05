# Database MCP Server - Project Brief

## Project Overview
A production-ready Model Context Protocol (MCP) provider for SQL databases, written in Go. This server enables AI agents and developers to interact with multiple database types through a unified conversational API. All 12 MCP tools (including analyze-schema) are implemented and comprehensively documented in the README, with configuration and usage examples for MySQL, MariaDB, PostgreSQL, and SQLite.

## Core Mission
Provide a secure, stateless, and unified interface for accessing MySQL, MariaDB, PostgreSQL, and SQLite databases via the Model Context Protocol, abstracting away database-specific complexities. Ensure robust documentation, easy onboarding, and safe credential management.

## Key Requirements

### Functional Requirements
1. **Multi-Database Support**: MySQL, MariaDB, PostgreSQL, SQLite
2. **Profile Management**: Create, update, list database connection profiles
3. **SQL Execution**: Execute arbitrary SQL queries with read-only enforcement option
4. **Schema Introspection**: List tables/views, describe table schemas, list databases
5. **Sample Data Fetching**: Fetch sample rows from tables to infer data formats.
6. **Interactive Setup**: CLI-based configuration wizard for first-time setup
7. **Analyze-Schema**: Advanced schema analysis MCP tool supporting BASIC, DETAILED, and COMPREHENSIVE levels, business context inference, data quality metrics, relationship discovery, Smart Query Builder integration
8. **MCP Actions**: Full suite of 12 database operations exposed as MCP tools, all documented in README.md

### Non-Functional Requirements
1. **Security**: AES-GCM encryption for stored passwords (32-char key)
2. **Performance**: Connection pooling with configurable max pool size
3. **Logging**: Structured JSON logging with file rotation (500KB limit)
4. **Statelessness**: Each action opens/closes its own connection
5. **Configuration**: Human-readable YAML configuration file with obsolete user_key/user_secret fields removed

## Project Constraints
- Written in Go (Golang) using the official Go MCP SDK
- Must work both locally and as a remote server
- No GUI - CLI and MCP interface only
- No transaction management beyond atomic operations
- Relies on database-level permissions for access control

## Success Criteria
- Seamless integration with Kilocode AI and other MCP-compatible systems
- Zero plaintext credential storage
- Robust error handling with structured error responses
- Easy setup process for non-technical users
- Production-ready reliability and performance
- Comprehensive documentation for all MCP tools and configuration scenarios