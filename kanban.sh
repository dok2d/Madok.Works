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
# nginx/systemd generation (not passed to compose):
HOST="${KANBAN_HOST:-mw.local}"
TLS="${KANBAN_TLS:-yes}"
SSL_CERT="${KANBAN_SSL_CERT:-/etc/nginx/ssl/mw.crt}"
SSL_KEY="${KANBAN_SSL_KEY:-/etc/nginx/ssl/mw.key}"
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
            --skip-drawio)  OVR[SKIP_DRAWIO]=1;   shift   ;;
            --host)         HOST="$2";            shift 2 ;;
            --tls)          TLS="yes";            shift   ;;
            --no-tls)       TLS="no";             shift   ;;
            --cert)         SSL_CERT="$2";        shift 2 ;;
            --key)          SSL_KEY="$2";         shift 2 ;;
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
        # Build args: skip the large draw.io download on flaky networks.
        echo "SKIP_DRAWIO=${SKIP_DRAWIO:-$(envget SKIP_DRAWIO 0)}"
        echo "DRAWIO_VERSION=${DRAWIO_VERSION:-$(envget DRAWIO_VERSION v30.0.4)}"
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
  passwd <user> <pass>  Сбросить пароль (создаёт админа, если юзера нет) — для
                        закрытых сетей, где Telegram-reset недоступен (напр. после restore).
  nginx        Сгенерировать nginx-конфиг (TLS на хосте → 127.0.0.1:<app-port>).
  systemd      Сгенерировать user-systemd юнит (автозапуск стека после ребута).
  deploy       systemd + nginx разом.

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
  --skip-drawio         (для build) не скачивать draw.io (~50 МБ с GitHub);
                        вики работает, редактор диаграмм — только через внешний
                        drawio_url. Альтернатива: положить vendor-cache/draw.war
  --host <FQDN>         (nginx) имя хоста / домен (по умолч. mw.local)
  --tls | --no-tls      (nginx) с TLS на 443 (по умолч.) или просто HTTP на 80
  --cert <путь>         (nginx) путь к TLS-сертификату
  --key  <путь>         (nginx) путь к TLS-ключу

Примеры:
  ./kanban.sh up --app-bind 127.0.0.1 --app-port 10010 -p s4m-works --build   # за nginx
  ./kanban.sh deploy -p s4m-works --app-port 10010 --host mw.example.com       # systemd + nginx
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

cmd_passwd() {
    local u="${ARGS[0]:-}" p="${ARGS[1]:-}"
    [[ -n "$u" && -n "$p" ]] || { echo "Использование: ./kanban.sh passwd <user> <password>" >&2; exit 1; }
    # Run the binary inside the running app container against the same DB it uses
    # (db.json/SQLite resolved identically). Recovery for closed networks where
    # the Telegram reset is unreachable — e.g. after a backup restore.
    compose exec -T app /app/kanban \
        -reset-user "$u" -reset-password "$p" \
        -db "$(envget APP_DB /data/kanban.db)" -data-dir /data
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

# gen_nginx prints an nginx server block that terminates TLS on the host and
# proxies to the app published on 127.0.0.1:<app-port>. Tuned for this app: Host
# forwarding (same-origin check), large bodies (backup import), and unbuffered
# SSE (live chat/calendar/calls signaling).
gen_nginx() {
    local port; port="$(envget APP_PORT 10010)"
    local upstream="127.0.0.1:${port}"
    if [[ "$TLS" == "yes" ]]; then
        cat <<NGINX
server {
    listen 80;
    server_name ${HOST};
    return 301 https://\$host\$request_uri;
}
server {
    listen 443 ssl http2;
    server_name ${HOST};

    ssl_certificate     ${SSL_CERT};
    ssl_certificate_key ${SSL_KEY};
    ssl_protocols       TLSv1.2 TLSv1.3;

    # Backup import uploads a multi-MB JSON; raise the body limit.
    client_max_body_size 64m;

    location / {
        proxy_pass http://${upstream};
        proxy_http_version 1.1;
        proxy_set_header Host              \$host;
        proxy_set_header X-Real-IP         \$remote_addr;
        proxy_set_header X-Forwarded-For   \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;

        # Server-Sent Events (chat, calendar, call signaling): stream, don't buffer.
        proxy_buffering    off;
        proxy_cache        off;
        proxy_read_timeout 3600s;
    }
}
NGINX
    else
        cat <<NGINX
server {
    listen 80;
    server_name ${HOST};

    client_max_body_size 64m;

    location / {
        proxy_pass http://${upstream};
        proxy_http_version 1.1;
        proxy_set_header Host              \$host;
        proxy_set_header X-Real-IP         \$remote_addr;
        proxy_set_header X-Forwarded-For   \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;

        proxy_buffering    off;
        proxy_cache        off;
        proxy_read_timeout 3600s;
    }
}
NGINX
    fi
}

cmd_nginx() {
    local proj out
    proj="$(envget COMPOSE_PROJECT_NAME madokworks)"
    out="$SCRIPT_DIR/deploy/${proj}-nginx.conf"
    gen_nginx > "$out"
    echo "==> Сгенерирован nginx-конфиг: $out"
    echo "    upstream: 127.0.0.1:$(envget APP_PORT 10010)   host: ${HOST}   tls: ${TLS}"
    echo ""
    echo "==> Установка (нужен sudo):"
    echo "    sudo cp '$out' /etc/nginx/sites-available/${proj}.conf"
    echo "    sudo ln -sf /etc/nginx/sites-available/${proj}.conf /etc/nginx/sites-enabled/${proj}.conf"
    if [[ "$TLS" == "yes" && ! -f "$SSL_CERT" ]]; then
        echo ""
        echo "==> Сертификата нет — self-signed:"
        echo "    sudo mkdir -p $(dirname "$SSL_CERT")"
        echo "    sudo openssl req -x509 -nodes -days 3650 -newkey rsa:2048 \\"
        echo "      -keyout ${SSL_KEY} -out ${SSL_CERT} -subj \"/CN=${HOST}\""
    fi
    echo ""
    echo "    sudo nginx -t && sudo systemctl reload nginx"
    echo ""
    echo "    ВАЖНО: приложение должно слушать только localhost —"
    echo "    запускай стек с  --app-bind 127.0.0.1 --app-port $(envget APP_PORT 10010)"
}

# cmd_systemd writes a user systemd unit that brings the compose stack up at
# boot via this wrapper (credentials stay in deploy/.env, not in the unit).
cmd_systemd() {
    local proj appbind appport unit_dir unit
    proj="$(envget COMPOSE_PROJECT_NAME madokworks)"
    appbind="$(envget APP_BIND 127.0.0.1)"
    appport="$(envget APP_PORT 10010)"
    unit_dir="${HOME}/.config/systemd/user"
    unit="${unit_dir}/${proj}.service"
    mkdir -p "$unit_dir"
    cat > "$unit" <<UNIT
[Unit]
Description=Madok.Works (${proj}) — podman-compose stack
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
RemainAfterExit=yes
WorkingDirectory=${SCRIPT_DIR}
ExecStart=${SCRIPT_DIR}/kanban.sh up -p ${proj} --app-bind ${appbind} --app-port ${appport}
ExecStop=${SCRIPT_DIR}/kanban.sh down -p ${proj}
TimeoutStartSec=600

[Install]
WantedBy=default.target
UNIT
    echo "==> Сгенерирован systemd-юнит: $unit"
    echo "    (порт/бинд/проект зашиты; доступы к БД берутся из deploy/.env)"
    echo ""
    echo "==> Включение и автозапуск после ребута:"
    echo "    loginctl enable-linger \$(whoami)"
    echo "    systemctl --user daemon-reload"
    echo "    systemctl --user enable --now ${proj}.service"
}

cmd_deploy() {
    cmd_systemd
    echo ""
    cmd_nginx
}

# --- main ---
CMD="${1:-}"; shift || true
parse_flags "$@"
# Config generators don't need a compose CLI; everything else does.
case "$CMD" in nginx|systemd|deploy) ;; *) detect_compose ;; esac

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
    passwd)     cmd_passwd ;;
    nginx)      cmd_nginx ;;
    systemd)    cmd_systemd ;;
    deploy)     cmd_deploy ;;
    *)          usage ;;
esac
