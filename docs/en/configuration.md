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
| Integrations | Admins and users with Telegram configured      | Telegram and SSO                                                |
| Board        | Admins only                                    | Columns, epics, sprints, tags                                   |
| Users        | Admins only                                    | User management                                                 |
| Services     | Admins only                                    | Enable/disable and reorder services                            |
| Site         | Admins only                                    | Application name (branding)                                     |
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

The "Site" tab (admins only) sets the application name. It changes the browser tab title, the name in the header and on the login page, and is used in backup file names and the caption of the daily Telegram backup. Defaults to "Kanban".

```
POST /api/settings/app
{"name": "Acme Hub"}
```

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
