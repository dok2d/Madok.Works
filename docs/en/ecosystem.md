# Ecosystem Services

The Kanban board is one of several services in a small self-hosted ecosystem. All
services share the same authentication, roles, notifications, global search,
themes and localization, and are reached from the **app rail** (left/bottom bar).

Admins enable or disable each service per instance in **Settings → Services**.
Disabled services are hidden from the UI and their API returns `404`.

## Wiki

A Confluence-style documentation service.

- **Spaces** with a nested **page tree**; drag-free reordering (move up/down,
  indent/outdent) from the page menu.
- **WYSIWYG editor** (headings, lists, code blocks, tables, callouts, images).
- **Revision history** per page with view and restore.
- **Per-space roles**: `admin` / `editor` / `viewer`, plus `open` (all users) or
  `restricted` (members only) visibility.
- Full-text **search** across spaces you can see.

## Chat

Real-time messaging.

- **Public channels**, private **groups**, and one-to-one **direct messages**.
- Live updates over Server-Sent Events; unread badges on the app rail.
- **@mentions** create in-app and Telegram notifications.
- Message **edit / delete**, history paging, and search across visible channels.

## Calendar

- **Private** and **shared** calendars with colors.
- **Month / week / day / agenda** views; drag an event to another day to
  reschedule it.
- **Recurring events** (RRULE: `FREQ` daily/weekly/monthly/yearly, `INTERVAL`,
  `COUNT`, `UNTIL`).
- **Reminders** (~15 minutes before) via in-app + Telegram notifications.
- A read-only overlay of **task deadlines and sprints** from the board; clicking
  one opens the board item.
- All times are stored in UTC and shown in your timezone.

## Mail

Internal mail works out of the box; external SMTP is optional.

- **Internal mail** between users: folders (Inbox/Sent/Drafts/Trash), threads,
  attachments, search, unread badge, flag/trash, reply with quoting.
- **Inbound SMTP** (optional): start a listener with `-smtp-addr` (e.g. `:25`).
  Received messages are MIME-parsed and filed into the matching local user's
  inbox (the address local-part is matched to a username).
- **Outbound SMTP** (optional): when configured, recipients written as full email
  addresses are relayed to their MX hosts, DKIM-signed if a key is set.

### Mail configuration

| Setting | Where | Purpose |
|---------|-------|---------|
| `-smtp-addr` | CLI flag | Inbound SMTP listen address (empty = receiving off) |
| `KANBAN_MAIL_DOMAIN` | env | Sending domain; enables outbound when set |
| `KANBAN_DKIM_SELECTOR` | env | DKIM selector (default `default`) |
| `KANBAN_DKIM_KEY` | env | Path to a PEM RSA private key for DKIM signing |

> **Deployment note.** External email delivery requires real infrastructure:
> a public IP, DNS records (MX, SPF, DKIM, DMARC) and TLS. These are configured
> at the network/DNS level, outside the application. Internal mail and inbound
> parsing need none of this.

## Global search

Press **Ctrl/Cmd+K** (or the 🔍 button in the header) to search across the board,
wiki, chat and mail at once; selecting a result jumps to that service.
