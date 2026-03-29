# Contributing to Database MCP Server

Thank you for your interest in contributing to the Database MCP Server! This document provides guidelines and information for contributors.

## Development Setup

1. **Prerequisites**
   - Go 1.25.5 or later
   - Git
   - Access to supported databases (MySQL, MariaDB, PostgreSQL, SQLite) for testing

2. **Setup**
   ```bash
   git clone https://github.com/guyinwonder168/database-mcp-server.git
   cd database-mcp-server
   go mod download
   go build -o mcp-server ./cmd/server/main.go
   ```

3. **Running Tests**
   ```bash
   # Run all tests
   go test ./...
   
   # Run tests with coverage
   go test -cover ./...
   
   # Run live integration tests (requires database setup)
   DB_MCP_IT_PG_HOST=localhost DB_MCP_IT_PG_PORT=5432 DB_MCP_IT_PG_USER=test DB_MCP_IT_PG_PASS=test DB_MCP_IT_PG_DB=test go test ./internal/mcp -run TestLive -count=1
   ```

## Code Style Guidelines

- Follow Go conventions and best practices
- Use `gofmt` for code formatting
- Write meaningful commit messages with conventional commit format
- Add comprehensive tests for new features
- Update documentation for any API changes

## Contribution Workflow

1. **Fork the repository**
2. **Create a feature branch**: `git checkout -b feature/your-feature-name`
3. **Make your changes**:
   - Write code following the existing patterns
   - Add tests for new functionality
   - Update documentation as needed
4. **Test your changes**:
   ```bash
   go test ./...
   go vet ./...
   go fmt ./...
   ```
5. **Commit your changes**:
   ```bash
   git add .
   git commit -m "feat: add your feature description"
   ```
6. **Push to your fork**: `git push origin feature/your-feature-name`
7. **Create a Pull Request** with:
   - Clear description of changes
   - Link to relevant issues
   - Test results

## Types of Contributions

### Bug Fixes
- Include steps to reproduce
- Add tests that cover the fix
- Update documentation if behavior changes

### New Features
- Follow existing MCP tool patterns
- Add comprehensive tests
- Update API documentation
- Consider backward compatibility

### Documentation
- Fix typos or clarify unclear sections
- Add examples for new use cases
- Update README.md for major changes

## MCP Tool Development

When adding new MCP tools:

1. **Add tool handler** in `internal/mcp/server.go`
2. **Register the tool** in the tools list
3. **Add comprehensive tests** in `internal/mcp/server_test.go`
4. **Update OpenAPI spec** in `docs/mcp-openapi.yaml`
5. **Update README.md** with usage examples
6. **Run integration tests** to ensure tool discovery works

## Testing Requirements

- Unit tests for all new functions
- Integration tests for MCP tools
- **Database-specific testing with go-sqlmock** for MySQL/PostgreSQL tests (see `server_analyze_schema_helpers_test.go` for examples)
- **Real SQLite testing** using in-memory databases for SQLite functionality
- Live database tests when applicable
- Test error scenarios and edge cases

### Testing Approach by Database

- **MySQL/MariaDB & PostgreSQL**: Use go-sqlmock for mocking database-specific queries and system catalogs (INFORMATION_SCHEMA, information_schema)
- **SQLite**: Use real in-memory databases (`:memory:`) to test actual SQLite functionality
- **Live Integration**: Use `DB_MCP_IT_*` environment variables for testing against real database instances

## Code Review Process

1. **Automated checks** must pass
2. **Manual review** focuses on:
   - Code quality and maintainability
   - Test coverage and effectiveness
   - Documentation accuracy
   - Security considerations
3. **Approval** required before merge

## Security Considerations

- Never commit secrets or credentials
- Follow secure coding practices
- Test for SQL injection vulnerabilities
- Validate all inputs properly

## Release Process

Releases are managed by the maintainers:
1. Update version in documentation
2. Update CHANGELOG.md
3. Create git tag
4. Build and test release binary
5. Create GitHub release

## Getting Help

- Check existing issues and documentation
- Create an issue for questions or problems
- Join discussions for feature ideas

Thank you for contributing to Database MCP Server!