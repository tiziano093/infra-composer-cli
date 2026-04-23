# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| 1.x     | Yes       |

## Reporting a Vulnerability

**Do not open a public issue for security vulnerabilities.**

Report privately via [GitHub Security Advisories](https://github.com/tiziano093/infra-composer-cli/security/advisories/new).

Include:
- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if any)

Response time: acknowledgement within 48 hours, patch within 14 days for critical issues.

## Scope

In scope:
- Command injection via user-supplied flags
- Path traversal in `--output-dir` or `--schema`
- Credential leakage in generated Terraform files
- Supply chain issues (dependency confusion, typosquatting)

Out of scope:
- Vulnerabilities in generated Terraform code (provider responsibility)
- Issues requiring physical access to the machine
