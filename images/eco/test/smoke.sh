#!/usr/bin/env bash
# Eco: ~500 MB from Steam. Runs offline (no server token) and saves through
# the RCON console.
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
SMOKE_MEMORY=6g SMOKE_SEND="/dk save" SMOKE_EXPECT="verb save ok" \
    exec bash "$here/../../../scripts/smoke-run.sh" "${1:?image}"
