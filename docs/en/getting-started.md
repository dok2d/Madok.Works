# Getting Started

## Requirements

- **Podman** or **Docker**
- For production deployment: **nginx**, **systemd**

## Build and Run

```bash
# Build the container
./kanban.sh build

# Run (defaults to http://127.0.0.1:8080)
./kanban.sh run
```

Open your browser at `http://127.0.0.1:8080`.

## First Launch

On the first visit, you will see the **administrator setup page**. Enter a username and password — this will be the main account with full access. The page also has:

- A **language/flag picker** that sets the new admin's interface language and the **default language** for everyone created afterwards (each user can change their own later).
- An optional **"use external PostgreSQL"** toggle with connection fields and a "Test connection" button. Leave it off to use the built-in SQLite database; turn it on to point the instance at an external PostgreSQL server (see [Configuration](configuration.md)).

After creating the administrator, a one-time **onboarding wizard** appears with three options:

- **Personal use** — kanban, calendar and wiki on; chat and mail off.
- **Team work** — pick which services to enable.
- **Restore from backup** — rebuild the instance from a backup file (via Import).

The wizard is followed by a short, skippable **guided tour** of the projects selector, service rail, search and settings. (Admins can replay the wizard later from Settings → Site.)

### Projects

The instance ships with a default **"General"** project (key `GEN`). Create more projects from the **project selector in the header** or under **Settings → Board → Projects**. Each project has its own board, wiki and calendars.

Once on a board, get started:

1. **Create columns** — e.g., "To Do", "In Progress", "Done"
2. **Add tasks** — click "+" in the desired column
3. **Drag and drop cards** between columns to track progress

## Running on a Different Port

```bash
./kanban.sh run --port 9090
```

## Management Commands

| Command                | Description                     |
|------------------------|---------------------------------|
| `./kanban.sh build`    | Build the container             |
| `./kanban.sh run`      | Start the container             |
| `./kanban.sh stop`     | Stop the container              |
| `./kanban.sh restart`  | Restart the container           |
| `./kanban.sh logs`     | View logs                       |
| `./kanban.sh backup`   | Back up the database            |
| `./kanban.sh restore`  | Restore from backup             |
| `./kanban.sh status`   | Container status                |
| `./kanban.sh deploy`   | Install systemd + nginx         |

## Next Steps

- [User Guide](user-guide.md) — how to work with the board
- [Deployment](deployment.md) — production setup with TLS
- [Authentication](authentication.md) — adding users and configuring SSO
