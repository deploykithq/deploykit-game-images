#!/usr/bin/env bash
# Minecraft: vanilla LATEST (needs the java25 image), `list` on the console.
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
SMOKE_SEND="list" SMOKE_EXPECT="There are 0" \
    exec bash "$here/../../../scripts/smoke-run.sh" "${1:?image}" \
    -e DK_VAR_EULA=true -e DK_VAR_TYPE="${MC_TYPE:-vanilla}" -e DK_GAME_VERSION="${MC_VERSION:-LATEST}"
