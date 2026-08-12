# Madok.Works — Kanban-centred Team Workspace

[Русская версия](README.RUS.md)

**Madok.Works** is a self-hosted team workspace built around a Kanban board, now a full **ecosystem**: projects with their own board + wiki, chat, calls, calendar and mail under one interface, one binary and one SQLite file (PostgreSQL optional).

## Projects

The top level is a **Project** — a self-contained container with its own Kanban board, wiki spaces and calendars. A default project ("General") exists out of the box; create more from the project selector in the header. Projects have open or restricted visibility and per-project roles (admin/editor/viewer). Tasks are numbered per project as **KEY-123**, and tags are scoped per project. Project moderation lives in **Settings → Board → Projects**. The calendar is cross-project so deadlines and sprints from every project line up on one timeline.

## Ecosystem services

Beyond the Kanban board, optional services share the same auth, roles, notifications, search, themes and i18n. From the **Services** tab, an admin can enable/disable each service (the board included) and reorder them in the service rail; a disabled service is blocked server-side, not just hidden. Each service adds its own contextual selectors to the header (kanban-style), and a single global **multi-term search** (every term must match) spans tasks, comments, wiki, chat, calendar and mail, with a filter to scope to one service.

- **Wiki** — Confluence-style spaces with a page tree, a Markdown editor with a formatting toolbar, revision history and per-space roles (admin/editor/viewer). Includes @mentions, `[[wiki links]]` with backlinks, auto-anchor heading links, `[[toc]]`, task links, comments with reactions, page templates, embedded draw.io diagrams (bundled) and Markdown/PDF export.
- **Chat** — Telegram-style messaging: bubbles, avatars, reactions, replies, pins, a typing indicator and read receipts (ticks) in DMs, plus a contacts list. Public channels, **read-only channels**, **private groups** (with a member picker) and direct messages with live updates (SSE), @-mentions, emoji picker, Markdown and image paste with lightbox, **notification sound with per-channel mute**, search and unread badges.
- **Calls** — built-in **WebRTC** voice/video: 1:1 and small-group mesh calls signalled over SSE+POST, a ring UI, mute and camera toggles, and the ability to invite anyone mid-call. Uses a built-in STUN server plus an optional self-hosted TURN (coturn) configured in settings.
- **Calendar** — private/shared calendars, recurring events (RRULE), month/week/day/agenda views, reminders, a cross-project overlay of task deadlines & sprints, and read-only subscriptions to external ICS/iCal feeds (e.g. Google Calendar).
- **Mail** — internal mail between users out of the box: folders in the header, two-pane view, threads, attachments, search, distribution rules, anti-spam and aliases. Optional inbound SMTP receiving via `-smtp-addr`, plus external send/DKIM configured in the Mail server settings (see docs).
- **Files** — a shared instance-wide file storage (network-drive style): a folder tree with breadcrumbs, multi-file upload with drag-and-drop, rename/move, image/text preview, and **public share links** (token URLs that work without login — revocable, and dark while the service is disabled). Uploads follow the admin-configurable file-type policy; read-only users can browse and download.

## Why this project

- **Minimal dependencies** — Go + SQLite at the core (single `mattn/go-sqlite3`); the mail stack adds a small, self-contained SMTP/MIME library set. No Redis, no message queue. PostgreSQL is supported but optional.
- **One command to run** — `./kanban.sh up --build`, open browser
- **Batteries included** — projects, sprints, epics, per-project tags, file attachments, Markdown, comments with @mentions, multi-term search (text + regex), drag-and-drop, Telegram notifications, **2FA (TOTP)**, audit log
- **Integrations** — Git (GitHub/GitLab/Gitea) webhooks link commits/branches/MRs to tasks by KEY-123/#123 and can auto-move tasks on merge; project-aware Telegram bot
- **Hardened by default** — rootless Podman, read-only FS, all capabilities dropped; bind to localhost with `--app-bind 127.0.0.1`
- **Works offline** — all static assets bundled, no external CDN calls
- **8 themes, 10 languages** — dark/light/ocean/forest/nord/dracula/solarized/spacedust, from English to Arabic

## Quick Start

The stack (app + PostgreSQL) runs via podman-compose; `./kanban.sh` is a thin wrapper that turns the knobs into the variables the compose file reads:

```bash
./kanban.sh up --build
# → http://<LAN-IP>:10010   (first visit = create the admin account)
```

Ports, project name and DB credentials are flags (defaults come from `deploy/.env`):

```bash
./kanban.sh up --app-port 8080 --db-port 5433 -p work     # custom ports + project name
./kanban.sh up --app-bind 127.0.0.1                       # local only (e.g. behind a reverse proxy)
./kanban.sh up --db "postgres://kanban:kanban@db:5432/kanban?sslmode=disable"  # force Postgres from the start
./kanban.sh logs            # follow app logs   (logs db for Postgres)
./kanban.sh backup          # pg_dump → backups/
./kanban.sh down            # stop (keeps data) ·  ./kanban.sh wipe  removes the volumes
```

On the first run the setup page creates the admin and optionally lets you switch to the bundled PostgreSQL; otherwise it stays on SQLite. Run `./kanban.sh` with no arguments for the full command list.

The image build downloads the bundled draw.io editor (~50 MB from GitHub releases). If that download fails on your network (`curl` exit 56), either pre-fetch it once into `vendor-cache/draw.war` (see `vendor-cache/README`) or build without it:

```bash
./kanban.sh build --skip-drawio     # wiki still works; diagrams then need an external drawio_url
```

Behind nginx with TLS (app on localhost, host terminates 443): the wrapper also generates the nginx config and a systemd unit (DB stays internal):

```bash
./kanban.sh up --app-bind 127.0.0.1 --app-port 10010 -p s4m-works --build
./kanban.sh deploy -p s4m-works --app-port 10010 --host mw.example.com   # systemd unit + nginx conf
# (./kanban.sh nginx … / ./kanban.sh systemd … to generate just one)
```
The generated nginx config forwards Host (for the same-origin check), allows large bodies (`client_max_body_size 64m` for backup import) and disables buffering for SSE (live chat/calendar/calls).

## Features at a Glance

| Area | What you get |
|------|-------------|
| Projects | Top-level multi-project containers, each with its own board, wiki and calendars; per-project roles; tasks keyed as KEY-123 |
| Board | Columns with drag-and-drop, inline editing, priorities, deadlines |
| Planning | Sprints (plan → active → done), epics with color coding and progress |
| Collaboration | Nested comments, @mentions, task subscriptions, activity feed |
| Organization | Per-project tags, dependencies (blocks / depends on), TODO checklists |
| Search | Multi-term plaintext and regex across tasks, comments, wiki, chat, calendar and mail, with per-service scope |
| Files | Attachments, image paste (Ctrl+V), Markdown with syntax highlighting |
| Notifications | In-app + project-aware Telegram bot (assignments, mentions, updates) |
| Integrations | Git (GitHub/GitLab/Gitea) webhooks linking commits/branches/MRs to tasks, auto-move on merge |
| Security | 2FA (TOTP, opt-in), audit log, three roles: Admin, User, Read-only; per-space wiki roles |
| Ecosystem | Wiki, Chat, Calls (WebRTC), Calendar, Mail — toggleable and reorderable services in one UI |
| Settings | Single settings page with role-based tabs, opened from the service rail |
| Data | JSON export/import (incl. ecosystem services), scheduled backups via `./kanban.sh backup` |
| Customization | App name / branding, 8 themes, 10 languages, adjustable font size, timezone selector |

## Comparison

| | **Madok.Works** | Trello | Jira | Notion | WeKan | Planka |
|---|---|---|---|---|---|---|
| Free & open source | **Yes, MIT** | Freemium | Freemium | Freemium | Yes, MIT | No (Fair Use) |
| Self-hosted | **Yes** | No | Data Center ($) | No | Yes | Yes |
| External dependencies | **None** (SQLite; Postgres optional) | — | Postgres, Elasticsearch, etc. | — | MongoDB | Postgres, Redis |
| Setup time | **~1 min** | — | Hours | — | ~15 min | ~10 min |
| Multi-project | **Yes** | Yes | Yes | Yes | Yes | Yes |
| Sprints & epics | **Yes** | No | Yes | Basic | No | No |
| Chat & calls (WebRTC) | **Yes** | No | No | No | No | No |
| Git integration | **Yes** | Yes | Yes | Partial | No | No |
| Telegram notifications | **Yes** | No | No | No | No | Yes |
| Offline / air-gapped | **Yes** | No | Yes (DC) | Partial | Partial | Yes |
| Resource usage | **~30 MB RAM** | — | 4+ GB RAM | — | ~400 MB | ~150 MB |
| Themes | **8** | 2 | 1 | 2 | 3 | 1 |
| Languages | **10** | 20+ | 20+ | ~15 | 100+ | 20+ |

Madok.Works is not trying to replace Jira for a 500-person company. It is built for individuals and small teams who want a fast, private, self-hosted workspace that works out of the box without infrastructure overhead.

## Stack

- **Go 1.22** — stdlib `net/http`, no framework
- **SQLite** — single `mattn/go-sqlite3` dependency (PostgreSQL optional via `-db postgres://…`)
- **Vanilla JS** — no React, no Vue, no build step
- **Podman** — rootless container with security hardening

## Documentation

Full guides: **[English](docs/en/README.md)** | **[Русский](docs/ru/README.md)**
