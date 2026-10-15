#!/usr/bin/env bash
# Project Zomboid: ~7 GB from Steam. Saves through the stdin console.
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
SMOKE_MEMORY=6g SMOKE_SEND="/dk save" SMOKE_EXPECT="verb save ok" \
    exec bash "$here/../../../scripts/smoke-run.sh" "${1:?image}" -e DK_VAR_ADMIN_PASSWORD=smoke-admin-pw
