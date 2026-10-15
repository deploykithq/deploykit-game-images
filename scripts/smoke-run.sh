#!/usr/bin/env bash
# Smoke-test a game image under the exact constraints DeployKit runs it with
# (apps/api/src/lib/game-isolation.ts GAME_HARDENING, no TTY, OpenStdin):
#
#   scripts/smoke-run.sh IMAGE [docker run args...]
#
# Env: SMOKE_TIMEOUT   seconds to wait for "[deploykit] ready" (default 900)
#      SMOKE_SEND      a console line to send once ready
#      SMOKE_EXPECT    a regex the output must then match
#      SMOKE_RESTART   1 (default) starts the server a second time on the same
#                      volumes: install must be skipped and the world must load
#      SMOKE_KEEP      1 keeps the volumes afterwards
#      SMOKE_VERBOSE   1 prints the full output of each pass
#
# The server starts twice, so a successful run proves: install, readiness, the
# console, a clean stop (exit 0) and that the saved state reloads.
set -euo pipefail

image=${1:?usage: smoke-run.sh IMAGE [docker run args...]}
shift
timeout=${SMOKE_TIMEOUT:-900}
name=dk-smoke-$$
work=$(mktemp -d)
log=$work/log

hardening=(
    --security-opt no-new-privileges:true
    --cap-drop NET_RAW --cap-drop MKNOD --cap-drop AUDIT_WRITE
    --cap-drop SYS_CHROOT --cap-drop SETFCAP --cap-drop SETPCAP
    --pids-limit 2048
    --memory "${SMOKE_MEMORY:-3g}" --memory-swap "${SMOKE_MEMORY:-3g}"
)
# Passed by name, so the token never shows up in a process listing.
[[ -z ${DK_GITHUB_TOKEN:-} ]] || hardening+=(-e DK_GITHUB_TOKEN)

cleanup() {
    exec 3>&- 2>/dev/null || true
    docker rm -f "$name" >/dev/null 2>&1 || true
    if [[ ${SMOKE_KEEP:-0} != 1 ]]; then
        docker volume rm -f "$name-data" "$name-server" >/dev/null 2>&1 || true
    fi
    rm -rf "$work"
}
trap cleanup EXIT

fail() {
    echo "--- container output ---"
    tail -n 80 "$log" || true
    echo "SMOKE FAIL: $*" >&2
    exit 1
}

wait_for() { # REGEX SECONDS
    local deadline=$((SECONDS + $2))
    while ((SECONDS < deadline)); do
        grep -Eq -- "$1" "$log" && return 0
        kill -0 "$runner" 2>/dev/null || fail "container exited while waiting for /$1/"
        sleep 1
    done
    fail "timed out after $2s waiting for /$1/"
}

run_once() { # PASS [docker run args...]
    local pass=$1
    shift
    : >"$log"
    mkfifo "$work/in$pass"
    docker run --rm -i --name "$name" \
        -v "$name-data:/data" -v "$name-server:/server" \
        "${hardening[@]}" "$@" "$image" <"$work/in$pass" >"$log" 2>&1 &
    runner=$!
    exec 3>"$work/in$pass"

    local started=$SECONDS
    wait_for '^\[deploykit\] ready$' "$timeout"
    echo "pass $pass: ready after $((SECONDS - started))s"

    [[ $(docker exec "$name" ps -o uid= -p 1 | tr -d " ") == 1000 ]] || fail "PID 1 is not running as uid 1000"
    docker exec "$name" dk-entrypoint health || fail "health check failed while ready"

    echo "/dk status" >&3
    wait_for 'state=running ready=true' 30
    if [[ -n ${SMOKE_SEND:-} ]]; then
        echo "$SMOKE_SEND" >&3
        wait_for "${SMOKE_EXPECT:-.}" 60
    fi

    docker stop -t 120 "$name" >/dev/null
    local code=0
    wait "$runner" || code=$?
    exec 3>&-
    [[ $code == 0 ]] || fail "container exited with $code after stop"
    grep -q '^\[deploykit\] stopped code=0$' "$log" || fail "no clean 'stopped code=0'"
    [[ ${SMOKE_VERBOSE:-0} != 1 ]] || sed "s/^/  | /" "$log"
    cp "$log" "$work/pass$pass.log"
}

run_once 1 "$@"
if [[ ${SMOKE_RESTART:-1} == 1 ]]; then
    run_once 2 "$@"
    grep -q 'already installed' "$log" || fail "second start reinstalled the server"
fi
echo "SMOKE OK: $image"
