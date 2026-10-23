#!/usr/bin/env bash
# Embedded in the binary: used by installers, update helper and the first upgraded root service.
#
# Privilege boundary: the panel runs as "gowave", but everything root executes or
# serves (gowave-server, frontend/) is owned by root, so a compromised panel can
# neither plant code that root later runs nor persist in the dashboard files.
# The service account may only create its own data files: the installation
# directory is root:gowave with the sticky bit, which keeps root's files from
# being renamed or removed by gowave.
set -euo pipefail
install_dir=$1
unit_dir=${2:-/etc/systemd/system}
[ "$(id -u)" -eq 0 ] || { echo 'Service installation requires root.' >&2; exit 1; }
case "$install_dir" in
    /*) ;;
    *) echo 'Installation path must be absolute.' >&2; exit 1 ;;
esac
# systemd paths have their own escaping rules. Refuse ambiguous paths.
case "$install_dir" in *[!a-zA-Z0-9_./-]*) echo 'Unsupported installation path.' >&2; exit 1 ;; esac
case "/$install_dir/" in */./*|*/../*) echo 'Installation path must be normalized.' >&2; exit 1 ;; esac
check_dir() {
    case "${1%/}" in
        ''|/bin|/sbin|/usr|/usr/bin|/usr/sbin|/usr/local|/usr/local/bin|/etc|/var|/var/lib|/tmp|/opt|/srv|/home|/home/*|/root|/root/*)
            echo 'Use a dedicated installation directory outside home directories.' >&2; exit 1 ;;
    esac
}
check_dir "$install_dir"
mkdir -p "$install_dir"
# Validate again after resolving symlinks in the path.
install_dir=$(cd "$install_dir" && pwd -P)
check_dir "$install_dir"
if [ -n "${3:-}" ] && [ "$3" != "$install_dir/gowave-server" ]; then
    install -m 0755 "$3" "$install_dir/gowave-server.new"
    mv -f "$install_dir/gowave-server.new" "$install_dir/gowave-server"
fi
if ! getent group gowave >/dev/null; then
    groupadd --system gowave
fi
if ! id gowave >/dev/null 2>&1; then
    useradd --system --gid gowave --home-dir "$install_dir" --no-create-home --shell /usr/sbin/nologin gowave
fi
[ "$(id -u gowave)" -ne 0 ] || { echo 'Refusing a service account with UID 0.' >&2; exit 1; }

chown root:gowave "$install_dir"
chmod 1770 "$install_dir"
# Code and dashboard files: root-owned, not writable by the service account.
# Leftovers of earlier in-panel updates are removed rather than trusted.
rm -f -- "$install_dir/gowave-server.new"
for name in gowave-server gowave-server.bak frontend dist static; do
    path=$install_dir/$name
    if [ -L "$path" ]; then
        echo "Refusing a symlinked $path" >&2; exit 1
    fi
    [ -e "$path" ] || continue
    if [ -d "$path" ]; then
        find "$path" -type l -delete
    fi
    chown -hR root:root "$path"
    find "$path" -exec chmod go-w {} +
done
[ ! -L "$install_dir/.env" ] || { echo "Refusing a symlinked .env" >&2; exit 1; }
if [ -f "$install_dir/.env" ]; then
    chown root:gowave "$install_dir/.env"
    chmod 0640 "$install_dir/.env"
fi
# SQLite files may have been created by root (installer imports, rescue commands).
find "$install_dir" -maxdepth 1 -type f \( -name '*.db' -o -name '*.db-*' -o -name '*.sqlite' -o -name '*.sqlite3' -o -name '*.sqlite-*' -o -name '*.sqlite3-*' \) \
    -exec chown gowave:gowave {} +

mkdir -p "$unit_dir/gowave.service.d"
policy=$(mktemp "$unit_dir/gowave.service.d/.hardening.XXXXXX")
trap 'rm -f -- "$policy"' EXIT
cat > "$policy" <<UNIT
[Service]
User=gowave
Group=gowave
WorkingDirectory=$install_dir
ExecStart=
ExecStart=$install_dir/gowave-server
DynamicUser=false
ProtectSystem=strict
ProtectHome=true
PrivateDevices=true
PrivateTmp=true
NoNewPrivileges=true
MemoryDenyWriteExecute=true
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
RestrictSUIDSGID=true
LockPersonality=true
UMask=0077
ReadWritePaths=$install_dir
RuntimeDirectory=gowave
RuntimeDirectoryMode=0700
RuntimeDirectoryPreserve=yes
Restart=always
CapabilityBoundingSet=
CapabilityBoundingSet=CAP_NET_BIND_SERVICE
AmbientCapabilities=
AmbientCapabilities=CAP_NET_BIND_SERVICE
UNIT
chmod 0644 "$policy"
mv -f "$policy" "$unit_dir/gowave.service.d/90-unprivileged.conf"
trap - EXIT

# In-panel updates: the panel leaves a request in its runtime directory and this
# root oneshot installs the latest release after checking SHA256SUMS.
cat > "$unit_dir/gowave-update.service" <<UNIT
[Unit]
Description=GoWave panel update from a verified release
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
ExecStart=$install_dir/gowave-server update apply $install_dir
TimeoutStartSec=15min
UNIT
cat > "$unit_dir/gowave-update.path" <<UNIT
[Unit]
Description=Watch for GoWave panel update requests

[Path]
PathExists=/run/gowave/update-request
Unit=gowave-update.service

[Install]
WantedBy=multi-user.target
UNIT
chmod 0644 "$unit_dir/gowave-update.service" "$unit_dir/gowave-update.path"
systemctl daemon-reload
systemctl enable --now gowave-update.path
