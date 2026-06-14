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
| S1 | High | Secrets (DKIM key, TG token) leaked in JSON export | ✅ Fixed |
| A1 | Medium | No CSRF tokens (relies on SameSite=Lax) | Planned P1 |
| A2 | Medium | Outbound SMTP SSRF / open-relay-ish surface | Planned P1 |
| A3 | Medium | Inbound SMTP: no rate-limiting / mail-bomb DoS | Planned P1 |
| A4 | Low | Self-XSS in wiki Markdown live preview | Planned P2 |
| A5 | Low | Mailbox alias can shadow a username | Planned P2 |
| A6 | Low | Antispam/DKIM/SPF best-effort, bypassable | Accepted/Deploy |
| A7 | Low | DKIM private key stored plaintext in DB | Accepted/Deploy |
| I1 | Info | Mail-rule regex is RE2 → no ReDoS | No action |
| I2 | Info | New dynamic SQL is int-id/fixed-table only → no SQLi | No action |

---

## Findings

### S1 — Secrets exported in JSON backup (High) — FIXED
`exportSettings()` dumped the entire `app_settings` table, so JSON export
(`/api/export`, downloadable by admins) contained `mail_dkim_key` (DKIM **private
key**) and `telegram_bot_token` in cleartext. A leaked JSON export would expose them.
**Fix:** `secretSettingKeys` denylist excludes those keys from JSON export; they remain
only in the live DB and the operator-controlled full `.db` backup. (`internal/db/store.go`.)

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

## Remediation plan (prioritized)
- **P0 (done):** S1 secret export leak — fixed and committed.
- **P1 (next):** A1 Origin/CSRF check on mutations; A2 outbound MX private-IP guard +
  rate cap; A3 inbound per-IP rate limit + RCPT cap. All three are localized, testable
  (unit tests for the guards), and do not change default behavior (SMTP off by default).
- **P2:** A4 sanitize wiki MD preview; A5 alias-vs-username collision guard.
- **P3 / deploy docs:** A6/A7 — document SPF/DKIM/DMARC DNS setup, TLS, and at-rest
  encryption; note antispam is heuristic.

Each P1 item ships with `go test` coverage and a `go run` smoke check, following the
per-phase workflow used so far. None require new runtime dependencies.
