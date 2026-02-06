# Database MCP Server - Product Document

## Why This Project Exists

Developers and AI agents often need database access across different SQL engines, but integration is usually fragmented and unsafe when done ad hoc.

Database MCP Server provides a unified MCP tool interface with consistent behavior and safety controls.

## Product Value

- One interface across MySQL, MariaDB, PostgreSQL, SQLite
- Structured tool contracts for agent workflows
- Secure credential handling and read-only execution controls
- Practical discovery + intelligence + governance workflows in one server

## How It Works

1. Client calls an MCP tool
2. Server validates parameters and policy (including read-only mode)
3. Server executes DB operation via selected profile
4. Structured response is returned to the client

## User Outcomes

### For Developers
- Faster onboarding to unfamiliar databases
- Less manual wiring across tools and drivers
- Better query quality through validate/optimize flow

### For AI Agents
- Discoverable capabilities via `list-tools`
- Schema-aware SQL workflows
- Reliable structured responses for chaining tasks

### For Teams/Admins
- Safer defaults for exploratory access
- Centralized profile and operation model
- Better maintainability of agent-db integrations

## Supported Tool Surface

The product currently exposes 18 tools, spanning:
- profile/connectivity,
- schema discovery,
- SQL execution,
- query intelligence,
- lineage/insight/schema governance,
- cross-profile federation,
- runtime metadata.

## Success Metrics

- Low time-to-first-query for new users
- Stable MCP integration across major coding clients
- No plaintext credential storage
- Up-to-date documentation synchronized with runtime contracts
