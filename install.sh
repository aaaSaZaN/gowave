#!/usr/bin/env bash
# GoWave installer. Run it again on the same server to update, change the
# domain or remove GoWave.
#
#   sudo bash -c "$(curl -fsSL https://raw.githubusercontent.com/aaaSaZaN/gowave/main/install.sh)"
#
# Installs the panel (SQLite with systemd, or PostgreSQL in Docker) behind
# Caddy or Nginx with HTTPS, or leaves the reverse proxy to the admin. The
# panel listens on 127.0.0.1 only. When an upstream panel or PasarGuard install is
# found, offers to move its data. Asks for the language (Russian or English)
# first.
#
# Options, also accepted as environment variables for Ansible and other tools
# (run `install.sh --help`). With a downloaded file: bash install.sh --update --dev
#   --stable | --dev | --channel stable|dev  GOWAVE_CHANNEL  update channel
#   --version v1.2.3                         GOWAVE_VERSION  pin a stable release
#   --update                                 GOWAVE_ACTION=update
#   --lang ru|en                             GOWAVE_LANG
# --update updates an existing install without any prompts and without a
# terminal, keeping its channel unless one is given.
set -Eeuo pipefail

REPO=aaaSaZaN/gowave
IMAGE_REPO=ghcr.io/aaasazan/gowave
GOWAVE_VERSION=${GOWAVE_VERSION:-latest}
GOWAVE_CHANNEL=${GOWAVE_CHANNEL:-}
GOWAVE_ACTION=${GOWAVE_ACTION:-}
GOWAVE_LANG=${GOWAVE_LANG:-}
NONINTERACTIVE=false
INSTALL_DIR=/opt/gowave
DOCKER_DIR=/opt/gowave-docker
UNIT_FILE=/etc/systemd/system/gowave.service
UPDATE_PATH_UNIT=/etc/systemd/system/gowave-update.path
UPDATE_SERVICE_UNIT=/etc/systemd/system/gowave-update.service
CLI_PATH=/usr/local/bin/gowave
CADDYFILE=/etc/caddy/Caddyfile
CADDY_SITE=/etc/caddy/gowave.caddy
NGINX_SITE=/etc/nginx/conf.d/gowave.conf
NGINX_RENEW_HOOK=/etc/letsencrypt/renewal-hooks/deploy/gowave-nginx.sh
ACME_ROOT=/var/www/gowave-acme
UPSTREAM_DIR=/opt/remnawave
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

# ---------------------------------------------------------------- options

usage() {
    cat <<'USAGE'
GoWave installer / Установщик GoWave

Usage: install.sh [options]
  sudo bash -c "$(curl -fsSL https://raw.githubusercontent.com/aaaSaZaN/gowave/main/install.sh)" _ [options]

  --stable               Stable channel: tagged releases (default)
                         Канал stable: релизы по тегам (по умолчанию)
  --dev                  Dev channel: a build of every commit to main
                         Канал dev: сборка каждого коммита в main
  --channel stable|dev   Same as --stable / --dev
  --version v1.2.3       Pin a stable release / Установить конкретный релиз
  --update               Update an existing install without prompts (no terminal needed)
                         Обновить установленную панель без вопросов (терминал не нужен)
  --lang ru|en           Language without asking / Язык без вопроса
  -h, --help             This help / Эта справка

Environment: GOWAVE_CHANNEL, GOWAVE_VERSION (latest, dev or v1.2.3),
GOWAVE_ACTION=update, GOWAVE_LANG.
Without a channel the installer asks; --update keeps the installed channel.
USAGE
}

# Errors here come before the language is known, so they are in both.
parse_args() {
    while [ $# -gt 0 ]; do
        case $1 in
        --stable) GOWAVE_CHANNEL=stable ;;
        --dev) GOWAVE_CHANNEL=dev ;;
        --channel | --version | --lang)
            [ $# -ge 2 ] || die "Не указано значение для $1 / $1 needs a value"
            case $1 in
            --channel) GOWAVE_CHANNEL=$2 ;;
            --version) GOWAVE_VERSION=$2 ;;
            --lang) GOWAVE_LANG=$2 ;;
            esac
            shift
            ;;
        --channel=*) GOWAVE_CHANNEL=${1#*=} ;;
        --version=*) GOWAVE_VERSION=${1#*=} ;;
        --lang=*) GOWAVE_LANG=${1#*=} ;;
        --update) GOWAVE_ACTION=update ;;
        -h | --help) usage; exit 0 ;;
        *) die "Неизвестный параметр: $1 (см. --help) / Unknown option: $1 (see --help)" ;;
        esac
        shift
    done
    case $GOWAVE_CHANNEL in '' | stable | dev) ;; *) die "Канал должен быть stable или dev / The channel must be stable or dev" ;; esac
    case $GOWAVE_LANG in '' | ru | en) ;; *) die "Язык должен быть ru или en / The language must be ru or en" ;; esac
    case $GOWAVE_ACTION in '' | update) ;; *) die "GOWAVE_ACTION поддерживает только update / GOWAVE_ACTION supports only update" ;; esac
    if [ "$GOWAVE_VERSION" = dev ]; then
        [ "$GOWAVE_CHANNEL" != stable ] || die "GOWAVE_VERSION=dev противоречит --stable / GOWAVE_VERSION=dev conflicts with --stable"
        GOWAVE_CHANNEL=dev
    elif [ "$GOWAVE_VERSION" != latest ]; then
        [[ $GOWAVE_VERSION =~ ^v[0-9][0-9A-Za-z.+-]*$ ]] ||
            die "Версия должна быть latest, dev или v1.2.3 / The version must be latest, dev or v1.2.3"
        [ "$GOWAVE_CHANNEL" != dev ] || die "--version закрепляет stable-релиз, он не сочетается с --dev / --version pins a stable release and conflicts with --dev"
        GOWAVE_CHANNEL=stable
    fi
    apply_channel
}

# apply_channel points downloads at the chosen channel: the dev pre-release, or
# the latest (or pinned) stable release.
apply_channel() {
    if [ "$GOWAVE_CHANNEL" = dev ]; then
        GOWAVE_VERSION=dev
    elif [ "$GOWAVE_VERSION" = dev ]; then
        GOWAVE_VERSION=latest
    fi
}

channel_label() {
    local label
    if [ "$1" = dev ]; then
        label=$(t 'Dev: сборка каждого коммита в main, может быть нестабильной' 'Dev: a build of every commit to main, may be unstable')
    else
        label=$(t 'Stable: проверенные релизы (рекомендуется)' 'Stable: tested releases (recommended)')
    fi
    [ "$1" = "${2:-}" ] && label="$label $(t '(текущий)' '(current)')"
    printf '%s' "$label"
}

# choose_channel asks for the channel unless an option already set it. The
# installed channel (if any) is listed first, so Enter keeps it.
choose_channel() {
    local current=${1:-} first=stable second=dev
    [ -z "$GOWAVE_CHANNEL" ] || return 0
    if [ "$current" = dev ]; then first=dev second=stable; fi
    case $(choose "$(t 'Канал обновлений:' 'Update channel:')" "$(channel_label $first "$current")" "$(channel_label $second "$current")") in
    1) GOWAVE_CHANNEL=$first ;;
    *) GOWAVE_CHANNEL=$second ;;
    esac
    apply_channel
}

# panel_image is the Docker image of the chosen channel or pinned release.
panel_image() {
    case $GOWAVE_VERSION in
    latest) printf '%s:latest' "$IMAGE_REPO" ;;
    dev) printf '%s:dev' "$IMAGE_REPO" ;;
    *) printf '%s:%s' "$IMAGE_REPO" "${GOWAVE_VERSION#v}" ;;
    esac
}

# installed_channel reads the channel of the existing install: the Compose image
# tag, or the version of a trusted native binary (dev builds are dev-<sha>).
installed_channel() {
    local version
    if [ "$MODE" = docker ]; then
        case $(env_get "$ENV_FILE" GOWAVE_IMAGE) in
        *:dev | *:dev-*) echo dev ;;
        *) echo stable ;;
        esac
    elif trusted_binary && version=$(cd "$INSTALL_DIR" && ./gowave-server --version </dev/null 2>/dev/null); then
        case $version in
        dev-*) echo dev ;;
        *) echo stable ;;
        esac
    else
        echo stable
    fi
}

# ---------------------------------------------------------------- system

PKG=''
ARCH=''
APT_UPDATED=false

detect_system() {
    # The language is not known yet: these two errors come in both.
    [ "$(id -u)" -eq 0 ] || die 'Запустите установщик от root (sudo). / Run the installer as root (sudo).'
    if [ "$GOWAVE_ACTION" = update ]; then
        # Unattended: prompts read nothing and take their defaults.
        NONINTERACTIVE=true
        exec 3</dev/null
        UI_LANG=${GOWAVE_LANG:-ru}
    else
        (: </dev/tty) 2>/dev/null ||
            die 'Нужен интерактивный терминал: запустите команду из README в SSH-сессии или обновите без вопросов с --update. / An interactive terminal is required: run the command from the README in an SSH session, or update without prompts with --update.'
        exec 3</dev/tty
        if [ -n "$GOWAVE_LANG" ]; then UI_LANG=$GOWAVE_LANG; else ask_language; fi
    fi
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

HAS_UPSTREAM=false
HAS_PASARGUARD=false
SOURCE=none
SOURCE_DIR=''

detect_old_panels() {
    [ -f "$UPSTREAM_DIR/.env" ] && HAS_UPSTREAM=true
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
OLD_PANEL_POLICIES=''

stop_old_panel() {
    local ids id
    ids=$(old_containers "$SOURCE_DIR")
    [ -n "$ids" ] || return 0
    # Kept for rollback_fresh_install, which brings the old panel back.
    for id in $ids; do
        OLD_PANEL_POLICIES+="$id $(docker inspect -f '{{.HostConfig.RestartPolicy.Name}}' "$id" 2>/dev/null || echo no)"$'\n'
    done
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
    tar --no-same-owner -xzf "$dir/gowave-frontend.tar.gz" -C "$INSTALL_DIR/frontend/dist.new"
    [ -d "$INSTALL_DIR/frontend/dist" ] && mv "$INSTALL_DIR/frontend/dist" "$INSTALL_DIR/frontend/dist.old"
    mv "$INSTALL_DIR/frontend/dist.new" "$INSTALL_DIR/frontend/dist"
    rm -rf "$INSTALL_DIR/frontend/dist.old"
}

# Notification settings that carry over from a migrated panel's .env.
NOTIFY_KEYS='IS_TELEGRAM_NOTIFICATIONS_ENABLED TELEGRAM_BOT_TOKEN TELEGRAM_BOT_API_ROOT TELEGRAM_BOT_PROXY
TELEGRAM_NOTIFY_USERS TELEGRAM_NOTIFY_NODES TELEGRAM_NOTIFY_CRM TELEGRAM_NOTIFY_SERVICE TELEGRAM_NOTIFY_TBLOCKER
WEBHOOK_ENABLED WEBHOOK_URL WEBHOOK_SECRET_HEADER
BANDWIDTH_USAGE_NOTIFICATIONS_ENABLED BANDWIDTH_USAGE_NOTIFICATIONS_THRESHOLD
NOT_CONNECTED_USERS_NOTIFICATIONS_ENABLED NOT_CONNECTED_USERS_NOTIFICATIONS_AFTER_HOURS
EXPIRATION_NOTIFICATIONS_ENABLED EXPIRATION_NOTIFICATIONS'

# copy_notifications SRC DEST copies NOTIFY_KEYS lines from SRC into DEST,
# replacing earlier values. Lines are copied as written and never printed: they
# hold the bot token. Unedited "change_me" values are skipped, and a channel is
# left disabled when its credentials would not pass the panel's validation.
# Prints the number of copied settings.
copy_notifications() {
    local src=$1 dest=$2 key line token secret count=0 kept=$WORK/env.kept added=$WORK/env.added
    token=$(env_get "$src" TELEGRAM_BOT_TOKEN)
    secret=$(env_get "$src" WEBHOOK_SECRET_HEADER)
    : >"$added"
    for key in $NOTIFY_KEYS; do
        line=$(grep "^$key=" "$src" | tail -n 1) || true
        [ -n "$line" ] || continue
        [ "$(env_get "$src" "$key")" != change_me ] || continue
        case $key in
            IS_TELEGRAM_NOTIFICATIONS_ENABLED)
                if [ -z "$token" ] || [ "$token" = change_me ]; then line="$key=false"; fi ;;
            WEBHOOK_ENABLED)
                [[ $secret =~ ^[A-Za-z0-9]{32,}$ ]] || line="$key=false" ;;
        esac
        printf '%s\n' "$line" >>"$added"
        count=$((count + 1))
    done
    if [ "$count" -gt 0 ]; then
        grep -vE "^($(echo $NOTIFY_KEYS | tr ' ' '|'))=" "$dest" >"$kept" || true
        # Rewrite in place so the file keeps its owner and mode.
        { cat "$kept"; printf '# Notifications copied from %s\n' "$src"; cat "$added"; } >"$dest"
    fi
    rm -f "$kept" "$added"
    echo "$count"
}

# import_notifications copies the old panel's Telegram and webhook settings into
# an existing installation and restarts the panel.
import_notifications() {
    local src=$UPSTREAM_DIR/.env count
    [ -f "$src" ] || { warn "$(t "Не найден $src." "$src not found.")"; return 0; }
    count=$(copy_notifications "$src" "$ENV_FILE")
    if [ "$count" -eq 0 ]; then
        warn "$(t "В $src нет настроек уведомлений." "$src has no notification settings.")"
        return 0
    fi
    info "$(t "Перенесено настроек уведомлений: $count. Перезапускаю панель" "Copied $count notification settings. Restarting the panel")"
    restart_panel || die "$(t 'Панель не перезапустилась, проверьте значения в .env.' 'The panel did not restart; check the values in .env.')"
    wait_healthy "$(env_get "$ENV_FILE" APP_PORT)" ||
        die "$(t "Панель не запустилась с новыми настройками. Проверьте уведомления в $ENV_FILE." "The panel did not start with the new settings. Check the notification values in $ENV_FILE.")"
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
# Prometheus metrics: set METRICS_ENABLED=true and METRICS_USER/METRICS_PASS
# to open METRICS_PORT on 127.0.0.1.
METRICS_ENABLED=false
METRICS_PORT=$metrics_port
APP_SECRET=$(openssl rand -hex 32)
DB_DRIVER=sqlite
DATABASE_URL=gowave.db
PANEL_DOMAIN=$DOMAIN
PANEL_BASE_PATH=$PANEL_BASE_PATH
SUB_PUBLIC_DOMAIN=$DOMAIN/api/sub
GOWAVE_PROXY=$PROXY
EOF
    )
}

write_unit() {
    if [ ! -f "$UNIT_FILE" ]; then
        cat >"$UNIT_FILE" <<EOF
[Unit]
Description=GoWave Panel
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=gowave
Group=gowave
WorkingDirectory=$INSTALL_DIR
ExecStart=$INSTALL_DIR/gowave-server
Restart=always
RestartSec=5
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF
    fi
    run "$INSTALL_DIR/gowave-server" service install "$INSTALL_DIR"
    if [ ! -e "$CLI_PATH" ] || [ -L "$CLI_PATH" ]; then
        ln -sfn "$INSTALL_DIR/gowave-server" "$CLI_PATH"
    fi
}

start_native() {
    run systemctl enable gowave
    run systemctl restart gowave
    wait_healthy "$APP_PORT" || {
        journalctl -u gowave -n 30 --no-pager >&2 || true
        die "$(t 'Панель не запустилась (journalctl -u gowave).' 'The panel did not start (journalctl -u gowave).')"
    }
}

# Root must never run a binary the gowave service account could have replaced:
# installs made before the root-owned layout gave it the whole directory.
trusted_binary() {
    local bin=$INSTALL_DIR/gowave-server mode
    mode=$(stat -c '%u %a' "$bin" 2>/dev/null) || return 1
    [ ! -L "$bin" ] && [ "${mode%% *}" = 0 ] && [ $((8#${mode#* } & 8#022)) -eq 0 ]
}

require_trusted_binary() {
    local bin=$INSTALL_DIR/gowave-server
    [ -e "$bin" ] || [ -L "$bin" ] || die "$(t "Не найден $bin." "$bin not found.")"
    if ! trusted_binary; then
        die "$(t "$bin может изменить сервисный пользователь, запускать его от root небезопасно. Выберите «Обновить» в меню: панель переустановится из проверенного релиза." \
            "$bin can be modified by the service account, so it is unsafe to run as root. Choose Update in the menu to reinstall the panel from a verified release.")"
    fi
}

gowave_cli() {
    require_trusted_binary
    (cd "$INSTALL_DIR" && ./gowave-server "$@") </dev/null 2>&1 | tee -a "$LOG"
    return "${PIPESTATUS[0]}"
}

# migrate_data copies the old panel into $INSTALL_DIR/gowave.db. The old
# panel's app container is stopped first for a consistent copy and started
# again if the copy fails; its database keeps running.
migrate_data() {
    local app_service app_ids db_ids status=0
    if [ "$SOURCE" = upstream ]; then
        app_service=remnawave
        db_ids=$(old_containers "$UPSTREAM_DIR" remnawave-db)
        # shellcheck disable=SC2086
        [ -n "$db_ids" ] && run docker start $db_ids
    else
        app_service=pasarguard
    fi
    app_ids=$(old_containers "$SOURCE_DIR" "$app_service")
    # shellcheck disable=SC2086
    [ -n "$app_ids" ] && run docker stop $app_ids

    info "$(t 'Переношу данные из' 'Migrating data from') $SOURCE"
    if [ "$SOURCE" = upstream ]; then
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
    local image_args=()
    # The default image is :latest; other channels and pinned releases name theirs.
    [ "$GOWAVE_VERSION" != latest ] && image_args=(--image "$(panel_image)")
    gowave_cli migrate to-docker --yes --from "$INSTALL_DIR" "${image_args[@]}" ||
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
	# Covers application/wasm; the Xray validator itself is already served gzip-compressed.
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
    # application/wasm: the Xray validator. GoWave already sends it gzip-compressed,
    # and Nginx passes such responses through without compressing them again.
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
    info "$(t 'Проверяю' 'Checking') https://$DOMAIN$PANEL_BASE_PATH"
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

# Dashboard paths are restricted to characters safe in URLs and .env files.
PANEL_BASE_PATH=/
INSTALL_NODE=false
NODE_INSTALL_URL=https://raw.githubusercontent.com/aaaSaZaN/gowave-node/main/install.sh

normalize_panel_path() {
    local value=${1:-/} first
    if [ "$value" = / ]; then printf /; return; fi
    [[ $value =~ ^/[a-zA-Z0-9_-]+(/[a-zA-Z0-9_-]+)*/?$ ]] || return 1
    first=${value#/}; first=${first%%/*}
    case $first in api|metrics) return 1 ;; esac
    printf '%s/' "${value%/}"
}

ask_panel_path() {
    local value normalized
    while :; do
        value=$(ask "$(t 'Путь панели' 'Dashboard path') [${PANEL_BASE_PATH:-/}]: ")
        if normalized=$(normalize_panel_path "${value:-${PANEL_BASE_PATH:-/}}"); then
            PANEL_BASE_PATH=$normalized
            return
        fi
        warn "$(t 'Пример: /panel/. Допустимы буквы, цифры, дефис и подчёркивание; /api и /metrics заняты.' 'Example: /panel/. Use letters, numbers, hyphens and underscores; /api and /metrics are reserved.')"
    done
}

restart_panel() {
    if [ "$MODE" = docker ]; then
        (cd "$DOCKER_DIR" && docker compose up -d --force-recreate --wait --wait-timeout 180 gowave) >>"$LOG" 2>&1
    else
        systemctl restart gowave >>"$LOG" 2>&1
    fi
}

verify_panel_path() {
    local output=$WORK/dashboard-check.html
    wait_healthy "$APP_PORT" || return 1
    curl -fsS --max-time 10 "http://127.0.0.1:$APP_PORT$PANEL_BASE_PATH" -o "$output" || return 1
    if [ "${1:-}" = --legacy ]; then
        # Recovery must also accept bundles installed before base-path support.
        grep -Eqi '<!doctype html|<html' "$output"
    else
        grep -Fq "<base href=\"$PANEL_BASE_PATH\" data-gowave-base" "$output"
    fi
}

configure_panel_path() {
    local previous backup
    APP_PORT=$(env_get "$ENV_FILE" APP_PORT); APP_PORT=${APP_PORT:-3000}
    DOMAIN=$(env_get "$ENV_FILE" PANEL_DOMAIN)
    previous=$(env_get "$ENV_FILE" PANEL_BASE_PATH); previous=${previous:-/}
    PANEL_BASE_PATH=$previous
    ask_panel_path
    [ "$PANEL_BASE_PATH" != "$previous" ] || return 0
    backup=$(mktemp "$ENV_FILE.path-backup.XXXXXX") || return 1
    (umask 077; cp -p "$ENV_FILE" "$backup"; chmod 600 "$backup") || return 1
    if env_set "$ENV_FILE" PANEL_BASE_PATH "$PANEL_BASE_PATH" && restart_panel && verify_panel_path; then
        info "$(t 'Панель доступна:' 'Dashboard available:') https://$DOMAIN$PANEL_BASE_PATH"
        info "$(t 'Резервная копия:' 'Configuration backup:') $backup"
        warn "$(t 'Для собственного прокси: передавайте путь без изменений. Это меняет адрес панели, но не ограничивает доступ к API.' 'For external proxies, forward the path unchanged. This changes the dashboard address; it does not restrict API access.')"
        return 0
    fi
    warn "$(t 'Новый путь не прошёл проверку. Восстанавливаю конфигурацию.' 'The new path failed verification. Restoring configuration.')"
    cp -p "$backup" "$ENV_FILE" || return 1
    PANEL_BASE_PATH=$previous
    if ! restart_panel || ! verify_panel_path --legacy; then
        warn "$(t 'Автовосстановление не прошло проверку. Конфигурация сохранена в' 'Automatic recovery failed verification. Configuration is saved at') $backup"
        return 1
    fi
    warn "$(t 'Прежняя конфигурация восстановлена.' 'The previous configuration has been restored.')"
    return 1
}

panel_node_secret() {
    # Do not use run/gowave_cli: those log command output, including secrets.
    if [ "$MODE" = docker ]; then
        (cd "$DOCKER_DIR" && docker compose exec -T gowave /app/gowave rescue --print-secret-key-raw </dev/null)
    else
        require_trusted_binary
        (cd "$INSTALL_DIR" && ./gowave-server rescue --print-secret-key-raw </dev/null)
    fi
}

install_local_node() {
    local installer=$WORK/node-install.sh secret port address
    port=$(ask "$(t 'Порт новой ноды (существующая нода сохранит свой порт)' 'New node port (existing nodes retain their port)') [2222]: ")
    port=${port:-2222}
    if ! [[ $port =~ ^[0-9]{1,5}$ ]] || [ "$((10#$port))" -lt 1 ] || [ "$((10#$port))" -gt 65535 ]; then
        warn "$(t 'Некорректный порт.' 'Invalid node port.')"; return 0
    fi
    port=$((10#$port))
    if ! secret=$(panel_node_secret) || ! [[ $secret =~ ^[A-Za-z0-9+/=_-]+$ ]]; then
        warn "$(t 'Не удалось получить ключ ноды. Обновите панель и повторите установку ноды через меню.' 'Could not obtain node credentials. Update the panel and retry node installation from the menu.')"
        return 0
    fi
    if ! curl -fsSL --connect-timeout 15 --max-time 180 --retry 2 -o "$installer" "$NODE_INSTALL_URL"; then
        unset secret
        warn "$(t 'Не удалось скачать установщик ноды. Повторите через меню; панель продолжает работать.' 'Could not download the node installer. Retry from the menu; the panel remains running.')"
        return 0
    fi
    # The maintained installer validates port conflicts before touching services,
    # preserves existing credentials/ports, and rolls back failed migrations.
    if ! SECRET_KEY="$secret" NODE_PORT="$port" sh "$installer"; then
        unset secret
        warn "$(t 'Нода не установлена. Повторите через меню; панель продолжает работать.' 'Node installation failed. Retry from the menu; the panel remains running.')"
        return 0
    fi
    unset secret
    if [ "$MODE" = docker ]; then
        address=$(server_ips | head -n 1)
        address=${address:-SERVER_IP}
    else
        address=127.0.0.1
    fi
    info "$(t 'Добавьте ноду в панели: адрес' 'Add the node in the dashboard: address') $address, $(t 'порт' 'port') $port."
    info "$(t 'Для существующей ноды используйте сохранённые адрес и порт. Затем назначьте профиль и хост.' 'For an existing node, use its saved address and port. Then assign a configuration profile and host.')"
}

fresh_install() {
    detect_old_panels
    local options=() sources=() modes=() pick
    if [ "$HAS_UPSTREAM" = true ]; then
        options+=("$(t 'Перенести прежнюю панель в GoWave (SQLite)' 'Migrate upstream panel to GoWave (SQLite)')"
            "$(t 'Перенести прежнюю панель в GoWave (PostgreSQL в Docker)' 'Migrate upstream panel to GoWave (PostgreSQL in Docker)')")
        sources+=(upstream upstream)
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
            SOURCE_DIR=$UPSTREAM_DIR
            [ "$SOURCE" = pasarguard ] && SOURCE_DIR=$PASARGUARD_DIR
        else
            choose_database
        fi
    else
        choose_database
    fi

    choose_channel
    ask_proxy
    ask_domain
    ask_panel_path
    INSTALL_NODE=false
    confirm "$(t 'Установить GoWave Node на этом сервере после установки панели?' 'Install GoWave Node on this server after the panel is ready?')" && INSTALL_NODE=true

    if [ "$PROXY" != external ] && ! check_web_ports true; then
        die "$(t 'Освободите порты 80/443 (или выберите другой прокси) и запустите установщик снова.' 'Free ports 80/443 (or pick another proxy) and run the installer again.')"
    fi
    APP_PORT=$(free_port 3000)

    printf '\n%s\n' "${BOLD}$(t 'Будет установлено:' 'About to install:')${RESET}"
    echo "$(t "  Панель:   https://$DOMAIN$PANEL_BASE_PATH (локально 127.0.0.1:$APP_PORT)" "  Panel:     https://$DOMAIN$PANEL_BASE_PATH (locally 127.0.0.1:$APP_PORT)")"
    if [ "$DB_MODE" = sqlite ]; then
        echo "$(t "  База:     SQLite, $INSTALL_DIR" "  Database:  SQLite, $INSTALL_DIR")"
    else
        echo "$(t "  База:     PostgreSQL в Docker, $DOCKER_DIR" "  Database:  PostgreSQL in Docker, $DOCKER_DIR")"
    fi
    echo "$(t "  Канал:    $GOWAVE_CHANNEL ($(panel_version_label))" "  Channel:   $GOWAVE_CHANNEL ($(panel_version_label))")"
    if [ "$PROXY" = external ]; then
        echo "$(t "  Прокси:   свой, направьте его на 127.0.0.1:$APP_PORT" "  Proxy:     your own, point it to 127.0.0.1:$APP_PORT")"
    else
        echo "$(t "  Прокси:   $PROXY" "  Proxy:     $PROXY")"
    fi
    if [ "$SOURCE" != none ]; then
        echo "$(t "  Перенос:  $SOURCE. Старая панель будет остановлена (контейнеры и данные не удаляются)." "  Migrate:   $SOURCE. The old panel will be stopped (containers and data are not deleted).")"
    fi
    confirm "$(t 'Продолжить?' 'Continue?')" || die "$(t 'Отменено.' 'Cancelled.')"

    FRESH_ROLLBACK=true
    install_release
    write_env
    if [ "$SOURCE" = upstream ] && [ -f "$UPSTREAM_DIR/.env" ]; then
        info "$(t 'Переношу настройки уведомлений:' 'Copying notification settings:') $(copy_notifications "$UPSTREAM_DIR/.env" "$INSTALL_DIR/.env")"
    fi
    if [ "$SOURCE" != none ]; then
        migrate_data
        stop_old_panel
        OLD_PANEL_STOPPED=true
    fi
    write_unit
    info "$(t 'Запускаю панель' 'Starting the panel')"
    start_native
    FRESH_ROLLBACK=false
    [ "$DB_MODE" = docker ] && move_to_docker

    sleep 2
    if [ "$PROXY" != external ] && ! check_web_ports false; then
        die "$(t "Панель работает на 127.0.0.1:$APP_PORT, но порты 80/443 заняты. Освободите их и запустите установщик снова: пункт «Настроить домен и HTTPS»." "The panel is running on 127.0.0.1:$APP_PORT, but ports 80/443 are taken. Free them and run the installer again: option \"Set up domain and HTTPS\".")"
    fi
    setup_proxy
    print_done
    MODE=native
    ENV_FILE=$INSTALL_DIR/.env
    if [ "$DB_MODE" = docker ]; then MODE=docker; ENV_FILE=$DOCKER_DIR/.env; fi
    if [ "$INSTALL_NODE" = true ]; then install_local_node; fi
}

FRESH_ROLLBACK=false
OLD_PANEL_STOPPED=false

# rollback_fresh_install removes what a fresh install created before the
# panel first started, so a rerun does not take the leftovers for an
# installed GoWave. Nothing else in $INSTALL_DIR is touched.
rollback_fresh_install() {
    warn "$(t 'Установка не завершена, убираю созданные файлы GoWave.' 'Installation did not finish, removing the GoWave files it created.')"
    systemctl disable --now gowave >/dev/null 2>&1 || true
    systemctl disable --now gowave-update.path >/dev/null 2>&1 || true
    rm -f "$UNIT_FILE" "${UNIT_FILE}.d/90-unprivileged.conf" "$UPDATE_PATH_UNIT" "$UPDATE_SERVICE_UNIT"
    systemctl daemon-reload >/dev/null 2>&1 || true
    [ "$(readlink "$CLI_PATH" 2>/dev/null)" = "$INSTALL_DIR/gowave-server" ] && rm -f "$CLI_PATH"
    rm -rf "$INSTALL_DIR/gowave-server" "$INSTALL_DIR/frontend" "$INSTALL_DIR/.env" "$INSTALL_DIR"/gowave.db*
    rmdir "$INSTALL_DIR" 2>/dev/null || true
    if [ "$OLD_PANEL_STOPPED" = true ] && [ -n "$OLD_PANEL_POLICIES" ]; then
        local id policy
        while read -r id policy; do
            [ -n "$id" ] || continue
            docker update --restart="${policy:-no}" "$id" >/dev/null 2>&1 || true
            docker start "$id" >/dev/null 2>&1 || true
        done <<<"$OLD_PANEL_POLICIES"
        warn "$(t "Старая панель запущена обратно ($SOURCE_DIR)." "The old panel has been started again ($SOURCE_DIR).")"
    fi
}

on_exit() {
    local status=$?
    if [ "$status" -ne 0 ] && [ "$FRESH_ROLLBACK" = true ]; then
        FRESH_ROLLBACK=false
        rollback_fresh_install
    fi
    rm -rf "$WORK"
}

# print_external_proxy tells the admin where to point their own proxy.
print_external_proxy() {
    printf '%s\n' "${YELLOW}$(t "Reverse proxy не настроен: направьте свой прокси с HTTPS на http://127.0.0.1:$APP_PORT" "Reverse proxy is not configured: point your own HTTPS proxy to http://127.0.0.1:$APP_PORT")${RESET}"
    echo "$(t 'Нужны заголовки Host, X-Forwarded-For, X-Forwarded-Proto, а для SSH-терминала нод - WebSocket (Upgrade, Connection).' \
        'Forward paths unchanged. Pass Host, X-Forwarded-For and X-Forwarded-Proto headers, plus WebSocket (Upgrade, Connection) for the node SSH terminal.')"
    echo "$(t 'Если прокси работает в Docker, ему нужен network_mode: host: панель слушает только 127.0.0.1.' \
        'If the proxy runs in Docker, it needs network_mode: host, because the panel listens on 127.0.0.1 only.')"
    echo "$(t 'Примеры:' 'Examples:') https://aaasazan.github.io/gowave-docs/install/reverse-proxy/overview/"
    echo "$(t 'Пока прокси не настроен, панель доступна только с этого сервера, создать администратора извне нельзя.' \
        'Until the proxy is set up, the panel is reachable only from this server, so the admin cannot be created from outside.')"
}

print_done() {
    printf '\n%s\n' "${GREEN}${BOLD}$(t 'GoWave установлена:' 'GoWave is installed:') https://$DOMAIN$PANEL_BASE_PATH${RESET}"
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
    upstream)
        echo "$(t 'Данные прежней панели перенесены, входите под прежним администратором.' \
            'Upstream panel data has been migrated, log in with your existing admin.')"
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
    if [ "$DB_MODE" = docker ]; then
        echo "$(t 'Консоль:       ' 'Console:       ') gowave rescue"
    else
        echo "$(t 'Консоль:       ' 'Console:       ') cd $INSTALL_DIR && sudo -u gowave gowave rescue"
    fi
    if [ "$SOURCE" != none ]; then
        echo "$(t "Старая панель остановлена и не запускается при загрузке. Её файлы остались в $SOURCE_DIR." \
            "The old panel is stopped and will not start on boot. Its files are still in $SOURCE_DIR.")"
        echo "$(t "Когда убедитесь, что всё работает, удалите её: cd $SOURCE_DIR && docker compose down" \
            "Once you are sure everything works, remove it: cd $SOURCE_DIR && docker compose down")"
    fi
    echo "$(t 'Повторный запуск установщика: обновление, смена домена, удаление.' \
        'Run the installer again to update, change the domain or dashboard path, install a node, or uninstall.')"
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
    local current
    APP_PORT=$(env_get "$ENV_FILE" APP_PORT)
    APP_PORT=${APP_PORT:-3000}
    PANEL_BASE_PATH=$(env_get "$ENV_FILE" PANEL_BASE_PATH)
    PANEL_BASE_PATH=${PANEL_BASE_PATH:-/}
    current=$(installed_channel)
    if [ "$NONINTERACTIVE" = true ] && [ -z "$GOWAVE_CHANNEL" ]; then
        GOWAVE_CHANNEL=$current
        apply_channel
    fi
    choose_channel "$current"
    if [ "$current" = dev ] && [ "$GOWAVE_CHANNEL" = stable ]; then
        warn "$(t 'Переход с dev на stable: stable-релиз может быть старше dev-сборки, а изменения базы данных, сделанные dev-сборкой, не откатываются. Сначала сделайте резервную копию.' \
            'Switching from dev to stable: the stable release may be older than the dev build, and database changes made by the dev build are not rolled back. Make a backup first.')"
        [ "$NONINTERACTIVE" = true ] || confirm "$(t 'Продолжить?' 'Continue?')" || die "$(t 'Отменено.' 'Cancelled.')"
    elif [ "$current" != "$GOWAVE_CHANNEL" ]; then
        info "$(t "Канал: $current -> $GOWAVE_CHANNEL" "Channel: $current -> $GOWAVE_CHANNEL")"
    fi
    if [ "$MODE" = docker ]; then
        # Older compose files published the panel on every interface.
        # shellcheck disable=SC2016
        sed -i 's#^\(\s*- \)"${APP_PORT:-3000}:3000"#\1"127.0.0.1:${APP_PORT:-3000}:3000"#' "$DOCKER_DIR/docker-compose.yml"
        set_docker_image
        info "$(t 'Обновляю образы' 'Updating images')"
        (cd "$DOCKER_DIR" && run docker compose pull && run docker compose up -d --wait --wait-timeout 180)
    else
        install_release
        write_unit
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

# set_docker_image points the Compose project at the chosen channel's image. An
# image from elsewhere (a local build, a mirror) is left alone.
set_docker_image() {
    local image wanted
    image=$(env_get "$ENV_FILE" GOWAVE_IMAGE)
    wanted=$(panel_image)
    case $image in
    '' | "$IMAGE_REPO" | "$IMAGE_REPO":*) ;;
    *)
        warn "$(t "Используется свой образ $image, канал не меняю." "A custom image $image is configured, the channel is left unchanged.")"
        return 0
        ;;
    esac
    [ "$image" = "$wanted" ] && return 0
    env_set "$ENV_FILE" GOWAVE_IMAGE "$wanted"
    info "$(t 'Образ панели:' 'Panel image:') $wanted"
}

panel_version_label() {
    case $GOWAVE_VERSION in
    latest) t 'последний релиз' 'latest release' ;;
    dev) t 'последняя dev-сборка' 'latest dev build' ;;
    *) printf '%s' "$GOWAVE_VERSION" ;;
    esac
}

configure_domain() {
    APP_PORT=$(env_get "$ENV_FILE" APP_PORT)
    APP_PORT=${APP_PORT:-3000}
    PANEL_BASE_PATH=$(env_get "$ENV_FILE" PANEL_BASE_PATH)
    PANEL_BASE_PATH=${PANEL_BASE_PATH:-/}
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
        info "$(t 'Готово:' 'Done:') https://$DOMAIN$PANEL_BASE_PATH"
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
    systemctl disable --now gowave-update.path >/dev/null 2>&1 || true
    rm -f "$UNIT_FILE" "${UNIT_FILE}.d/90-unprivileged.conf" "$UPDATE_PATH_UNIT" "$UPDATE_SERVICE_UNIT"
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
        "$(t 'Изменить путь панели' 'Change dashboard path')" \
        "$(t 'Установить или обновить локальную ноду' 'Install or update the local node')" \
        "$(t 'Перенести уведомления Telegram и вебхуки из старой панели' 'Copy Telegram and webhook notifications from the old panel')" \
        "$(t 'Удалить GoWave' 'Uninstall GoWave')" \
        "$(t 'Выход' 'Exit')") in
    1) update_gowave ;;
    2) configure_domain ;;
    3) configure_panel_path ;;
    4) install_local_node ;;
    5) import_notifications ;;
    6) uninstall_gowave ;;
    7) exit 0 ;;
    esac
}

main() {
    parse_args "$@"
    detect_system
    WORK=$(mktemp -d)
    trap on_exit EXIT
    trap 'line=$LINENO; printf "%s\n" "${RED}[x] $(t "Неожиданная ошибка (строка $line), лог: $LOG" "Unexpected error (line $line), log: $LOG")${RESET}" >&2' ERR
    ensure_tools
    detect_existing
    if [ "$GOWAVE_ACTION" = update ]; then
        [ -n "$MODE" ] || die "$(t 'GoWave не установлена: --update обновляет только существующую установку.' 'GoWave is not installed: --update only updates an existing install.')"
        update_gowave
    elif [ -n "$MODE" ]; then
        manage_existing
    else
        fresh_install
    fi
}

# Sourced by scripts/tests/installer.sh. Under `bash -c "$(curl ...)"` there is no
# source file, and BASH_SOURCE is empty.
if [[ ${BASH_SOURCE[0]:-$0} = "$0" ]]; then main "$@"; fi
