#!/bin/sh
# Use the node's maintained installer for checksum validation, rollback, and
# unprivileged service migration on both fresh installs and updates.
set -eu

installer=$(mktemp)
trap 'rm -f -- "$installer"' EXIT
curl -fsSL https://raw.githubusercontent.com/aaaSaZaN/gowave-node/main/install.sh -o "$installer"
sh "$installer" "$@"
