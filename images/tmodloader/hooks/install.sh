#!/usr/bin/env bash
# Install tModLoader (DK_GAME_VERSION: a release tag such as v2026.08.3.0, or
# "latest") into DK_SERVER_DIR/tmodloader.
. /opt/deploykit/lib/common.sh

repo=https://github.com/tModLoader/tModLoader
dir=$DK_SERVER_DIR/tmodloader
tag=${DK_GAME_VERSION:-latest}

if [[ ${tag,,} == latest ]]; then
    tag=$(curl -fsSI "$repo/releases/latest" | tr -d '\r' | awk -F/ 'tolower($1) ~ /^location:/ {print $NF}')
    [[ -n $tag ]] || dk_die "could not resolve the latest tModLoader release"
fi
[[ $tag =~ ^v[0-9]{4}\.[0-9]{2}\.[0-9]+\.[0-9]+$ ]] || dk_die "DK_GAME_VERSION must be a tModLoader tag like v2026.08.3.0, got '$tag'"

if [[ $(dk_stamp tmodloader-version) == "$tag" && -f $dir/tModLoader.dll ]]; then
    dk_log "tmodloader $tag already installed"
    exit 0
fi

dk_log "installing tmodloader $tag"
# GitHub publishes a sha256 digest for release assets; verify against it when
# the API answers (it is rate limited per IP).
digest=$(dk_get "https://api.github.com/repos/tModLoader/tModLoader/releases/tags/$tag" 2>/dev/null |
    jq -r '.assets[] | select(.name == "tModLoader.zip") | .digest // empty' || true)
[[ -n $digest ]] || dk_log "warning: no published digest for $tag; downloading without hash verification"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
dk_fetch "$repo/releases/download/$tag/tModLoader.zip" "$tmp/tModLoader.zip" "$digest"
rm -rf "$dir"
mkdir -p "$dir"
unzip -q "$tmp/tModLoader.zip" -d "$dir"
dk_stamp tmodloader-version "$tag"
