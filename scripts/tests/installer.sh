#!/usr/bin/env bash
# Isolated installer behavior tests. No root privileges or host services required.
set -Eeuo pipefail
ROOT=$(cd "$(dirname "$0")/../.." && pwd)
source "$ROOT/install.sh"
WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT
REAL_VERIFY=$(declare -f verify_panel_path)
UI_LANG=en
LOG=$WORK/install.log
ENV_FILE=$WORK/.env
MODE=native
INSTALL_DIR=$WORK/panel
mkdir -p "$INSTALL_DIR"
fail() { echo "FAIL: $*" >&2; exit 1; }

for value in /api /api/sub /metrics /a/../b //evil /bad?x /a//b; do
    if normalize_panel_path "$value" >/dev/null; then fail "accepted $value"; fi
done
[ "$(normalize_panel_path /private/panel)" = /private/panel/ ] || fail normalization

# Failed reconfiguration restores the entire original environment and restarts it.
printf 'APP_PORT=3000\nPANEL_DOMAIN=panel.example\nAPP_SECRET=preserved\n' >"$ENV_FILE"
cp "$ENV_FILE" "$WORK/original"
ask() { printf /new; }
RESTART_COUNT=0
restart_panel() { RESTART_COUNT=$((RESTART_COUNT+1)); }
verify_panel_path() { [ "$PANEL_BASE_PATH" = / ]; }
if configure_panel_path; then fail 'failed change reported success'; fi
cmp "$ENV_FILE" "$WORK/original" || fail 'configuration not restored'
[ "$RESTART_COUNT" = 2 ] || fail 'rollback did not restart'

# A successful change updates only the dashboard path.
verify_panel_path() { [ "$PANEL_BASE_PATH" = /new/ ]; }
configure_panel_path
[ "$(env_get "$ENV_FILE" PANEL_BASE_PATH)" = /new/ ] || fail 'path not saved'
[ "$(env_get "$ENV_FILE" APP_SECRET)" = preserved ] || fail 'secret changed'

# Download and execution stubs never access the network or host services.
ask() { printf 2222; }
panel_node_secret() { printf c2VjcmV0; }
curl() { return 1; }
install_local_node || fail 'download failure affected panel'

curl() {
    local destination
    while [ "$#" -gt 0 ]; do
        if [ "$1" = -o ]; then destination=$2; break; fi
        shift
    done
    cat >"$destination" <<'NODE'
[ "$SECRET_KEY" = c2VjcmV0 ] || exit 8
[ "$NODE_PORT" = 2222 ] || exit 9
exit 7
NODE
}
install_local_node || fail 'node failure affected panel'
[ -z "${secret+x}" ] || fail 'secret leaked from function'

# Successful install passes credentials and chosen port only to the child.
curl() {
    local destination
    while [ "$#" -gt 0 ]; do
        if [ "$1" = -o ]; then destination=$2; break; fi
        shift
    done
    cat >"$destination" <<'NODE'
[ "$SECRET_KEY" = c2VjcmV0 ] && [ "$NODE_PORT" = 2222 ]
NODE
}
install_local_node || fail 'fresh install failed'
# Credential failures do not execute the downloaded installer.
panel_node_secret() { return 1; }
curl() { fail 'download attempted without credentials'; }
install_local_node || fail 'credential failure affected panel'
# Invalid ports fail before credential generation.
ask() { printf 70000; }
panel_node_secret() { fail 'credentials requested for invalid port'; }
install_local_node
! grep -q c2VjcmV0 "$LOG" 2>/dev/null || fail 'credentials logged'
# Old bundles lack the new marker but must still pass rollback verification.
eval "$REAL_VERIFY"
APP_PORT=3000
PANEL_BASE_PATH=/
wait_healthy() { return 0; }
curl() { printf '<!doctype html><html>old dashboard</html>' >"$WORK/dashboard-check.html"; }
if verify_panel_path; then fail 'old bundle accepted for new configuration'; fi
verify_panel_path --legacy || fail 'old bundle rejected during recovery'

# Channel options: flags, environment and pinned versions resolve consistently.
reset_channel() { GOWAVE_CHANNEL=''; GOWAVE_VERSION=latest; GOWAVE_ACTION=''; GOWAVE_LANG=''; }
reset_channel; parse_args --dev
[ "$GOWAVE_CHANNEL/$GOWAVE_VERSION" = dev/dev ] || fail '--dev'
[ "$(panel_image)" = ghcr.io/aaasazan/gowave:dev ] || fail 'dev image'
reset_channel; parse_args --channel=stable --update --lang en
[ "$GOWAVE_CHANNEL/$GOWAVE_VERSION/$GOWAVE_ACTION/$GOWAVE_LANG" = stable/latest/update/en ] || fail '--channel=stable --update'
reset_channel; GOWAVE_VERSION=v1.2.3; parse_args
[ "$GOWAVE_CHANNEL" = stable ] && [ "$(panel_image)" = ghcr.io/aaasazan/gowave:1.2.3 ] || fail 'pinned version'
reset_channel; GOWAVE_VERSION=dev; parse_args
[ "$GOWAVE_CHANNEL" = dev ] || fail 'GOWAVE_VERSION=dev'
for args in '--dev --version v1.2.3' '--channel nightly' '--version 1.2' '--lang de' '--bogus' '--channel'; do
    reset_channel
    # shellcheck disable=SC2086
    if (parse_args $args) 2>/dev/null; then fail "accepted $args"; fi
done
reset_channel; GOWAVE_VERSION='v1;rm'
if (parse_args) 2>/dev/null; then fail 'accepted an unsafe version'; fi

# Enter keeps the installed channel; an explicit option skips the question.
ask() { printf ''; }
reset_channel; choose_channel dev 2>/dev/null
[ "$GOWAVE_CHANNEL/$GOWAVE_VERSION" = dev/dev ] || fail 'Enter switched away from dev'
reset_channel; choose_channel 2>/dev/null
[ "$GOWAVE_CHANNEL/$GOWAVE_VERSION" = stable/latest ] || fail 'fresh install default'
ask() { printf 2; }
reset_channel; choose_channel dev 2>/dev/null
[ "$GOWAVE_CHANNEL/$GOWAVE_VERSION" = stable/latest ] || fail 'switch from dev to stable'
ask() { fail 'asked although the channel was given'; }
reset_channel; GOWAVE_CHANNEL=dev; choose_channel stable

# Docker: the channel is read from and written to GOWAVE_IMAGE; custom images stay.
MODE=docker
printf 'GOWAVE_IMAGE=ghcr.io/aaasazan/gowave:latest\n' >"$ENV_FILE"
[ "$(installed_channel)" = stable ] || fail 'docker stable channel'
reset_channel; GOWAVE_CHANNEL=dev; apply_channel; set_docker_image >/dev/null
[ "$(env_get "$ENV_FILE" GOWAVE_IMAGE)" = ghcr.io/aaasazan/gowave:dev ] || fail 'docker switch to dev'
[ "$(installed_channel)" = dev ] || fail 'docker dev channel'
printf 'GOWAVE_IMAGE=registry.local/gowave:custom\n' >"$ENV_FILE"
set_docker_image 2>/dev/null
[ "$(env_get "$ENV_FILE" GOWAVE_IMAGE)" = registry.local/gowave:custom ] || fail 'custom image replaced'
MODE=native

echo 'Installer behavior checks passed.'
