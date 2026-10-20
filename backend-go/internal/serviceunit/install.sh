#!/usr/bin/env bash
# Embedded in the binary: used by installers and the first upgraded root service.
set -euo pipefail
install_dir=$1
unit_dir=${2:-/etc/systemd/system}
install_dir=$(realpath -m -- "$install_dir")
[ "$(id -u)" -eq 0 ] || { echo 'Service installation requires root.' >&2; exit 1; }
case "$install_dir" in
    /*) ;;
    *) echo 'Installation path must be absolute.' >&2; exit 1 ;;
esac
# systemd paths have their own escaping rules. Refuse ambiguous paths.
case "$install_dir" in *[!a-zA-Z0-9_./-]*) echo 'Unsupported installation path.' >&2; exit 1 ;; esac
case "$install_dir" in
    /|/bin|/sbin|/usr|/usr/bin|/usr/sbin|/usr/local|/usr/local/bin|/etc|/var|/var/lib|/tmp|/opt|/srv|/home|/home/*|/root|/root/*)
        echo 'Use a dedicated installation directory outside home directories.' >&2; exit 1 ;;
esac
mkdir -p "$install_dir"
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
# Persistent ownership is required for self-updates and private database/runtime files.
chown -hR gowave:gowave "$install_dir"
chmod 0750 "$install_dir"
[ ! -L "$install_dir/.env" ] || { echo "Refusing a symlinked .env" >&2; exit 1; }
[ ! -f "$install_dir/.env" ] || chmod 0600 "$install_dir/.env"
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
systemctl daemon-reload
