# Ecosystem Services

The Kanban board is one of several services in the **Madok.Works** self-hosted
ecosystem (compact mark **MW**). All services share the same authentication, roles,
notifications, global search, themes and localization, and are reached from the
**app rail** (left/bottom bar).

Admins enable or disable each service (including the **Kanban board itself**) and
change their order in **Settings → Services** — every service is toggleable. A
disabled service is hidden from the UI and unavailable on the server (its API
returns `403`/`404`).

Madok.Works runs on its built-in database out of the box, with an optional
**PostgreSQL** backend (`-db postgres://…`) for larger deployments.

## Projects

A **Project** is the top-level container that scopes the **kanban board**, **wiki**
and **calendars** that belong to it. Out of the box there is a default **"General"**
project, and admins can create more.

- **Visibility:** each project is `open` (all users) or `restricted` (members only).
- **Roles** within a project: `admin` / `editor` / `viewer`.
- **Per-project task keys:** tasks are numbered `KEY-123` using the project's key, so
  numbers stay unique and meaningful across projects. Tags are per-project too.
- **Project selector in the header** switches the active project; the kanban board and
  wiki spaces shown depend on the selected project.
- **The calendar is cross-project** — it spans all projects you can see rather than
  being scoped to one.
- Project creation and moderation live in **Settings → Board → Projects**.

## Wiki

A Confluence-style documentation service.

- **Spaces** with `admin` / `editor` / `viewer` roles and `open` (all users) or
  `restricted` (members only) visibility.
- **Space selection lives in the app header** (a dropdown styled like the Kanban
  sprint filter). The **page tree** is in the sidebar; reorder pages from the page
  menu (move up/down, indent/outdent).
- **Markdown-only editor**: a textarea with live preview plus a formatting toolbar —
  headings, bold / italic / strikethrough, inline code, lists, quote, code block,
  link, image insertion (upload), table (drag to pick size → markdown table), and
  callouts.
- **Revision history** per page with view and restore. Both the markdown source and
  the sanitized HTML are stored.
- **Auto-anchor heading links:** every heading gets a stable anchor; hovering a
  heading shows a **#** you can click to copy a direct link to that section.
- There is no separate wiki search — the shared **global search** is used.

## Chat

Real-time messaging, modeled closely on Telegram.

- **Public channels**, private **groups**, and one-to-one **direct messages**. The
  DM section lists every other user as a contact up front; clicking one starts a
  conversation.
- **Private group creation** with a **member picker** — choose exactly who joins when
  you create the group.
- **Read-only channels:** a channel can be marked read-only so that only its
  **designated writers** (plus the creator and admins) can post, while everyone else
  reads along.
- **Notifications:** a global **notification-sound toggle**, plus **per-channel mute**
  for **1 hour / 8 hours / forever**.
- **Messages**: text and attachments (files and inline images), editing, deleting,
  and **@mentions** (in-app + Telegram notifications).
- **Replies** (quoting) with a jump to the original message.
- **Emoji reactions** (positive and negative), toggleable, with your own reaction
  highlighted.
- **Pinned messages**: a pinned bar at the top (multiple, paging, jump), pin/unpin
  (managers in public channels; any member in groups/DMs).
- Per-message actions live in a dropdown menu (⋯): reply / react / pin / edit /
  delete.
- **Telegram-style look**: bubbles (your own on the right with an accent, others on
  the left), initials avatars, author name in groups/channels, timestamp in the
  bubble corner, date separators, and grouping of consecutive messages.
- **Read receipts** in DMs (✓ sent / ✓✓ read by the other person).
- A **typing…** indicator.
- **System messages** (chat created, joined/left, renamed).
- **Chat list**: avatar, name, last-message preview, time, and unread badge; sorted
  by last-message date.
- Live updates over Server-Sent Events; unread badges on the app rail. There is no
  separate chat search — the shared **global search** is used.

### Calls

Voice and video calls run in the browser over **WebRTC**.

- **1:1 and small-group calls** using a mesh topology (each participant connects
  directly to the others).
- A **ring UI** for incoming calls, with in-call controls to **mute** and toggle the
  **camera**.
- **Invite anyone mid-call** to join an ongoing call.
- Signaling goes over the existing SSE + POST channel. A **STUN** server is built in;
  an optional **self-hosted TURN** server (e.g. coturn) can be configured for relaying
  through stricter NATs/firewalls.

## Calendar

- **Private** and **shared** calendars with colors.
- **View selection** (month / week / day / agenda) and the **calendar list**
  (visibility via checkboxes) live in the **header** (dropdowns styled like the
  Kanban controls); the separate calendar sidebar has been removed.
- **Navigation:** ISO week number in the month and week views; paging through
  periods with the **mouse wheel** (except in Agenda); clicking the year opens a
  mini calendar to pick a year, then a month.
- **Recurring events** (RRULE: `FREQ` daily/weekly/monthly/yearly, `INTERVAL`,
  `COUNT`, `UNTIL`).
- **Configurable reminders** (Telegram / in-app / both), including for recurring
  events.
- A read-only overlay of **task deadlines and sprints** from the board; toggleable,
  and clicking one opens the board item.
- **External calendar subscriptions (ICS/iCal)**, read-only. Each user adds the
  "secret address in iCal format" of their calendar (for example, Google Calendar)
  from the "Calendars" dropdown → **"+ Google Calendar (ICS)"**. Events are shown as
  a read-only overlay in their own color; the server periodically (every ~15
  minutes) syncs subscriptions. Subscriptions are private (visible only to their
  owner). There is SSRF protection, a request timeout, and a response size limit.
  There is no two-way sync or OAuth — ICS import only.
- All times are stored in UTC and shown in your timezone.

## Mail

Internal mail works out of the box; external SMTP is optional.

- **Internal mail** between users: folders (Inbox/Sent/Drafts/Spam/Trash + custom),
  threads, attachments, unread badge, flag/trash, reply with quoting.
- **Folders live in the header** as a dropdown, immediately followed by the
  **Compose** button (Kanban style). The separate mail search has been removed — the
  shared **global search** is used.
- **Two-pane interface** on wide screens: message list on the left, the selected
  message on the right; the divider is draggable (the width is remembered in the
  browser).
- **Mail routing rules** (filters) live in **Settings, the "Mail" tab**.
- **Anti-spam**: heuristics + DKIM verification of inbound mail → the "Spam"
  folder. A mail-event **audit log** (for admins), **mailbox/alias** management
  (admin), and mail server settings: domain, DKIM selector and private key (admin;
  the "Mail" / "Mail server" tabs).
- **Inbound SMTP** (optional): start a listener with `-smtp-addr` (e.g. `:25`).
  Received messages are MIME-parsed and filed into the matching local user's inbox
  (the address local-part is matched to a username).
- **Outbound SMTP** (optional): domain + DKIM (via env or admin settings);
  recipients written as full email addresses are relayed to their MX hosts,
  DKIM-signed if a key is set.
- **Protection**: inbound rate-limiting, outbound SSRF guard, CRLF/header-injection
  protection, and size limits.

### Mail configuration

| Setting | Where | Purpose |
|---------|-------|---------|
| `-smtp-addr` | CLI flag | Inbound SMTP listen address (empty = receiving off) |
| `KANBAN_MAIL_DOMAIN` | env | Sending domain; enables outbound when set |
| `KANBAN_DKIM_SELECTOR` | env | DKIM selector (default `default`) |
| `KANBAN_DKIM_KEY` | env | Path to a PEM RSA private key for DKIM signing |

The same outbound settings (domain, DKIM selector and key) can be set in the admin
UI under the "Mail server" tab — these override the environment variables.

> **Deployment note.** External email delivery requires real infrastructure:
> a public IP, DNS records (MX, SPF, DKIM, DMARC) and TLS. These are configured
> at the network/DNS level, outside the application. Internal mail and inbound
> parsing need none of this.

## Global search

Press **Ctrl/Cmd+K** (or use the search field in the header) to search across all
services at once. The field has a service selector: **all / kanban / wiki / chat /
calendar / mail**; selecting a result jumps to that service.
