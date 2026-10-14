#!/usr/bin/env bash
# CS2: downloads ~60 GB from Steam — run on a runner with the disk for it.
# `status` goes over the container-local RCON.
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
SMOKE_MEMORY=6g SMOKE_TIMEOUT=7200 SMOKE_SEND="status" SMOKE_EXPECT="hostname|map" \
    exec bash "$here/../../../scripts/smoke-run.sh" "${1:?image}" -e DK_VAR_SERVER_NAME=smoke
