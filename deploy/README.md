# LAN test stack (kanban + PostgreSQL) via podman-compose

A preconfigured two-service stack for trying the **PostgreSQL backend** in a
browser over the local network:

- **app** — the kanban server (built from the repo `Containerfile`), started with
  `-db postgres://…@db:5432/kanban` and `-data-dir /data`.
- **db** — PostgreSQL 16.

All published ports are inside the reserved **10010–10020** range and are
configurable in [`.env`](.env):

| Service | Container port | Published (LAN)   | Purpose                       |
|---------|----------------|-------------------|-------------------------------|
| app     | 8080           | **10010**         | Web UI in the browser         |
| db      | 5432           | not published¹    | Postgres (internal only)      |

¹ The app reaches Postgres over the internal network (`db:5432`); the DB port is
not exposed by default. Publish it for direct `psql` access with
`../kanban.sh up --db-port 10011` (or add a `ports:` entry to the `db` service).

## Run

```sh
cd deploy
podman-compose up -d --build
```

Then, from any machine on the LAN, open:

```
http://<THIS-HOST-LAN-IP>:10010
```

Find the host's LAN IP with `ip -4 addr` / `hostname -I` (Linux) and make sure
the firewall allows inbound TCP on the published ports (e.g.
`sudo firewall-cmd --add-port=10010/tcp` or the `ufw` equivalent).

**First visit** shows the setup page. The app starts on SQLite, so the page
includes a **"Use external PostgreSQL"** toggle. Tick it and enter the bundled
db service's connection details:

| Field        | Value                          |
|--------------|--------------------------------|
| Host         | `db`                           |
| Port         | `5432`                         |
| Database     | `kanban` (PGDATABASE in .env)  |
| User         | `kanban` (PGUSER)              |
| Password     | `kanban` (PGPASSWORD)          |

Click **Test connection**, then create the admin. The choice is saved to
`/data/db.json` and the app restarts onto Postgres (you stay logged in). If you
leave the toggle off, the app simply runs on SQLite inside the `kanban-appdata`
volume and the `db` service goes unused.

> To skip the chooser and run on Postgres from the very first start, use the
> `-db` URL form in `podman-compose.yml` (commented there).

## Manage

```sh
podman-compose logs -f app     # follow app logs (migration, requests)
podman-compose ps              # status
podman-compose restart app     # restart just the app
podman-compose down            # stop, keep the data volumes
podman-compose down -v         # stop and WIPE the DB + backups volumes
```

## Data & backups

- Postgres data → named volume `kanban-pgdata`.
- App artifacts (automatic daily + manual backups) → named volume
  `kanban-appdata`, mounted at `/data` (so `-data-dir` works even though the DB
  is a URL, not a file path).

Backups are plain JSON and backend-agnostic, so a backup taken here can be
restored into a SQLite deployment and vice versa.

## Notes

- Change `PGPASSWORD` in `.env` before exposing this beyond a trusted LAN.
- To keep Postgres private to the app, delete the `db` `ports:` mapping in
  `podman-compose.yml` (the app reaches it over the internal compose network as
  host `db`).
- `docker compose` works too if you prefer it (`docker compose up -d --build`).
