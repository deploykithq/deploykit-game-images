#!/usr/bin/env bash
# Terraria: small world, `playing` answers on the stdin console.
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
SMOKE_SEND="playing" SMOKE_EXPECT="No players connected|players? connected" \
    exec bash "$here/../../../scripts/smoke-run.sh" "${1:?image}" -e DK_VAR_WORLD_SIZE=1
