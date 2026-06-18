# Configuration

## Server Startup Flags

The server (`cmd/server/main.go`) accepts the following flags:

| Flag         | Description                                                       | Default           |
|--------------|-----------------------------------------------------------------|-------------------|
| `-addr`      | Server address and port                                         | `:8080`           |
| `-db`        | Database: a SQLite file path **or** a `postgres://…`/`postgresql://…` URL | `/data/kanban.db` |
| `-data-dir`  | Directory for on-disk artifacts (backups); defaults to the SQLite file's directory, or `/data` for Postgres | `` (auto)         |
| `-verbose`   | Verbose logging                                                | `false`           |
| `-smtp-addr` | Inbound SMTP listen address (e.g. `:25`); empty disables it     | `` (disabled)     |

### External database (PostgreSQL)

The backend is chosen by the `-db` value: a file path → SQLite (default), a
`postgres://user:pass@host:5432/dbname` URL → PostgreSQL. When using a Postgres
URL, set `-data-dir` so the backups directory has a home (defaults to `/data`).
The schema is created automatically on startup. The JSON export/backup format is
identical for both backends, so a SQLite backup can be restored into Postgres and
vice versa.

Example:

```
kanban-server -db "postgres://kanban:secret@localhost:5432/kanban" -data-dir /var/lib/kanban
```

**Choosing the DB on first run.** When the server starts on SQLite (the default),
the first-run setup page offers a "use external PostgreSQL" toggle with connection
fields and a "Test connection" button. If chosen, the settings are saved to
`<data-dir>/db.json` and the server restarts on PostgreSQL (you stay logged in).
At startup the DB is resolved as: explicit `-db postgres://` flag > saved
`db.json` > SQLite path. The chooser is hidden when already running on Postgres
(e.g. a compose stack that passes a URL in `-db`).

## kanban.sh Flags

For `run` and `deploy` commands:

| Flag              | Description                     | Default                         |
|-------------------|---------------------------------|---------------------------------|
| `--host <value>`  | FQDN or IP address              | `kanban.local`                  |
| `--port <port>`   | Port (nginx + container)        | `443` (TLS) / `80` (HTTP)      |
| `--tls`           | Enable TLS (HTTPS)              | enabled                         |
| `--no-tls`        | HTTP only, no TLS               | —                               |
| `--cert <path>`   | Path to TLS certificate         | `/etc/nginx/ssl/kanban.crt`    |
| `--key <path>`    | Path to TLS key                 | `/etc/nginx/ssl/kanban.key`    |

## Environment Variables

As an alternative to flags, environment variables can be used:

| Variable           | Corresponds to    |
|--------------------|-------------------|
| `KANBAN_HOST`      | `--host`          |
| `KANBAN_PORT`      | `--port`          |
| `KANBAN_TLS`       | `--tls` / `--no-tls` |
| `KANBAN_SSL_CERT`  | `--cert`          |
| `KANBAN_SSL_KEY`   | `--key`           |

Outbound mail (external sending with DKIM signing) has its own variables. The same values can be set in admin under the "Mail server" tab (admin values take precedence):

| Variable               | Description                                                  |
|------------------------|--------------------------------------------------------------|
| `KANBAN_MAIL_DOMAIN`   | Sending domain; setting it enables outbound mail             |
| `KANBAN_DKIM_SELECTOR` | DKIM selector (default `default`)                            |
| `KANBAN_DKIM_KEY`      | Path to a PEM RSA private key used for DKIM signing          |

## UI Settings

All settings live on a **single Settings page**, opened from the gear at the bottom of the left service rail (app rail). The page is available to every user; its contents depend on the role. Settings are grouped into tabs:

| Tab          | Available to                                  | Contents                                                        |
|--------------|-----------------------------------------------|-----------------------------------------------------------------|
| General      | Everyone                                       | Theme, language, font size, timezone                            |
| Mail         | When mail is enabled (except read-only users)  | Per-user mail distribution rules; mailbox/alias management (admin) |
| Integrations | Admins and users with Telegram configured      | Telegram, SSO, Calls/TURN, Git repositories, LLM               |
| Board        | Admins only                                    | Columns, epics, sprints, tags, projects                        |
| Users        | Admins only                                    | User management                                                 |
| Services     | Admins only                                    | Enable/disable and reorder services                            |
| Site         | Admins only                                    | Application name (branding), default interface language, WAP toggle |
| Mail server  | Admins only                                    | Sending domain, DKIM selector and key, DNS TXT record, audit   |
| Data         | Admins only                                    | Export/import and backups                                       |

### Theme

8 themes to choose from: dark, light, ocean, forest, nord, dracula, solarized, spacedust. The selection is saved in the browser's localStorage.

### Language

10 languages: Russian, English, Chinese, Spanish, French, German, Portuguese, Japanese, Korean, Arabic.

### Font Size

Configurable font size (default 18px).

### Timezone

The timezone is set on the "General" tab and persisted on the server:

```
POST /api/settings/timezone
{"timezone": "Europe/Moscow"}
```

### Application Name (branding)

The "Site" tab (admins only) sets the application name. Defaults to **Madok.Works**. The full name is used in the browser tab title, on the login page, in backup file names and the caption of the daily Telegram backup. The header logo shows the **initials** of the name (e.g. "Madok.Works" → "MW"), not the full string.

```
POST /api/settings/app
{"name": "Acme Hub"}
```

### Default interface language

Also on the "Site" tab (admins only): the default interface language is the language **new users start in**; each user can change their own afterwards on the "General" tab. The default is also chosen at first admin registration via the language/flag picker on the setup page (see [Getting Started](getting-started.md)). The same `POST /api/settings/app` request carries an optional `lang` field (one of the 10 supported language codes).

### WAP interface (legacy phones)

The "Site" tab has an optional **"Enable WAP interface (legacy phones)"** toggle, **off by default**. When on, a minimal WML interface is exposed at `/wap/` for very old devices. When off, all `/wap/*` routes return 404.

### Calls / TURN

On the "Integrations" tab (admins only), **Calls (WebRTC)** configures a TURN server: **TURN URL**, **TURN username** and **TURN credential**. STUN is built-in; a self-hosted TURN server (e.g. coturn) is recommended for reliable voice/video calls across NAT and firewalls. The media stays peer-to-peer.

### Git repositories

On the "Integrations" tab (admins only), **Git repositories** connects GitHub/GitLab/Gitea repositories **per project**. For each repository you set the provider (github/gitlab/gitea), the owner/name, a webhook secret, and an optional **"on merge → move to column"** action. The page shows the **webhook URL** to paste into the provider's repository settings (along with the secret). Inbound webhooks link commits, branches and merge requests to tasks by `KEY-<n>` / `#<n>` references found in branch names and commit/MR text; a merged MR can auto-move the linked task to the configured column.

### LLM (AI summaries and task generation)

On the "Integrations" tab (admins only), the **LLM** section connects an OpenAI-compatible API (e.g. llama.cpp). Fields: **API base URL** (e.g. `http://host:8080/v1`), **model**, and an optional **API key**. "Enable" tests the connection (a GET to `{base}/models`) before turning it on. Once enabled it powers the **"Summary"** button on wiki pages and **"✨ Generate"** (description + TODO) in the task form.

### Services

On the "Services" tab (admins only) every ecosystem service — including the Kanban board itself — can be enabled or disabled, and reordered with up/down buttons. The order is reflected in the service rail. A disabled service is enforced server-side too: its `/api` endpoints return 403 and its pages redirect, rather than just being hidden in the UI.

### Mail Server

The "Mail server" tab (admins only) configures the sending domain, DKIM selector and private key, and shows the public DNS TXT record to publish. These values mirror the `KANBAN_MAIL_DOMAIN` / `KANBAN_DKIM_SELECTOR` / `KANBAN_DKIM_KEY` environment variables and take precedence over them.

### Telegram Bot

Configuration on the "Integrations" tab. See [Telegram Bot](telegram.md) for details.

## Database

- SQLite with WAL mode (Write-Ahead Logging) for concurrent access
- Foreign key support
- Automatic migrations on startup
- DB path is set via the `-db` flag
