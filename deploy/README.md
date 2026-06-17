# LAN test stack (kanban + PostgreSQL) via podman-compose

A preconfigured two-service stack for trying the **PostgreSQL backend** in a
browser over the local network:

- **app** — the kanban server (built from the repo `Containerfile`), started with
  `-db postgres://…@db:5432/kanban` and `-data-dir /data`.
- **db** — PostgreSQL 16.

All published ports are inside the reserved **10010–10020** range and are
configurable in [`.env`](.env):

| Service | Container port | Published (LAN) | Purpose                       |
|---------|----------------|-----------------|-------------------------------|
| app     | 8080           | **10010**       | Web UI in the browser         |
| db      | 5432           | 10011           | Optional direct psql access   |

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

**First visit** prompts you to create the admin account; everything is stored in
Postgres.

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
