# Security Policy

## Supported Versions

Only the latest version of Database MCP Server receives security updates. Users are encouraged to upgrade to the most recent release.

## Reporting a Vulnerability

If you discover a security vulnerability, please report it responsibly before disclosing it publicly.

### How to Report

- **Email**: security@guyinwonder.dev
- **Private Issue**: Create a private GitHub issue
- **Include**: 
  - Detailed description of the vulnerability
  - Steps to reproduce the issue
  - Potential impact assessment
  - Any available proof-of-concept

### Response Timeline

- **Initial Response**: Within 48 hours
- **Detailed Assessment**: Within 7 days
- **Public Disclosure**: After fix is released, or with user permission

## Security Features

### Credential Protection
- **AES-GCM Encryption**: All passwords encrypted at rest with 256-bit AES
- **Key Management**: 32-character encryption key with secure generation
- **No Plaintext Storage**: Credentials never stored or logged in plaintext
- **Automatic Redaction**: Sensitive data automatically redacted from logs

### Access Control
- **Read-only Profiles**: Configurable read-only access to prevent accidental writes
- **SQL Injection Prevention**: Parameterized queries and input validation
- **Connection Isolation**: Each operation uses separate database connections
- **Profile-based Access**: Database permissions enforced at connection level

### Transport Security
- **SSL/TLS Support**: Encrypted database connections
- **Certificate Validation**: Configurable SSL modes for PostgreSQL
- **Secure Defaults**: Default to secure connection settings

### Operational Security
- **Structured Logging**: Comprehensive audit trail with credential redaction
- **Error Handling**: Secure error messages without information leakage
- **Input Validation**: Comprehensive parameter validation and sanitization
- **Resource Limits**: Configurable connection pooling prevents resource exhaustion

## Best Practices for Users

### Configuration Security
- Use strong, random AES keys (32 characters minimum)
- Set appropriate file permissions on `config.yaml` (600)
- Never commit configuration files to version control
- Use read-only profiles for AI/agent access
- Rotate encryption keys if compromise suspected

### Database Security
- Use least-privilege database users
- Enable SSL/TLS for all database connections
- Regularly update database drivers and dependencies
- Monitor database access logs
- Use separate credentials for different environments

### Operational Security
- Monitor `mcp-provider.log` for unusual activity
- Regularly update to latest version
- Use firewall rules to restrict database access
- Implement backup and recovery procedures
- Test security updates in staging before production

## Vulnerability Categories

### Critical
- Remote code execution
- Credential disclosure
- Database access bypass
- Data encryption bypass

### High
- SQL injection vulnerabilities
- Authentication bypass
- Privilege escalation
- Information disclosure

### Medium
- Denial of service
- Cross-site scripting (if web interface added)
- Configuration bypass

### Low
- Information leakage in logs
- Weak cryptography
- Missing security headers

## Security Testing

### Automated Testing
```bash
# Run security-focused tests
go test ./... -tags security

# Check for known vulnerabilities
go list -m -json all | nancy sleuth

# Static analysis
gosec ./...
```

### Manual Testing
- Penetration testing of MCP endpoints
- Database connection security validation
- Configuration file access testing
- Log analysis for sensitive data leakage

## Dependencies

We regularly update dependencies to address security vulnerabilities:

- **Go Modules**: `go get -u ./...` and `go mod tidy`
- **Database Drivers**: Keep updated to latest stable versions
- **Security Scanning**: Regular automated vulnerability scans

## Security Updates

### Update Process
1. **Assessment**: Vulnerability impact analysis
2. **Development**: Security fix implementation
3. **Testing**: Comprehensive security testing
4. **Release**: Security update with CVE details
5. **Notification**: Security advisory and update instructions

### Update Channels
- **GitHub Releases**: Security updates published with detailed notes
- **CHANGELOG.md**: Security fixes documented with version info
- **Advisories**: Security bulletins for critical vulnerabilities

## Contact

For security-related questions or concerns:
- **Security Email**: security@guyinwonder.dev
- **GitHub Issues**: Use "security" label for sensitive reports
- **Discussions**: Non-sensitive security discussions welcome

Thank you for helping keep Database MCP Server secure!