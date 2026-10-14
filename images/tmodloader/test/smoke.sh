#!/usr/bin/env bash
# tModLoader: two Workshop items (Magic Storage, Recipe Browser), one enabled.
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
SMOKE_SEND="playing" SMOKE_EXPECT="No players connected|players? connected" \
    exec bash "$here/../../../scripts/smoke-run.sh" "${1:?image}" \
    -e DK_VAR_WORLD_SIZE=1 -e DK_MODS_STEAM=2563309347,2619954303 -e DK_MODS_STEAM_ENABLED=2619954303
