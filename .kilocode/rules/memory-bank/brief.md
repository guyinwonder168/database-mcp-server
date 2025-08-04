# Database MCP Server - Project Brief

## Project Overview
A production-ready Model Context Protocol (MCP) provider for SQL databases, written in Go. This server enables AI agents and developers to interact with multiple database types through a unified conversational API.

## Core Mission
Provide a secure, stateless, and unified interface for accessing MySQL, MariaDB, PostgreSQL, and SQLite databases via the Model Context Protocol, abstracting away database-specific complexities.

## Key Requirements

### Functional Requirements
1. **Multi-Database Support**: MySQL, MariaDB, PostgreSQL, SQLite
2. **Profile Management**: Create, update, list database connection profiles
3. **SQL Execution**: Execute arbitrary SQL queries with read-only enforcement option
4. **Schema Introspection**: List tables/views, describe table schemas, list databases
5. **Interactive Setup**: CLI-based configuration wizard for first-time setup
6. **MCP Actions**: Full suite of database operations exposed as MCP tools

### Non-Functional Requirements
1. **Security**: AES-GCM encryption for stored passwords (32-char key)
2. **Performance**: Connection pooling with configurable max pool size
3. **Logging**: Structured JSON logging with file rotation (500KB limit)
4. **Statelessness**: Each action opens/closes its own connection
5. **Configuration**: Human-readable YAML configuration file

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