#!/usr/bin/env bash
# GoWave installer. Run it again on the same server to update, change the
# domain or remove GoWave.
#
#   sudo bash -c "$(curl -fsSL https://raw.githubusercontent.com/aaaSaZaN/gowave/main/install.sh)"
#
# Installs the panel (SQLite with systemd, or PostgreSQL in Docker) behind
# Caddy or Nginx with HTTPS, or leaves the reverse proxy to the admin. The
# panel listens on 127.0.0.1 only. When a Remnawave or PasarGuard install is
# found, offers to move its data. Asks for the language (Russian or English)
# first.
# GOWAVE_VERSION=v1.2.3 pins a release instead of the latest one.
set -Eeuo pipefail

REPO=aaaSaZaN/gowave
GOWAVE_VERSION=${GOWAVE_VERSION:-latest}
INSTALL_DIR=/opt/gowave
DOCKER_DIR=/opt/gowave-docker
UNIT_FILE=/etc/systemd/system/gowave.service
CLI_PATH=/usr/local/bin/gowave
CADDYFILE=/etc/caddy/Caddyfile
CADDY_SITE=/etc/caddy/gowave.caddy
NGINX_SITE=/etc/nginx/conf.d/gowave.conf
NGINX_RENEW_HOOK=/etc/letsencrypt/renewal-hooks/deploy/gowave-nginx.sh
ACME_ROOT=/var/www/gowave-acme
REMNAWAVE_DIR=/opt/remnawave
PASARGUARD_DIR=/opt/pasarguard
LOG=/var/log/gowave-install.log

if [ -t 1 ]; then
    RED=$'\e[31m' GREEN=$'\e[32m' YELLOW=$'\e[33m' BOLD=$'\e[1m' RESET=$'\e[0m'
else
    RED='' GREEN='' YELLOW='' BOLD='' RESET=''
fi

info() { printf '%s\n' "${GREEN}==>${RESET} $*"; }
warn() { printf '%s\n' "${YELLOW}[!] $*${RESET}" >&2; }
die() {
    printf '%s\n' "${RED}[x] $*${RESET}" >&2
    exit 1
}
have() { command -v "$1" >/dev/null 2>&1; }

UI_LANG=ru

# t prints its first argument (Russian) or its second (English) depending on
# the language picked at startup.
t() {
    if [ "$UI_LANG" = en ]; then printf '%s' "$2"; else printf '%s' "$1"; fi
}

# run hides command output in $LOG and shows its tail on failure.
run() {
    printf '\n$ %s\n' "$*" >>"$LOG"
    if ! "$@" </dev/null >>"$LOG" 2>&1; then
        tail -n 25 "$LOG" >&2
        die "$(t "Команда завершилась с ошибкой: $*  (полный лог: $LOG)" "Command failed: $*  (full log: $LOG)")"
    fi
}

# Prompts read the terminal, so the script also works when piped into bash.
ask() {
    local reply=''
    printf '%s' "$1" >&2
    IFS= read -r reply <&3 || true
    printf '%s' "$reply"
}

confirm() {
    case $(ask "$1 [y/N]: ") in
    [yYдД]*) return 0 ;;
    *) return 1 ;;
    esac
}

# choose prints the 1-based number of the picked option.
choose() {
    local title=$1 reply i
    shift
    while :; do
        printf '\n%s\n' "${BOLD}${title}${RESET}" >&2
        i=1
        for option in "$@"; do
            printf '  %d) %s\n' "$i" "$option" >&2
            i=$((i + 1))
        done
        reply=$(ask "$(t 'Выбор' 'Choice') [1]: ")
        reply=${reply:-1}
        if [[ $reply =~ ^[0-9]+$ ]] && [ "$reply" -ge 1 ] && [ "$reply" -le $# ]; then
            printf '%s' "$reply"
            return
        fi
        warn "$(t "Введите число от 1 до $#" "Enter a number from 1 to $#")"
    done
}

# ask_language runs before anything else is printed, so its own prompt is in
# both languages.
ask_language() {
    local reply
    while :; do
        printf '\n%s\n  1) Русский\n  2) English\n' "${BOLD}Язык / Language:${RESET}" >&2
        reply=$(ask 'Выбор / Choice [1]: ')
        case ${reply:-1} in
        1) UI_LANG=ru && return ;;
        2) UI_LANG=en && return ;;
        esac
        warn 'Введите 1 или 2 / Enter 1 or 2'
    done
}

# ---------------------------------------------------------------- system

PKG=''
ARCH=''
APT_UPDATED=false

detect_system() {
    # The language is not known yet: these two errors come in both.
    [ "$(id -u)" -eq 0 ] || die 'Запустите установщик от root (sudo). / Run the installer as root (sudo).'
    (: </dev/tty) 2>/dev/null ||
        die 'Нужен интерактивный терминал: запустите команду из README в SSH-сессии. / An interactive terminal is required: run the command from the README in an SSH session.'
    exec 3</dev/tty
    ask_language
    [ -d /run/systemd/system ] || die "$(t 'Нужен systemd.' 'systemd is required.')"
    if have apt-get; then
        PKG=apt
    elif have dnf; then
        PKG=dnf
    else
        die "$(t 'Поддерживаются Debian/Ubuntu (apt) и RHEL/Alma/Rocky/Fedora (dnf).' 'Supported systems: Debian/Ubuntu (apt) and RHEL/Alma/Rocky/Fedora (dnf).')"
    fi
    case $(uname -m) in
    x86_64 | amd64) ARCH=amd64 ;;
    aarch64 | arm64) ARCH=arm64 ;;
    *) die "$(t "Архитектура $(uname -m) не поддерживается: релизы GoWave собираются для amd64 и arm64." "Architecture $(uname -m) is not supported: GoWave releases are built for amd64 and arm64.")" ;;
    esac
    : >>"$LOG"
    chmod 600 "$LOG"
}

pkg_install() {
    if [ "$PKG" = apt ]; then
        if [ "$APT_UPDATED" = false ]; then
            run apt-get update
            APT_UPDATED=true
        fi
        run env DEBIAN_FRONTEND=noninteractive apt-get install -y "$@"
    else
        run dnf install -y "$@"
    fi
}

ensure_tools() {
    local missing=()
    have curl || missing+=(curl)
    have tar || missing+=(tar)
    have openssl || missing+=(openssl)
    have ss || missing+=(iproute2)
    have sha256sum || missing+=(coreutils)
    if [ ${#missing[@]} -gt 0 ]; then
        [ "$PKG" = dnf ] && missing=("${missing[@]/iproute2/iproute}")
        info "$(t 'Устанавливаю' 'Installing') ${missing[*]}"
        pkg_install "${missing[@]}"
    fi
}

ensure_docker() {
    if have docker && docker compose version >/dev/null 2>&1; then
        return
    fi
    info "$(t 'Устанавливаю Docker (get.docker.com)' 'Installing Docker (get.docker.com)')"
    curl -fsSL https://get.docker.com -o "$WORK/get-docker.sh" || die "$(t 'Не удалось скачать установщик Docker.' 'Could not download the Docker installer.')"
    run sh "$WORK/get-docker.sh"
    run systemctl enable --now docker
}

port_busy() { [ -n "$(ss -Hltn "sport = :$1" 2>/dev/null)" ]; }

free_port() {
    local port=$1
    while port_busy "$port" || [ "$port" = "${2:-}" ]; do
        port=$((port + 1))
    done
    printf '%s' "$port"
}

env_get() {
    [ -f "$1" ] || return 0
    sed -n "s/^$2=//p" "$1" | tail -n 1 | sed -e "s/^['\"]//" -e "s/['\"]\$//"
}

env_set() {
    if grep -q "^$2=" "$1"; then
        sed -i "s|^$2=.*|$2=$3|" "$1"
    else
        printf '%s=%s\n' "$2" "$3" >>"$1"
    fi
}

wait_healthy() {
    local i
    for i in $(seq 1 60); do
        curl -fsS --max-time 3 "http://127.0.0.1:$1/api/system/health" >/dev/null 2>&1 && return 0
        sleep 2
    done
    return 1
}

# ---------------------------------------------------------------- domain

DOMAIN=''

valid_domain() {
    [[ $1 =~ ^([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]([a-z0-9-]{0,61}[a-z0-9])?$ ]]
}

server_ips() {
    local version ip
    for version in 4 6; do
        ip=$(curl -"$version" -fsS --max-time 5 https://cloudflare.com/cdn-cgi/trace 2>/dev/null | sed -n 's/^ip=//p') || true
        [ -n "$ip" ] && printf '%s\n' "$ip"
    done
    return 0
}

# check_dns refuses a domain that does not resolve and asks before accepting
# one that points elsewhere (for example through the Cloudflare proxy).
check_dns() {
    local records ips
    records=$(getent ahosts "$1" 2>/dev/null | awk '{print $1}' | sort -u) || true
    if [ -z "$records" ]; then
        warn "$(t "Домен $1 не резолвится. Создайте A-запись (и AAAA, если есть IPv6) на IP этого сервера и подождите обновления DNS." "Domain $1 does not resolve. Create an A record (and AAAA if the server has IPv6) pointing to this server's IP and wait for DNS to update.")"
        return 1
    fi
    ips=$(server_ips)
    if [ -z "$ips" ]; then
        warn "$(t 'Не удалось определить внешний IP сервера, пропускаю проверку DNS.' "Could not detect the server's public IP, skipping the DNS check.")"
        return 0
    fi
    if grep -qxF -f <(printf '%s\n' "$ips") <(printf '%s\n' "$records"); then
        return 0
    fi
    records=$(printf '%s' "$records" | tr '\n' ' ')
    ips=$(printf '%s' "$ips" | tr '\n' ' ')
    warn "$(t "Домен $1 указывает на $records, а у сервера $ips" "Domain $1 points to $records, but this server has $ips")"
    warn "$(t 'Сертификат не выпустится, пока домен не указывает на этот сервер.' 'The certificate will not be issued until the domain points to this server.')"
    confirm "$(t 'Продолжить всё равно (например, домен за Cloudflare-прокси)?' 'Continue anyway (for example, the domain is behind the Cloudflare proxy)?')"
}

ask_domain() {
    local domain
    while :; do
        domain=$(ask "$(t 'Домен панели (например panel.example.com): ' 'Panel domain (for example panel.example.com): ')")
        domain=$(printf '%s' "$domain" | tr '[:upper:]' '[:lower:]' | sed -e 's#^[a-z]*://##' -e 's#[/:].*$##' -e 's/\.$//')
        if ! valid_domain "$domain"; then
            warn "$(t 'Нужен домен вида panel.example.com. Панель без домена и HTTPS не устанавливается.' 'A domain like panel.example.com is required. The panel is not installed without a domain and HTTPS.')"
            continue
        fi
        # An external proxy may live on another host or behind a tunnel, so
        # the domain does not have to point here.
        [ "$PROXY" = external ] && break
        check_dns "$domain" && break
    done
    DOMAIN=$domain
}

# ---------------------------------------------------------------- old panels

HAS_REMNAWAVE=false
HAS_PASARGUARD=false
SOURCE=none
SOURCE_DIR=''

detect_old_panels() {
    [ -f "$REMNAWAVE_DIR/.env" ] && HAS_REMNAWAVE=true
    if [ -f "$PASARGUARD_DIR/.env" ] || [ -f /var/lib/pasarguard/db.sqlite3 ]; then
        if grep -Eqs '^[[:space:]]*SQLALCHEMY_DATABASE_URL[[:space:]]*=[[:space:]]*["'\'']?(mysql|mariadb)' "$PASARGUARD_DIR/.env"; then
            warn "$(t 'Найден PasarGuard на MySQL/MariaDB: перенос поддерживается только из SQLite и PostgreSQL.' 'Found PasarGuard on MySQL/MariaDB: migration is supported only from SQLite and PostgreSQL.')"
        else
            HAS_PASARGUARD=true
        fi
    fi
    return 0
}

under_dir() {
    case $1 in "$2" | "$2"/*) return 0 ;; esac
    return 1
}

# old_containers lists containers of compose projects under a directory,
# optionally only one compose service.
old_containers() {
    have docker || return 0
    local id labels dir service
    for id in $(docker ps -aq 2>/dev/null); do
        labels=$(docker inspect -f '{{index .Config.Labels "com.docker.compose.project.working_dir"}}|{{index .Config.Labels "com.docker.compose.service"}}' "$id" 2>/dev/null) || continue
        dir=${labels%%|*}
        service=${labels#*|}
        under_dir "$dir" "$1" || continue
        [ -n "${2:-}" ] && [ "$service" != "$2" ] && continue
        printf '%s\n' "$id"
    done
}

container_names() {
    local id
    for id in "$@"; do docker inspect -f '{{.Name}}' "$id" | sed 's#^/##'; done | tr '\n' ' '
}

# stop_old_panel stops every container of the old panel and keeps it from
# starting on boot. Nothing is deleted.
stop_old_panel() {
    local ids
    ids=$(old_containers "$SOURCE_DIR")
    [ -n "$ids" ] || return 0
    # shellcheck disable=SC2086
    info "$(t 'Останавливаю старую панель:' 'Stopping the old panel:') $(container_names $ids)"
    # shellcheck disable=SC2086
    run docker update --restart=no $ids
    # shellcheck disable=SC2086
    run docker stop $ids
}

# ---------------------------------------------------------------- ports

PROXY=''
APP_PORT=''

# check_web_ports makes sure 80 and 443 are free, held by the chosen proxy
# (installed on the host), or held by the old panel that is about to stop.
check_web_ports() {
    local allow_old=$1 port line name pid ids id dir problems=()
    for port in 80 443; do
        line=$(ss -Hltnp "sport = :$port" 2>/dev/null | head -n 1)
        [ -n "$line" ] || continue
        name=$(printf '%s' "$line" | grep -o 'users:(("[^"]*"' | cut -d'"' -f2)
        pid=$(printf '%s' "$line" | grep -o 'pid=[0-9]*' | head -n 1 | cut -d= -f2)
        ids=''
        if [ "$name" = docker-proxy ]; then
            ids=$(docker ps -q --filter "publish=$port" 2>/dev/null)
        elif [ -n "$pid" ]; then
            ids=$(grep -oE '[0-9a-f]{64}' "/proc/$pid/cgroup" 2>/dev/null | head -n 1)
        fi
        if [ -n "$ids" ]; then
            for id in $ids; do
                dir=$(docker inspect -f '{{index .Config.Labels "com.docker.compose.project.working_dir"}}' "$id" 2>/dev/null)
                if [ "$allow_old" = true ] && [ -n "$SOURCE_DIR" ] && under_dir "$dir" "$SOURCE_DIR"; then
                    continue
                fi
                problems+=("$(t "порт $port занят Docker-контейнером" "port $port is held by Docker container") $(container_names "$id")${dir:+ ($dir)}")
            done
        elif [ "$name" = "$PROXY" ] && systemctl is-active --quiet "$PROXY"; then
            continue
        else
            problems+=("$(t "порт $port занят процессом ${name:-неизвестно}" "port $port is held by process ${name:-unknown}")")
        fi
    done
    [ ${#problems[@]} -eq 0 ] && return 0
    printf '  %s\n' "${problems[@]}" >&2
    return 1
}

# ---------------------------------------------------------------- GoWave files

release_url() {
    if [ "$GOWAVE_VERSION" = latest ]; then
        printf 'https://github.com/%s/releases/latest/download/%s' "$REPO" "$1"
    else
        printf 'https://github.com/%s/releases/download/%s/%s' "$REPO" "$GOWAVE_VERSION" "$1"
    fi
}

# install_release downloads the binary and the web interface, checks them
# against SHA256SUMS and swaps them into $INSTALL_DIR.
install_release() {
    local dir="$WORK/release" file
    mkdir -p "$dir"
    info "$(t 'Скачиваю' 'Downloading') GoWave ($GOWAVE_VERSION, linux-$ARCH)"
    for file in "gowave-linux-$ARCH" gowave-frontend.tar.gz SHA256SUMS; do
        curl -fsSL --retry 3 -o "$dir/$file" "$(release_url "$file")" || die "$(t 'Не удалось скачать' 'Could not download') $file"
    done
    grep -E " \*?(gowave-linux-$ARCH|gowave-frontend\.tar\.gz)\$" "$dir/SHA256SUMS" >"$dir/expected" || true
    [ "$(wc -l <"$dir/expected")" -eq 2 ] || die "$(t 'В SHA256SUMS нет нужных файлов.' 'SHA256SUMS does not list the required files.')"
    (cd "$dir" && sha256sum -c --quiet expected) || die "$(t 'Контрольные суммы не совпали, установка прервана.' 'Checksums do not match, installation aborted.')"

    mkdir -p "$INSTALL_DIR/frontend"
    install -m 0755 "$dir/gowave-linux-$ARCH" "$INSTALL_DIR/gowave-server.new"
    mv -f "$INSTALL_DIR/gowave-server.new" "$INSTALL_DIR/gowave-server"
    rm -rf "$INSTALL_DIR/frontend/dist.new" "$INSTALL_DIR/frontend/dist.old"
    mkdir -p "$INSTALL_DIR/frontend/dist.new"
    tar -xzf "$dir/gowave-frontend.tar.gz" -C "$INSTALL_DIR/frontend/dist.new"
    [ -d "$INSTALL_DIR/frontend/dist" ] && mv "$INSTALL_DIR/frontend/dist" "$INSTALL_DIR/frontend/dist.old"
    mv "$INSTALL_DIR/frontend/dist.new" "$INSTALL_DIR/frontend/dist"
    rm -rf "$INSTALL_DIR/frontend/dist.old"
}

write_env() {
    local metrics_port
    metrics_port=$(free_port 3001 "$APP_PORT")
    (
        umask 077
        cat >"$INSTALL_DIR/.env" <<EOF
# Generated by install.sh. All settings: https://aaasazan.github.io/gowave-docs/reference/environment/
APP_HOST=127.0.0.1
APP_PORT=$APP_PORT
METRICS_PORT=$metrics_port
APP_SECRET=$(openssl rand -hex 32)
DB_DRIVER=sqlite
DATABASE_URL=gowave.db
PANEL_DOMAIN=$DOMAIN
SUB_PUBLIC_DOMAIN=$DOMAIN/api/sub
GOWAVE_PROXY=$PROXY
EOF
    )
}

write_unit() {
    cat >"$UNIT_FILE" <<EOF
[Unit]
Description=GoWave Panel
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=root
WorkingDirectory=$INSTALL_DIR
ExecStart=$INSTALL_DIR/gowave-server
Restart=always
RestartSec=5
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF
    run systemctl daemon-reload
}

start_native() {
    run systemctl enable gowave
    run systemctl restart gowave
    wait_healthy "$APP_PORT" || {
        journalctl -u gowave -n 30 --no-pager >&2 || true
        die "$(t 'Панель не запустилась (journalctl -u gowave).' 'The panel did not start (journalctl -u gowave).')"
    }
}

gowave_cli() {
    (cd "$INSTALL_DIR" && ./gowave-server "$@") </dev/null 2>&1 | tee -a "$LOG"
    return "${PIPESTATUS[0]}"
}

# migrate_data copies the old panel into $INSTALL_DIR/gowave.db. The old
# panel's app container is stopped first for a consistent copy and started
# again if the copy fails; its database keeps running.
migrate_data() {
    local app_service app_ids db_ids status=0
    if [ "$SOURCE" = remnawave ]; then
        app_service=remnawave
        db_ids=$(old_containers "$REMNAWAVE_DIR" remnawave-db)
        # shellcheck disable=SC2086
        [ -n "$db_ids" ] && run docker start $db_ids
    else
        app_service=pasarguard
    fi
    app_ids=$(old_containers "$SOURCE_DIR" "$app_service")
    # shellcheck disable=SC2086
    [ -n "$app_ids" ] && run docker stop $app_ids

    info "$(t 'Переношу данные из' 'Migrating data from') $SOURCE"
    if [ "$SOURCE" = remnawave ]; then
        sleep 3
        gowave_cli migrate pg-to-sqlite "" "$INSTALL_DIR/gowave.db" || status=$?
    else
        gowave_cli import pasarguard || status=$?
    fi
    if [ "$status" -ne 0 ]; then
        rm -f "$INSTALL_DIR"/gowave.db*
        # shellcheck disable=SC2086
        [ -n "$app_ids" ] && docker start $app_ids >/dev/null 2>&1
        die "$(t "Перенос не удался, старая панель запущена обратно. Подробности выше и в $LOG" "Migration failed, the old panel has been started again. Details above and in $LOG")"
    fi
}

move_to_docker() {
    ensure_docker
    info "$(t 'Перевожу панель в Docker с PostgreSQL' 'Moving the panel to Docker with PostgreSQL')"
    gowave_cli migrate to-docker --yes --from "$INSTALL_DIR" ||
        die "$(t 'Переход в Docker не удался, панель продолжает работать на SQLite.' 'Moving to Docker failed, the panel keeps running on SQLite.')"
}

# ---------------------------------------------------------------- Caddy

install_caddy() {
    have caddy && return
    info "$(t 'Устанавливаю Caddy' 'Installing Caddy')"
    if [ "$PKG" = apt ]; then
        pkg_install debian-keyring debian-archive-keyring apt-transport-https gnupg
        curl -fsSL https://dl.cloudsmith.io/public/caddy/stable/gpg.key | gpg --dearmor --yes -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg ||
            die "$(t 'Не удалось добавить ключ репозитория Caddy.' 'Could not add the Caddy repository key.')"
        curl -fsSL https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt -o /etc/apt/sources.list.d/caddy-stable.list ||
            die "$(t 'Не удалось добавить репозиторий Caddy.' 'Could not add the Caddy repository.')"
        chmod o+r /usr/share/keyrings/caddy-stable-archive-keyring.gpg /etc/apt/sources.list.d/caddy-stable.list
        APT_UPDATED=false
        pkg_install caddy
    else
        pkg_install 'dnf-command(copr)'
        run dnf copr enable -y @caddy/caddy
        pkg_install caddy
    fi
}

# The distribution's Caddyfile only serves a placeholder page on :80.
is_stock_caddyfile() {
    [ "$(grep -v '^[[:space:]]*#' "$CADDYFILE" | tr -d ' \t\n')" = ':80{root*/usr/share/caddyfile_server}' ]
}

configure_caddy() {
    local backup="$WORK/Caddyfile.bak"
    install_caddy
    cat >"$CADDY_SITE" <<EOF
# Managed by GoWave install.sh
$DOMAIN {
	encode zstd gzip
	reverse_proxy 127.0.0.1:$APP_PORT
}
EOF
    [ -f "$CADDYFILE" ] && cp -p "$CADDYFILE" "$backup"
    if [ ! -s "$CADDYFILE" ] || grep -q '^# Managed by GoWave' "$CADDYFILE" || is_stock_caddyfile; then
        printf '# Managed by GoWave install.sh\nimport %s\n' "$CADDY_SITE" >"$CADDYFILE"
    elif ! grep -qF "import $CADDY_SITE" "$CADDYFILE"; then
        printf '\nimport %s\n' "$CADDY_SITE" >>"$CADDYFILE"
    fi
    if ! caddy validate --config "$CADDYFILE" --adapter caddyfile >>"$LOG" 2>&1; then
        if [ -f "$backup" ]; then cp -p "$backup" "$CADDYFILE"; fi
        rm -f "$CADDY_SITE"
        tail -n 15 "$LOG" >&2
        die "$(t "Caddy не принял конфигурацию (возможно, $DOMAIN уже описан в $CADDYFILE). Исходный Caddyfile восстановлен." "Caddy rejected the configuration ($DOMAIN may already be defined in $CADDYFILE). The original Caddyfile has been restored.")"
    fi
    run systemctl enable caddy
    run systemctl reload-or-restart caddy
}

remove_caddy_site() {
    [ -f "$CADDY_SITE" ] || return 0
    rm -f "$CADDY_SITE"
    if grep -q '^# Managed by GoWave' "$CADDYFILE" 2>/dev/null; then
        # Caddy was set up only for GoWave.
        systemctl disable --now caddy >/dev/null 2>&1 || true
        printf ':80 {\n\troot * /usr/share/caddy\n\tfile_server\n}\n' >"$CADDYFILE"
    else
        sed -i "\#^import $CADDY_SITE\$#d" "$CADDYFILE"
        systemctl reload caddy >/dev/null 2>&1 || true
    fi
}

# ---------------------------------------------------------------- Nginx

install_nginx() {
    if ! have nginx || ! have certbot; then
        info "$(t 'Устанавливаю Nginx и Certbot' 'Installing Nginx and Certbot')"
        if [ "$PKG" = dnf ] && ! grep -qi fedora /etc/os-release; then
            pkg_install epel-release
        fi
        pkg_install nginx certbot
    fi
    if have getenforce && [ "$(getenforce)" = Enforcing ]; then
        run setsebool -P httpd_can_network_connect 1
    fi
}

version_ge() { [ "$(printf '%s\n%s\n' "$2" "$1" | sort -V | head -n 1)" = "$2" ]; }

nginx_http_block() {
    cat <<EOF
server {
    listen 80;
$( [ -f /proc/net/if_inet6 ] && printf '    listen [::]:80;\n')
    server_name $DOMAIN;

    location /.well-known/acme-challenge/ {
        root $ACME_ROOT;
    }
    location / {
        return 301 https://\$host\$request_uri;
    }
}
EOF
}

write_nginx_full() {
    local version listen6='' http2 listen_opts='' reject=''
    version=$(nginx -v 2>&1 | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -n 1)
    [ -f /proc/net/if_inet6 ] && listen6=yes
    if version_ge "$version" 1.25.1; then
        http2='    http2 on;'
    else
        http2=''
        listen_opts=' http2'
    fi
    if version_ge "$version" 1.19.4 && ! grep -rqsE --exclude="$(basename "$NGINX_SITE")" 'listen[^;]*443[^;]*default_server' /etc/nginx; then
        reject=$(
            cat <<EOF

# Handshakes for any other name (scans by IP) are refused.
server {
    listen 443 ssl default_server;
${listen6:+    listen [::]:443 ssl default_server;}
    server_name _;
    ssl_reject_handshake on;
}
EOF
        )
    fi
    cat >"$NGINX_SITE" <<EOF
# Managed by GoWave install.sh
map \$http_upgrade \$gowave_connection_upgrade {
    default upgrade;
    ""      close;
}

$(nginx_http_block)

server {
    listen 443 ssl$listen_opts;
${listen6:+    listen [::]:443 ssl$listen_opts;}
$http2
    server_name $DOMAIN;

    ssl_certificate     /etc/letsencrypt/live/$DOMAIN/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/$DOMAIN/privkey.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256:ECDHE-ECDSA-AES256-GCM-SHA384:ECDHE-RSA-AES256-GCM-SHA384:ECDHE-ECDSA-CHACHA20-POLY1305:ECDHE-RSA-CHACHA20-POLY1305;
    ssl_session_timeout 1d;
    ssl_session_cache shared:GoWaveSSL:10m;
    ssl_session_tickets off;

    client_max_body_size 50M;

    gzip on;
    gzip_vary on;
    gzip_proxied any;
    gzip_comp_level 6;
    gzip_min_length 256;
    gzip_types application/javascript application/json application/manifest+json application/wasm application/xml image/svg+xml text/css text/javascript text/plain text/xml;

    location / {
        proxy_pass http://127.0.0.1:$APP_PORT;
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection \$gowave_connection_upgrade;
        # The node SSH console is a long-lived WebSocket.
        proxy_read_timeout 1h;
        proxy_send_timeout 1h;
    }
}
$reject
EOF
}

nginx_apply() {
    if ! nginx -t >>"$LOG" 2>&1; then
        tail -n 15 "$LOG" >&2
        rm -f "$NGINX_SITE"
        die "$(t "Nginx не принял конфигурацию, $NGINX_SITE удалён." "Nginx rejected the configuration, $NGINX_SITE has been removed.")"
    fi
    run systemctl enable nginx
    run systemctl reload-or-restart nginx
}

configure_nginx() {
    install_nginx
    local taken
    taken=$(grep -rlsE --exclude="$(basename "$NGINX_SITE")" "server_name[^;]*[[:space:]]${DOMAIN//./\\.}[[:space:];]" /etc/nginx || true)
    [ -z "$taken" ] || die "$(t "Домен $DOMAIN уже описан в $taken. Уберите этот server-блок и запустите установщик снова." "Domain $DOMAIN is already defined in $taken. Remove that server block and run the installer again.")"
    mkdir -p "$ACME_ROOT"
    {
        echo '# Managed by GoWave install.sh (certificate request)'
        nginx_http_block
    } >"$NGINX_SITE"
    nginx_apply
    info "$(t "Выпускаю сертификат Let's Encrypt для $DOMAIN" "Requesting a Let's Encrypt certificate for $DOMAIN")"
    run certbot certonly --webroot -w "$ACME_ROOT" -d "$DOMAIN" --cert-name "$DOMAIN" \
        --non-interactive --agree-tos --register-unsafely-without-email --keep-until-expiring
    mkdir -p "$(dirname "$NGINX_RENEW_HOOK")"
    printf '#!/bin/sh\nsystemctl reload nginx\n' >"$NGINX_RENEW_HOOK"
    chmod 755 "$NGINX_RENEW_HOOK"
    write_nginx_full
    nginx_apply
}

remove_nginx_site() {
    [ -f "$NGINX_SITE" ] || return 0
    rm -f "$NGINX_SITE" "$NGINX_RENEW_HOOK"
    if nginx -t >/dev/null 2>&1; then systemctl reload nginx >/dev/null 2>&1 || true; fi
}

# ---------------------------------------------------------------- proxy

ask_proxy() {
    case $(choose "$(t 'Reverse proxy с HTTPS:' 'Reverse proxy with HTTPS:')" \
        "$(t 'Caddy (сертификат выпускается и продлевается сам)' 'Caddy (issues and renews the certificate itself)')" \
        'Nginx + Certbot' \
        "$(t 'Не настраивать: свой reverse proxy' 'Skip: I will use my own reverse proxy')") in
    1) PROXY=caddy ;;
    2) PROXY=nginx ;;
    3) PROXY=external ;;
    esac
}

open_firewall() {
    if have ufw && ufw status 2>/dev/null | grep -q 'Status: active'; then
        run ufw allow 80/tcp
        run ufw allow 443/tcp
    fi
    if have firewall-cmd && firewall-cmd --state >/dev/null 2>&1; then
        run firewall-cmd --permanent --add-service=http --add-service=https
        run firewall-cmd --reload
    fi
}

setup_proxy() {
    if [ "$PROXY" = external ]; then
        remove_caddy_site
        remove_nginx_site
        return 0
    fi
    if [ "$PROXY" = caddy ]; then
        remove_nginx_site
        configure_caddy
    else
        remove_caddy_site
        configure_nginx
    fi
    open_firewall
    info "$(t 'Проверяю' 'Checking') https://$DOMAIN"
    local i
    for i in $(seq 1 30); do
        if curl -fsS --max-time 5 "https://$DOMAIN/api/system/health" >/dev/null 2>&1; then
            return 0
        fi
        sleep 2
    done
    warn "$(t "https://$DOMAIN пока не отвечает. Проверьте DNS и журнал: journalctl -u $PROXY -n 50" "https://$DOMAIN is not responding yet. Check DNS and the log: journalctl -u $PROXY -n 50")"
}

# ---------------------------------------------------------------- install

choose_database() {
    case $(choose "$(t 'База данных:' 'Database:')" \
        "$(t 'SQLite: один файл, без Docker, обновление из панели (рекомендуется)' 'SQLite: a single file, no Docker, updates from the panel (recommended)')" \
        "$(t 'PostgreSQL в Docker' 'PostgreSQL in Docker')") in
    1) DB_MODE=sqlite ;;
    2) DB_MODE=docker ;;
    esac
}

DB_MODE=sqlite

fresh_install() {
    detect_old_panels
    local options=() sources=() modes=() pick
    if [ "$HAS_REMNAWAVE" = true ]; then
        options+=("$(t 'Перенести Remnawave в GoWave (SQLite)' 'Migrate Remnawave to GoWave (SQLite)')"
            "$(t 'Перенести Remnawave в GoWave (PostgreSQL в Docker)' 'Migrate Remnawave to GoWave (PostgreSQL in Docker)')")
        sources+=(remnawave remnawave)
        modes+=(sqlite docker)
    fi
    if [ "$HAS_PASARGUARD" = true ]; then
        options+=("$(t 'Перенести PasarGuard в GoWave (SQLite)' 'Migrate PasarGuard to GoWave (SQLite)')"
            "$(t 'Перенести PasarGuard в GoWave (PostgreSQL в Docker)' 'Migrate PasarGuard to GoWave (PostgreSQL in Docker)')")
        sources+=(pasarguard pasarguard)
        modes+=(sqlite docker)
    fi
    if [ ${#options[@]} -gt 0 ]; then
        options+=("$(t 'Установить GoWave без переноса' 'Install GoWave without migration')")
        pick=$(choose "$(t 'На сервере найдена другая панель. Что сделать?' 'Another panel was found on this server. What should be done?')" "${options[@]}")
        if [ "$pick" -le ${#sources[@]} ]; then
            SOURCE=${sources[$((pick - 1))]}
            DB_MODE=${modes[$((pick - 1))]}
            SOURCE_DIR=$REMNAWAVE_DIR
            [ "$SOURCE" = pasarguard ] && SOURCE_DIR=$PASARGUARD_DIR
        else
            choose_database
        fi
    else
        choose_database
    fi

    ask_proxy
    ask_domain

    if [ "$PROXY" != external ] && ! check_web_ports true; then
        die "$(t 'Освободите порты 80/443 (или выберите другой прокси) и запустите установщик снова.' 'Free ports 80/443 (or pick another proxy) and run the installer again.')"
    fi
    APP_PORT=$(free_port 3000)

    printf '\n%s\n' "${BOLD}$(t 'Будет установлено:' 'About to install:')${RESET}"
    echo "$(t "  Панель:   https://$DOMAIN (локально 127.0.0.1:$APP_PORT)" "  Panel:     https://$DOMAIN (locally 127.0.0.1:$APP_PORT)")"
    if [ "$DB_MODE" = sqlite ]; then
        echo "$(t "  База:     SQLite, $INSTALL_DIR" "  Database:  SQLite, $INSTALL_DIR")"
    else
        echo "$(t "  База:     PostgreSQL в Docker, $DOCKER_DIR" "  Database:  PostgreSQL in Docker, $DOCKER_DIR")"
    fi
    if [ "$PROXY" = external ]; then
        echo "$(t "  Прокси:   свой, направьте его на 127.0.0.1:$APP_PORT" "  Proxy:     your own, point it to 127.0.0.1:$APP_PORT")"
    else
        echo "$(t "  Прокси:   $PROXY" "  Proxy:     $PROXY")"
    fi
    if [ "$SOURCE" != none ]; then
        echo "$(t "  Перенос:  $SOURCE. Старая панель будет остановлена (контейнеры и данные не удаляются)." "  Migrate:   $SOURCE. The old panel will be stopped (containers and data are not deleted).")"
    fi
    confirm "$(t 'Продолжить?' 'Continue?')" || die "$(t 'Отменено.' 'Cancelled.')"

    install_release
    write_env
    write_unit
    if [ "$SOURCE" != none ]; then
        migrate_data
        stop_old_panel
    fi
    info "$(t 'Запускаю панель' 'Starting the panel')"
    start_native
    [ "$DB_MODE" = docker ] && move_to_docker

    sleep 2
    if [ "$PROXY" != external ] && ! check_web_ports false; then
        die "$(t "Панель работает на 127.0.0.1:$APP_PORT, но порты 80/443 заняты. Освободите их и запустите установщик снова: пункт «Настроить домен и HTTPS»." "The panel is running on 127.0.0.1:$APP_PORT, but ports 80/443 are taken. Free them and run the installer again: option \"Set up domain and HTTPS\".")"
    fi
    setup_proxy
    print_done
}

# print_external_proxy tells the admin where to point their own proxy.
print_external_proxy() {
    printf '%s\n' "${YELLOW}$(t "Reverse proxy не настроен: направьте свой прокси с HTTPS на http://127.0.0.1:$APP_PORT" "Reverse proxy is not configured: point your own HTTPS proxy to http://127.0.0.1:$APP_PORT")${RESET}"
    echo "$(t 'Нужны заголовки Host, X-Forwarded-For, X-Forwarded-Proto, а для SSH-терминала нод - WebSocket (Upgrade, Connection).' \
        'It must pass the Host, X-Forwarded-For and X-Forwarded-Proto headers, plus WebSocket (Upgrade, Connection) for the node SSH terminal.')"
    echo "$(t 'Если прокси работает в Docker, ему нужен network_mode: host: панель слушает только 127.0.0.1.' \
        'If the proxy runs in Docker, it needs network_mode: host, because the panel listens on 127.0.0.1 only.')"
    echo "$(t 'Примеры:' 'Examples:') https://aaasazan.github.io/gowave-docs/install/reverse-proxy/overview/"
    echo "$(t 'Пока прокси не настроен, панель доступна только с этого сервера, создать администратора извне нельзя.' \
        'Until the proxy is set up, the panel is reachable only from this server, so the admin cannot be created from outside.')"
}

print_done() {
    printf '\n%s\n' "${GREEN}${BOLD}$(t 'GoWave установлена:' 'GoWave is installed:') https://$DOMAIN${RESET}"
    [ "$PROXY" = external ] && print_external_proxy
    case $SOURCE in
    none)
        if [ "$PROXY" = external ]; then
            echo "$(t 'Когда прокси заработает, откройте адрес и создайте администратора: это сделает первый, кто откроет панель.' \
                'Once the proxy works, open the address and create the admin: whoever opens the panel first gets to do it.')"
        else
            echo "$(t 'Откройте адрес и создайте администратора: это сделает первый, кто откроет панель.' \
                'Open the address and create the admin: whoever opens the panel first gets to do it.')"
        fi
        ;;
    remnawave)
        echo "$(t 'Данные Remnawave перенесены, входите под прежним администратором.' \
            'Remnawave data has been migrated, log in with your existing admin.')"
        ;;
    pasarguard)
        echo "$(t 'Перенесены пользователи PasarGuard и их HWID. Создайте администратора, затем настройте ноды, профили и хосты.' \
            'PasarGuard users and their HWIDs have been migrated. Create an admin, then set up nodes, profiles and hosts.')"
        ;;
    esac
    if [ "$DB_MODE" = docker ]; then
        echo "$(t 'Проект Docker: ' 'Docker project:') $DOCKER_DIR (docker compose logs -f gowave)"
    else
        echo "$(t 'Конфигурация:  ' 'Configuration: ') $INSTALL_DIR/.env ($(t 'логи' 'logs'): journalctl -u gowave -f)"
    fi
    echo "$(t 'Консоль:       ' 'Console:       ') gowave rescue"
    if [ "$SOURCE" != none ]; then
        echo "$(t "Старая панель остановлена и не запускается при загрузке. Её файлы остались в $SOURCE_DIR." \
            "The old panel is stopped and will not start on boot. Its files are still in $SOURCE_DIR.")"
        echo "$(t "Когда убедитесь, что всё работает, удалите её: cd $SOURCE_DIR && docker compose down" \
            "Once you are sure everything works, remove it: cd $SOURCE_DIR && docker compose down")"
    fi
    echo "$(t 'Повторный запуск установщика: обновление, смена домена, удаление.' \
        'Run the installer again to update, change the domain or uninstall.')"
}

# ---------------------------------------------------------------- existing install

MODE=''
ENV_FILE=''

detect_existing() {
    if [ -f "$DOCKER_DIR/docker-compose.yml" ]; then
        MODE=docker
        ENV_FILE=$DOCKER_DIR/.env
    elif [ -f "$UNIT_FILE" ] || [ -x "$INSTALL_DIR/gowave-server" ]; then
        MODE=native
        ENV_FILE=$INSTALL_DIR/.env
    fi
}

configured_proxy() {
    if [ -f "$CADDY_SITE" ]; then
        echo caddy
    elif [ -f "$NGINX_SITE" ]; then
        echo nginx
    elif [ "$(env_get "$ENV_FILE" GOWAVE_PROXY)" = external ]; then
        echo external
    fi
}

update_gowave() {
    APP_PORT=$(env_get "$ENV_FILE" APP_PORT)
    APP_PORT=${APP_PORT:-3000}
    if [ "$MODE" = docker ]; then
        # Older compose files published the panel on every interface.
        # shellcheck disable=SC2016
        sed -i 's#^\(\s*- \)"${APP_PORT:-3000}:3000"#\1"127.0.0.1:${APP_PORT:-3000}:3000"#' "$DOCKER_DIR/docker-compose.yml"
        info "$(t 'Обновляю образы' 'Updating images')"
        (cd "$DOCKER_DIR" && run docker compose pull && run docker compose up -d --wait --wait-timeout 180)
    else
        install_release
        run systemctl restart gowave
        wait_healthy "$APP_PORT" || die "$(t 'Панель не запустилась после обновления (journalctl -u gowave).' 'The panel did not start after the update (journalctl -u gowave).')"
    fi
    info "$(t 'GoWave обновлена' 'GoWave is updated')"

    if [ -z "$(configured_proxy)" ]; then
        warn "$(t 'Панель слушает только 127.0.0.1, а домен с HTTPS не настроен.' 'The panel listens on 127.0.0.1 only and no domain with HTTPS is set up.')"
        confirm "$(t 'Настроить домен и HTTPS сейчас?' 'Set up a domain and HTTPS now?')" && configure_domain
    fi
    return 0
}

configure_domain() {
    APP_PORT=$(env_get "$ENV_FILE" APP_PORT)
    APP_PORT=${APP_PORT:-3000}
    ask_proxy
    ask_domain
    local previous
    previous=$(configured_proxy)
    if [ -n "$previous" ] && [ "$previous" != "$PROXY" ]; then
        case $previous in
        caddy) remove_caddy_site ;;
        nginx) remove_nginx_site ;;
        esac
    fi
    if [ "$PROXY" != external ]; then
        check_web_ports false || die "$(t 'Освободите порты 80/443 и запустите установщик снова.' 'Free ports 80/443 and run the installer again.')"
    fi
    env_set "$ENV_FILE" PANEL_DOMAIN "$DOMAIN"
    env_set "$ENV_FILE" SUB_PUBLIC_DOMAIN "$DOMAIN/api/sub"
    env_set "$ENV_FILE" GOWAVE_PROXY "$PROXY"
    if [ "$MODE" = docker ]; then
        (cd "$DOCKER_DIR" && run docker compose up -d --force-recreate --wait gowave)
    else
        env_set "$ENV_FILE" APP_HOST 127.0.0.1
        run systemctl restart gowave
        wait_healthy "$APP_PORT" || die "$(t 'Панель не запустилась (journalctl -u gowave).' 'The panel did not start (journalctl -u gowave).')"
    fi
    setup_proxy
    if [ "$PROXY" = external ]; then
        print_external_proxy
    else
        info "$(t 'Готово:' 'Done:') https://$DOMAIN"
    fi
}

uninstall_gowave() {
    warn "$(t 'Будут удалены панель, её база данных и конфигурация прокси для неё.' 'This removes the panel, its database and its proxy configuration.')"
    [ "$(ask "$(t 'Введите yes для подтверждения: ' 'Type yes to confirm: ')")" = yes ] || die "$(t 'Отменено.' 'Cancelled.')"
    if confirm "$(t 'Сохранить резервную копию базы и .env в /root?' 'Save a backup of the database and .env to /root?')"; then
        local backup
        backup=/root/gowave-backup-$(date +%Y%m%d-%H%M%S)
        mkdir -p "$backup"
        if [ "$MODE" = docker ]; then
            cp -p "$DOCKER_DIR/.env" "$backup/" 2>/dev/null || true
            (cd "$DOCKER_DIR" && docker compose exec -T postgres sh -c 'pg_dump -U "$POSTGRES_USER" "$POSTGRES_DB"') >"$backup/gowave.sql" ||
                die "$(t 'Не удалось снять дамп PostgreSQL, ничего не удалено.' 'Could not dump PostgreSQL, nothing was removed.')"
        fi
        if [ -d "$INSTALL_DIR" ]; then
            mkdir -p "$backup/sqlite"
            cp -p "$INSTALL_DIR/.env" "$INSTALL_DIR"/gowave.db* "$backup/sqlite/" 2>/dev/null || true
        fi
        info "$(t 'Копия сохранена:' 'Backup saved:') $backup"
    fi
    if [ -f "$DOCKER_DIR/docker-compose.yml" ]; then
        (cd "$DOCKER_DIR" && run docker compose down --volumes --remove-orphans)
    fi
    rm -rf "$DOCKER_DIR"
    systemctl disable --now gowave >/dev/null 2>&1 || true
    rm -f "$UNIT_FILE"
    systemctl daemon-reload
    rm -rf "$INSTALL_DIR"
    rm -f "$CLI_PATH"
    remove_caddy_site
    remove_nginx_site
    info "$(t 'GoWave удалена. Caddy/Nginx, Docker и сертификаты оставлены.' 'GoWave is removed. Caddy/Nginx, Docker and certificates are left in place.')"
}

manage_existing() {
    local where=$INSTALL_DIR
    [ "$MODE" = docker ] && where=$DOCKER_DIR
    case $(choose "$(t "GoWave уже установлена ($where). Что сделать?" "GoWave is already installed ($where). What should be done?")" \
        "$(t 'Обновить GoWave' 'Update GoWave')" \
        "$(t 'Настроить домен и HTTPS' 'Set up domain and HTTPS')" \
        "$(t 'Удалить GoWave' 'Uninstall GoWave')" \
        "$(t 'Выход' 'Exit')") in
    1) update_gowave ;;
    2) configure_domain ;;
    3) uninstall_gowave ;;
    4) exit 0 ;;
    esac
}

main() {
    detect_system
    WORK=$(mktemp -d)
    trap 'rm -rf "$WORK"' EXIT
    trap 'line=$LINENO; printf "%s\n" "${RED}[x] $(t "Неожиданная ошибка (строка $line), лог: $LOG" "Unexpected error (line $line), log: $LOG")${RESET}" >&2' ERR
    ensure_tools
    detect_existing
    if [ -n "$MODE" ]; then
        manage_existing
    else
        fresh_install
    fi
}

main "$@"
