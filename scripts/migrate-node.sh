#!/usr/bin/env bash
# Use the node's maintained installer for checksum validation, rollback, and
# unprivileged service migration on both fresh installs and updates.
set -euo pipefail

installer=$(mktemp)
trap 'rm -f -- "$installer"' EXIT
curl -fsSL https://raw.githubusercontent.com/aaaSaZaN/remnanode-go/main/migrate.sh -o "$installer"
bash "$installer" "$@"
