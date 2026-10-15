#!/usr/bin/env bash
# Prepares the reference upstream backend used by scripts/subscription-diff:
#   scripts/subscription-diff/setup-upstream.sh <work-dir>
# It extracts remnawave/backend at the pinned commit from a local clone
# (UPSTREAM_REPO, default ~/remnawave-backend), installs its locked
# dependencies, generates the Prisma client and downloads Node 24.21 (the
# version upstream's Dockerfile uses; Node 26 breaks its x25519 helper).
set -euo pipefail
WORK=${1:?work directory required}
REPO=${UPSTREAM_REPO:-$HOME/remnawave-backend}
COMMIT=010b365ab1fabea01192b5e6ade4e98e66ee1dbd
NODE_VERSION=v24.21.0
mkdir -p "$WORK"
cd "$WORK"
if [ ! -x "node-$NODE_VERSION-linux-x64/bin/node" ]; then
    curl -fsSLO "https://nodejs.org/dist/$NODE_VERSION/SHASUMS256.txt"
    curl -fsSLO "https://nodejs.org/dist/$NODE_VERSION/node-$NODE_VERSION-linux-x64.tar.xz"
    grep " node-$NODE_VERSION-linux-x64.tar.xz\$" SHASUMS256.txt | sha256sum -c -
    tar -xJf "node-$NODE_VERSION-linux-x64.tar.xz"
fi
export PATH="$WORK/node-$NODE_VERSION-linux-x64/bin:$PATH"
if [ ! -d upstream/node_modules ]; then
    git -C "$REPO" cat-file -e "$COMMIT" 2>/dev/null || git -C "$REPO" fetch --quiet origin
    rm -rf upstream && mkdir upstream
    git -C "$REPO" archive "$COMMIT" | tar -x -C upstream
    (cd upstream && npm ci --ignore-scripts --no-audit --no-fund)
    (cd upstream && PATH="$PWD/node_modules/.bin:$PATH" node node_modules/prisma/build/index.js generate --no-engine >/dev/null)
fi
echo "UPSTREAM_DIR=$WORK/upstream"
echo "NODE=$WORK/node-$NODE_VERSION-linux-x64/bin/node"
