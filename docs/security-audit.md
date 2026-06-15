# Security audit — ecosystem features

Scope: the mail/calendar/chat/wiki/admin/search changes added on `feature/ecosystem`
(Phases 0–10), plus the shared surfaces they touch (auth/roles, file uploads, SSE,
export/import, sanitizer). Method: manual code review of store/handler/JS, data-flow
tracing for the new endpoints, and targeted checks (SQLi, XSS, SSRF, IDOR, secrets,
DoS, CSRF, role gates).

Overall: the new code reuses the established safe patterns (parameterized SQL,
`sanitizeHTML` on stored HTML, per-user/`IsAdmin` gates, RE2 regex). One high-severity
secret-leak was found and **fixed**; the rest are medium/low hardening items with a
prioritized plan below.

## Severity summary
| # | Severity | Area | Status |
|---|----------|------|--------|
| S1 | High | Secrets (DKIM key, TG token) present in JSON export | ⚠️ Accepted (owner decision) — see note |
| A1 | Medium | No CSRF tokens (relies on SameSite=Lax) | ✅ Fixed |
| A2 | Medium | Outbound SMTP SSRF / open-relay-ish surface | ✅ Fixed |
| A3 | Medium | Inbound SMTP: no rate-limiting / mail-bomb DoS | ✅ Fixed |
| A4 | Low | Self-XSS in wiki Markdown live preview | ✅ Fixed |
| A5 | Low | Mailbox alias can shadow a username | ✅ Fixed |
| A6 | Low | Antispam/DKIM/SPF best-effort, bypassable | Accepted/Deploy |
| A7 | Low | DKIM private key stored plaintext in DB | Accepted/Deploy |
| I1 | Info | Mail-rule regex is RE2 → no ReDoS | No action |
| I2 | Info | New dynamic SQL is int-id/fixed-table only → no SQLi | No action |

---

## Findings

### S1 — Secrets in JSON backup (High) — ACCEPTED (owner decision)
`exportSettings()` dumps the entire `app_settings` table, so the JSON export
(`/api/export`, the in-app JSON backups, and the daily Telegram backup) contains
`mail_dkim_key` (DKIM **private key**) and `telegram_bot_token` in cleartext (the export
also already contains user password **hashes**).

Decision: keep secrets **in** the export so it stays a complete, one-file migratable
backup (a restore onto a fresh instance reproduces DKIM signing and the bot without
manual re-entry). The trade-off accepted is that the JSON export is a top-secret artifact.
(An earlier exclusion was reverted because it also made `ImportAll` — which wipes and
re-inserts `app_settings` — silently destroy the live DKIM key/token on restore.)

**Required mitigations (handle the export as a secret):**
- Restrict who can download `/api/export`; transport/store it encrypted; never commit it.
- The **daily Telegram backup** ships this file through the bot — review whether that
  channel is acceptable for a private key; consider disabling the auto-backup or routing
  it to a trusted destination.
- Recommend volume/disk encryption at rest.
- Optional future hardening: an "include secrets" toggle (default off) for exports used
  for data sharing vs. full migration.

### A1 — CSRF (Medium)
Sessions are cookie-based (`SameSite=Lax`, `HttpOnly`). Mutations are POST/PUT/DELETE
with JSON bodies; Lax + JSON content-type blocks classic form-CSRF, but not all flows.
**Remediation (P1):** verify `Origin`/`Referer` on state-changing requests (reject
cross-origin), or set the session cookie `SameSite=Strict`, or add a per-session CSRF
token header. Lowest-friction: an Origin check in `ServeHTTP` for non-GET.

### A2 — Outbound SMTP SSRF / relay surface (Medium)
`deliverToMX` resolves the recipient domain's MX and connects to it on :25. Recipient
domains are user-controlled (the "to" field). Risk: probing internal infrastructure or
abuse as a sending relay. Mitigations already present: outbound is **off by default**
(needs `mail_domain` configured); it never relays *received* external mail (inbound only
delivers to local users). **Remediation (P1):** before connecting, resolve the MX host
and refuse private/loopback/link-local IPs; add an optional recipient-domain allowlist;
cap outbound concurrency and per-user send rate.

### A3 — Inbound SMTP DoS / mail-bomb (Medium)
The inbound listener (`-smtp-addr`, off by default) has `MaxMessageBytes=25MB` but no
per-IP connection/rate limits or per-session recipient cap, so a peer could flood
mailboxes or exhaust resources. **Remediation (P1):** per-source-IP rate limit and
concurrent-connection cap; max RCPT per session; reuse the `mail_audit.source_ip` data
to auto-throttle/deny repeat offenders.

### A4 — Self-XSS in wiki Markdown preview (Low)
The Markdown editor's live preview renders `marked` output via `innerHTML` **without**
sanitizing (only the author sees it; stored content is sanitized server-side on save).
Impact limited to self-XSS. **Remediation (P2):** sanitize the preview client-side
(small DOMPurify-style pass) or render in a sandboxed iframe.

### A5 — Mailbox alias shadowing (Low)
`resolveLocalRecipient` checks `mail_aliases` before usernames, so an admin could create
an alias equal to another user's username and reroute that user's inbound mail. Admin-only
(trusted), but a footgun. **Remediation (P2):** reject creating an alias whose local-part
equals a different user's username; surface a warning in the Mailboxes tab.

### A6 — Antispam / DKIM / SPF are best-effort (Low, accepted)
Heuristic scoring is bypassable and DKIM/SPF verification degrades to "no verdict" without
DNS. This is expected for a self-contained built-in filter. Real enforcement (SPF/DMARC
reject, greylisting) is deploy-time infrastructure. No code action; documented.

### A7 — DKIM private key at rest (Low, accepted)
`mail_dkim_key` is stored plaintext in `app_settings` (SQLite file). Acceptable for a
single-binary self-hosted app; **recommend** full-disk/volume encryption in deployment
docs. (Export leak already fixed in S1.)

### I1 — Mail-rule regex (Info, no action)
User-supplied rule conditions allow `regex`. Go's `regexp` is RE2 (linear time), so there
is **no ReDoS** risk; invalid patterns are caught and treated as non-matching.

### I2 — Dynamic SQL (Info, no action)
New `IN (...)` clauses are built only from integer IDs (calendar/message ids) or a fixed
table-name list (`ecosystemTables`); LIKE uses `escapeLike` + bound parameters. No SQLi.

---

## Verified-OK (no finding)
- **Role gates:** new admin endpoints (`/api/mail/audit`, `/api/mail/settings`,
  `/api/mail/aliases*`) check `IsAdmin`; folders/rules are scoped by `user_id`; read-only
  users are blocked from all new mutations by the `ServeHTTP` gate.
- **IDOR:** mailbox ops (`MoveMail`, `GetMailBox`, flag, delete) filter by `user_id`;
  calendar edits check `CanEditCalendar`; chat edit/delete check author/admin.
- **XSS (stored):** chat attachment names/URLs via `esc/escAttr`; mail/wiki HTML bodies
  pass `sanitizeHTML` on store; search/audit tables escape output.
- **Orphan-file GC:** chat/mail attachment file_ids are referenced so live attachments
  are not deleted (regression-tested).
- **Secret exposure via API:** `/api/mail/settings` returns only `dkim_configured` + the
  *public* DKIM TXT, never the private key.

## Status update
P1 and P2 are implemented and tested:
- **A1** `sameOriginOK` CSRF guard in `ServeHTTP` (`csrf_test.go`).
- **A2** `deliverToMX` refuses MX hosts resolving to internal IPs (`isInternalIP`, test).
- **A3** inbound SMTP per-IP sliding-window limiter + recipient cap (`smtpRateLimiter`, test).
- **A4** `wikiSanitizePreview` strips active content from the Markdown preview.
- **A5** `CreateAlias` rejects aliases shadowing another user's username (test).

Remaining are deploy-time/operational (A6, A7) — no code change:
- **A6/A7:** publish SPF/DKIM/DMARC DNS records and enable TLS at the MTA edge for real
  inbound enforcement; run on an encrypted volume; protect the JSON export artifact and
  review the Telegram daily-backup channel (see S1).

## Remediation plan (prioritized)
- **P0:** S1 — owner chose to keep secrets in the export (complete one-file backup);
  treat the export as secret (see S1 mitigations); optional "include secrets" toggle later.
- **P1 (next):** A1 Origin/CSRF check on mutations; A2 outbound MX private-IP guard +
  rate cap; A3 inbound per-IP rate limit + RCPT cap. All three are localized, testable
  (unit tests for the guards), and do not change default behavior (SMTP off by default).
- **P2:** A4 sanitize wiki MD preview; A5 alias-vs-username collision guard.
- **P3 / deploy docs:** A6/A7 — document SPF/DKIM/DMARC DNS setup, TLS, and at-rest
  encryption; note antispam is heuristic.

Each P1 item ships with `go test` coverage and a `go run` smoke check, following the
per-phase workflow used so far. None require new runtime dependencies.
