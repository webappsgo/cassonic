# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| Latest stable release | Yes |
| Previous minor release | Security fixes only |
| Older releases | No |

## Reporting a Vulnerability

**Do NOT open a public GitHub issue for security vulnerabilities.**

Report security issues privately:

- **Email:** casjay@yahoo.com
- **Subject line:** `[SECURITY] cassonic - <brief description>`
- **Expected response:** Within 72 hours for acknowledgment; patch timeline disclosed in response

Include in your report:
- Affected version and platform
- Description of the vulnerability
- Steps to reproduce
- Potential impact

## Disclosure Policy

- We will acknowledge receipt within 72 hours
- We will investigate and provide a timeline within 7 days
- We will coordinate a fix and disclosure date with the reporter
- Credit will be given to reporters in the release notes unless anonymity is requested

## Out of Scope

- Issues in dependencies (report upstream; we will update when patches are available)
- Issues requiring physical access to the server
- Social engineering attacks

## Security Features

See the [security documentation](https://local-cassonic.readthedocs.io/security/) for information on:
- Authentication and authorization model
- TLS configuration
- Security headers
- Rate limiting and IP blocking
- Tor hidden service
