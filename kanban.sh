#!/usr/bin/env bash
# kanban.sh — wrapper around the deploy/ podman-compose stack (Madok.Works app
# + PostgreSQL). All knobs are passed as flags (or env); the script forwards them
# to podman-compose as the variables the compose file reads.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
COMPOSE_FILE="${KANBAN_COMPOSE_FILE:-$SCRIPT_DIR/deploy/podman-compose.yml}"
ENV_FILE="$(dirname "$COMPOSE_FILE")/.env"

[[ -f "$COMPOSE_FILE" ]] || { echo "kanban.sh: compose-файл не найден: $COMPOSE_FILE" >&2; exit 1; }

# --- locate a compose CLI (podman-compose, or 'podman compose', or docker-compose) ---
detect_compose() {
    if command -v podman-compose >/dev/null 2>&1; then
        COMPOSE=(podman-compose)
    elif podman compose version >/dev/null 2>&1; then
        COMPOSE=(podman compose)
    elif command -v docker-compose >/dev/null 2>&1; then
        COMPOSE=(docker-compose)
    else
        echo "kanban.sh: не найден podman-compose (или 'podman compose')." >&2
        exit 1
    fi
}

# envget KEY DEFAULT — effective value: shell-flag override > .env > default.
envget() {
    local k="$1" d="${2:-}"
    if [[ -n "${OVR[$k]:-}" ]]; then echo "${OVR[$k]}"; return; fi
    if [[ -f "$ENV_FILE" ]]; then
        local v; v="$(grep -E "^${k}=" "$ENV_FILE" 2>/dev/null | tail -1 | cut -d= -f2-)"
        [[ -n "$v" ]] && { echo "$v"; return; }
    fi
    echo "$d"
}

# Overrides collected from flags; only these are exported (so .env still supplies
# everything the user did not explicitly set).
declare -A OVR=()
BUILD=0
NOCACHE=0
declare -a ARGS=()   # leftover positionals (logs <svc>, restore <file>, exec …)

parse_flags() {
    while [[ $# -gt 0 ]]; do
        case "$1" in
            -p|--project)   OVR[COMPOSE_PROJECT_NAME]="$2"; shift 2 ;;
            --app-port)     OVR[APP_PORT]="$2";   shift 2 ;;
            --db-port)      OVR[DB_PORT]="$2";    shift 2 ;;
            --app-bind)     OVR[APP_BIND]="$2";   shift 2 ;;
            --db-bind)      OVR[DB_BIND]="$2";    shift 2 ;;
            --pg-user)      OVR[PGUSER]="$2";     shift 2 ;;
            --pg-pass)      OVR[PGPASSWORD]="$2"; shift 2 ;;
            --pg-db)        OVR[PGDATABASE]="$2"; shift 2 ;;
            --db)           OVR[APP_DB]="$2";     shift 2 ;;   # postgres://… forces PG
            --build)        BUILD=1;              shift   ;;
            --no-cache)     NOCACHE=1;            shift   ;;
            --)             shift; ARGS+=("$@");  break   ;;
            -h|--help)      usage ;;
            # Anything else (incl. unknown -flags) is a positional passed through
            # to the underlying command — needed for `exec`/`logs`/`restart` args.
            *)              ARGS+=("$1");         shift   ;;
        esac
    done
}

# podman-compose gives the project's .env precedence over the process environment,
# so passing overrides as env vars doesn't work. Instead we materialize the full
# effective config (flag > deploy/.env > default) into a temp env file and hand it
# to compose via --env-file (which replaces the default .env). -p pins the project.
make_env_file() {
    local f; f="$(mktemp "${TMPDIR:-/tmp}/kanban-env.XXXXXX")"
    {
        echo "COMPOSE_PROJECT_NAME=$(envget COMPOSE_PROJECT_NAME madokworks)"
        echo "APP_PORT=$(envget APP_PORT 10010)"
        echo "DB_PORT=$(envget DB_PORT 10011)"
        echo "APP_BIND=$(envget APP_BIND 0.0.0.0)"
        echo "DB_BIND=$(envget DB_BIND 0.0.0.0)"
        echo "PGUSER=$(envget PGUSER kanban)"
        echo "PGPASSWORD=$(envget PGPASSWORD kanban)"
        echo "PGDATABASE=$(envget PGDATABASE kanban)"
        echo "APP_DB=$(envget APP_DB /data/kanban.db)"
    } > "$f"
    echo "$f"
}

# Postgres is internal-only by default; publish it to the host only when the user
# asked with --db-port, via a small override compose file (keeps multi-instance
# runs free of DB port clashes).
make_dbport_override() {
    local f; f="$(mktemp "${TMPDIR:-/tmp}/kanban-dbport.XXXXXX.yml")"
    cat > "$f" <<'YML'
services:
  db:
    ports:
      - "${DB_BIND:-0.0.0.0}:${DB_PORT:-10011}:5432"
YML
    echo "$f"
}

compose() {
    local ef proj ov="" rc=0
    ef="$(make_env_file)"
    proj="$(envget COMPOSE_PROJECT_NAME madokworks)"
    local -a files=(-f "$COMPOSE_FILE")
    if [[ -n "${OVR[DB_PORT]:-}" ]]; then
        ov="$(make_dbport_override)"
        files+=(-f "$ov")
    fi
    ( cd "$(dirname "$COMPOSE_FILE")" && "${COMPOSE[@]}" --env-file "$ef" -p "$proj" "${files[@]}" "$@" ) || rc=$?
    rm -f "$ef" ${ov:+"$ov"}
    return $rc
}

usage() {
    cat <<HELP
Madok.Works — обёртка над podman-compose (приложение + PostgreSQL).
Использование: ./kanban.sh <команда> [флаги]

Команды:
  up           Поднять стек (-d). С --build пересобрать образ.
  build        Собрать образ приложения (--no-cache — без кэша).
  down         Остановить (тома с данными сохраняются).
  wipe         Остановить и УДАЛИТЬ тома (БД + бэкапы). С подтверждением.
  restart [svc]  Перезапустить всё или сервис (app|db).
  logs [svc]   Логи (по умолчанию app), follow.
  ps           Статус сервисов.
  config       Показать итоговый compose-конфиг (с подставленными значениями).
  exec <svc> <cmd...>   Выполнить команду в сервисе.
  psql         Открыть psql в БД.
  backup [файл]   pg_dump БД → backups/<project>_<ts>.sql.gz (или в указанный файл).
  restore <файл>  Восстановить БД из дампа (psql). С подтверждением.

Флаги (любой команды; перекрывают deploy/.env):
  -p, --project <имя>   Имя проекта = префикс контейнеров/томов/сети (по умолч. madokworks)
  --app-port <порт>     Порт приложения (по умолч. 10010)
  --db-port  <порт>     Опубликовать PostgreSQL на хост-порту (по умолчанию БД наружу НЕ торчит)
  --app-bind <addr>     Интерфейс публикации приложения (по умолч. 0.0.0.0; 127.0.0.1 — только локально)
  --db-bind  <addr>     Интерфейс публикации БД (по умолч. 0.0.0.0)
  --pg-user <имя>       Пользователь PostgreSQL (по умолч. kanban)
  --pg-pass <пароль>    Пароль PostgreSQL (по умолч. kanban)
  --pg-db   <имя>       База PostgreSQL (по умолч. kanban)
  --db <dsn|путь>       Значение -db приложения: postgres://… форсит Postgres
                        (по умолч. SQLite-файл → мастер выбора БД при первом запуске)
  --build               (для up) пересобрать образ перед запуском
  --no-cache            (для build) собрать без кэша

Примеры:
  ./kanban.sh up --build
  ./kanban.sh up --app-port 8080 --db-port 5433
  ./kanban.sh up -p work --app-port 10020 --pg-pass s3cret
  ./kanban.sh up --app-bind 127.0.0.1            # только локально, за reverse-proxy
  ./kanban.sh up --db "postgres://kanban:kanban@db:5432/kanban?sslmode=disable"
  ./kanban.sh logs db
  ./kanban.sh backup && ./kanban.sh restore backups/madokworks_*.sql.gz
  ./kanban.sh wipe

Примечание: бэкап через pg_dump относится к PostgreSQL-бэкенду. Если на первом
запуске оставить SQLite (не выбирать внешний PostgreSQL), портируемый бэкап — это
JSON-экспорт в UI (Настройки → Данные), он же вход для cmd/backupconv.
HELP
    exit 1
}

show_url() {
    local port; port="$(envget APP_PORT 10010)"
    local bind; bind="$(envget APP_BIND 0.0.0.0)"
    local host="$bind"; [[ "$bind" == "0.0.0.0" ]] && host="<LAN-IP>"
    echo "==> Приложение: http://${host}:${port}  (первый визит — создание администратора)"
}

cmd_up() {
    local -a a=(up -d)
    [[ "$BUILD" == 1 ]] && a+=(--build)
    compose "${a[@]}"
    show_url
}

cmd_build() {
    local -a a=(build)
    [[ "$NOCACHE" == 1 ]] && a+=(--no-cache)
    compose "${a[@]}"
}

cmd_wipe() {
    echo "==> ВНИМАНИЕ: будут удалены тома (БД + бэкапы) проекта '$(envget COMPOSE_PROJECT_NAME madokworks)'."
    read -rp "    Продолжить? (yes/no): " c
    [[ "$c" == "yes" ]] || { echo "==> Отменено."; exit 1; }
    compose down -v
}

cmd_backup() {
    local ts dir out
    ts="$(date +%Y%m%d_%H%M%S)"
    dir="${BACKUP_DIR:-$SCRIPT_DIR/backups}"; mkdir -p "$dir"
    out="${ARGS[0]:-$dir/$(envget COMPOSE_PROJECT_NAME madokworks)_${ts}.sql.gz}"
    local u p d
    u="$(envget PGUSER kanban)"; p="$(envget PGPASSWORD kanban)"; d="$(envget PGDATABASE kanban)"
    echo "==> pg_dump ($d) → $out"
    compose exec -T db sh -c "PGPASSWORD='$p' pg_dump -U '$u' -d '$d' --clean --if-exists" | gzip > "$out"
    echo "==> Готово: $out  ($(du -h "$out" | cut -f1))"
}

cmd_restore() {
    local f="${ARGS[0]:-}"
    [[ -n "$f" ]] || { echo "Использование: ./kanban.sh restore <файл.sql[.gz]>" >&2; exit 1; }
    [[ -f "$f" ]] || { echo "==> Файл не найден: $f" >&2; exit 1; }
    echo "==> ВНИМАНИЕ: данные БД будут заменены содержимым '$f'."
    read -rp "    Продолжить? (yes/no): " c
    [[ "$c" == "yes" ]] || { echo "==> Отменено."; exit 1; }
    local u p d cat=cat
    u="$(envget PGUSER kanban)"; p="$(envget PGPASSWORD kanban)"; d="$(envget PGDATABASE kanban)"
    [[ "$f" == *.gz ]] && cat=zcat
    $cat "$f" | compose exec -T db sh -c "PGPASSWORD='$p' psql -v ON_ERROR_STOP=1 -U '$u' -d '$d'"
    echo "==> Готово: данные восстановлены."
}

# --- main ---
CMD="${1:-}"; shift || true
parse_flags "$@"
detect_compose

case "$CMD" in
    up|start)   cmd_up ;;
    build)      cmd_build ;;
    down|stop)  compose down ;;
    wipe)       cmd_wipe ;;
    restart)    compose restart "${ARGS[@]}" ;;
    logs)       compose logs -f "${ARGS[@]:-app}" ;;
    ps|status)  compose ps ;;
    config)     compose config ;;
    exec)       compose exec "${ARGS[@]}" ;;
    psql)       compose exec db psql -U "$(envget PGUSER kanban)" -d "$(envget PGDATABASE kanban)" ;;
    backup)     cmd_backup ;;
    restore)    cmd_restore ;;
    *)          usage ;;
esac
