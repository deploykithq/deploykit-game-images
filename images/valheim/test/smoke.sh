#!/usr/bin/env bash
# Valheim: ~1 GB from Steam. No console: /dk save must report unsupported.
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
SMOKE_MEMORY=4g SMOKE_SEND="/dk save" SMOKE_EXPECT="verb save unsupported" \
    exec bash "$here/../../../scripts/smoke-run.sh" "${1:?image}" -e DK_VAR_SERVER_PASS=smoke-test-pw
