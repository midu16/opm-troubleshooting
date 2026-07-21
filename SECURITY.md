# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 1.1.x   | :white_check_mark: |
| < 1.0   | :x:                |

## Reporting a Vulnerability

If you discover a security vulnerability in this project, please report it responsibly.

**Do not open a public GitHub issue for security vulnerabilities.**

Instead, please send an email to **midu@redhat.com** with:

- A description of the vulnerability
- Steps to reproduce the issue
- Potential impact
- Suggested fix (if any)

You should receive a response within 48 hours. If the vulnerability is confirmed, a fix will be developed and released as soon as possible, and you will be credited in the release notes (unless you prefer to remain anonymous).

## Security Best Practices

When using this tool:

- Never commit registry credentials or authentication tokens
- Use read-only access when inspecting catalog images
- Review the `.env.example` file for required environment variables and avoid storing secrets in plain text
- Run with least-privilege access when connecting to live OpenShift clusters
