#!/usr/bin/env bash
# Shared helpers for hooks. Source it:  . /opt/deploykit/lib/common.sh
#
# Hooks run as `dk` with the resolved contract environment: DK_DATA_DIR,
# DK_SERVER_DIR, DK_STATE_DIR, DK_MEMORY_MB, DK_HEAP_MB, DK_PORT_*, DK_VAR_*…

set -euo pipefail

dk_log() { printf '[deploykit] %s\n' "$*"; }
dk_die() { printf '[deploykit] error: %s\n' "$*" >&2; exit 1; }

mkdir -p "${DK_STATE_DIR:?}"

# dk_bool VALUE -> prints true/false (accepts true/1/yes/on, any case)
dk_bool() {
    case "${1,,}" in
        true | 1 | yes | on) echo true ;;
        *) echo false ;;
    esac
}

# dk_set_prop FILE KEY VALUE — upsert KEY=VALUE in a key=value file, keeping
# every other line (and the user's edits) as they are. A line break in VALUE
# would add keys of its own, so it is refused.
dk_set_prop() {
    local file=$1 key=$2 value=$3
    [[ $value != *[$'\n\r']* ]] || dk_die "$key: line breaks are not allowed"
    [[ -f $file ]] || : >"$file"
    K="$key" V="$value" awk '
        BEGIN { k = ENVIRON["K"]; v = ENVIRON["V"]; done = 0 }
        !done && index($0, k "=") == 1 { print k "=" v; done = 1; next }
        { print }
        END { if (!done) print k "=" v }
    ' "$file" >"$file.dk-tmp"
    mv "$file.dk-tmp" "$file"
}

# dk_stamp NAME [VALUE] — read, or with VALUE write, a small piece of state
# (e.g. the installed version) under DK_STATE_DIR/stamps.
dk_stamp() {
    local f="$DK_STATE_DIR/stamps/$1"
    if [[ $# -ge 2 ]]; then
        mkdir -p "$(dirname "$f")"
        printf '%s\n' "$2" >"$f"
    else
        cat "$f" 2>/dev/null || true
    fi
}

# dk_fetch URL DEST [ALGO:HASH] — download atomically, verifying the hash
# when one is given (sha1/sha256/sha512).
dk_fetch() {
    local url=$1 dest=$2 want=${3:-}
    mkdir -p "$(dirname "$dest")"
    curl -fsSL --retry 3 --retry-delay 2 -A "deploykithq/deploykit-game-images" -o "$dest.part" "$url" ||
        dk_die "download failed: $url"
    if [[ -n $want ]]; then
        local algo=${want%%:*} hash=${want#*:} got
        got=$("${algo}sum" "$dest.part" | cut -d' ' -f1)
        if [[ ${got,,} != "${hash,,}" ]]; then
            rm -f "$dest.part"
            dk_die "hash mismatch for $url"
        fi
    fi
    mv "$dest.part" "$dest"
}

# dk_get URL — fetch a small API document to stdout.
dk_get() {
    curl -fsSL --retry 3 --retry-delay 2 -A "deploykithq/deploykit-game-images" "$1"
}

# dk_github_api PATH — GET api.github.com/PATH. Anonymous calls are limited to
# 60 an hour per IP, which a shared IP (CI runners, NAT) runs out of; with
# DK_GITHUB_TOKEN set (any token, no scopes needed) the limit is per token.
dk_github_api() {
    local auth=()
    [[ -z ${DK_GITHUB_TOKEN:-} ]] || auth=(-H "Authorization: Bearer $DK_GITHUB_TOKEN")
    curl -fsSL --retry 3 --retry-delay 2 -A "deploykithq/deploykit-game-images" \
        -H "Accept: application/vnd.github+json" "${auth[@]}" "https://api.github.com/$1"
}
