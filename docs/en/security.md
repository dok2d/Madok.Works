# Security

## Container Hardening

The container runs with maximum restrictions:

| Measure                 | Description                                       |
|-------------------------|---------------------------------------------------|
| **Non-root**            | Process runs as the `kanban` user                 |
| **Read-only FS**        | Root filesystem is mounted read-only              |
| **CAP_DROP ALL**        | All Linux capabilities are dropped                |
| **no-new-privileges**   | Privilege escalation is blocked                   |
| **Resource limits**     | 256 MB RAM, 0.5 CPU                              |
| **Network**             | Listens only on 127.0.0.1, exposed via nginx     |
| **Health check**        | Built-in container health check                   |

## Authentication

- **PBKDF2-HMAC-SHA256** — 100,000 iterations, 32-byte salt
- Sessions: random 64-character tokens, 90-day expiry
- HttpOnly cookies — protection against XSS theft
- Automatic cleanup of expired sessions

## Attack Protection

### SQL Injection

All database queries use prepared statements.

### XSS

- Content-Security-Policy (CSP) header
- HTML escaping of user input
- HttpOnly cookies

### File Uploads

- Size limit: 50 MB
- MIME type validation
- Dangerous extensions blocked: `.exe`, `.bat`, `.js`, `.html`, `.svg`, etc.

## Git Webhooks

Inbound git events are received at `/api/integrations/git/webhook/{repoID}` and verified per provider:

- **GitHub / Gitea** — HMAC-SHA256 over the raw request body, compared against the `X-Hub-Signature-256` (GitHub) / `X-Gitea-Signature` (Gitea) header using a per-repository shared secret
- **GitLab** — the `X-Gitlab-Token` header is matched against the configured shared secret

If the configured secret is empty, verification is **skipped** — only use this for trusted/LAN setups. The request body is read with a size cap to bound resource use.

## Calls

Audio/video calls use **WebRTC**:

- Media is **peer-to-peer** — the server only relays signaling (over SSE + POST) and never sees the media streams
- A **STUN** server is built in for NAT discovery
- For reliable connectivity behind restrictive NAT/firewalls, a self-hosted **TURN** server (coturn) can be configured (see [Deployment](deployment.md))

## nginx

The nginx configuration includes:

- TLS 1.2 / 1.3 with modern ciphers
- HSTS (HTTP Strict Transport Security)
- X-Frame-Options: DENY
- X-Content-Type-Options: nosniff
- Referrer-Policy: strict-origin-when-cross-origin
- Rate limiting zone for brute-force protection
- Request body limit: 2 MB

## Recommendations

- Use TLS in production (self-signed or Let's Encrypt)
- Make regular [backups](backup.md)
- Restrict server access via firewall
- Use strong passwords for the administrator
- Consider LDAP/OIDC for enterprise use
