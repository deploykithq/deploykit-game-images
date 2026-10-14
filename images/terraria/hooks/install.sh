#!/usr/bin/env bash
# Install the Terraria dedicated server into DK_SERVER_DIR/terraria.
. /opt/deploykit/lib/common.sh

version=${DK_GAME_VERSION:-$TERRARIA_DEFAULT_VERSION}
[[ $version =~ ^1\.[0-9]\.[0-9]\.[0-9]{1,2}$ ]] || dk_die "DK_GAME_VERSION must look like 1.4.5.8, got '$version'"
code=${version//./}
dir=$DK_SERVER_DIR/terraria

if [[ $(dk_stamp terraria-version) == "$version" && -x $dir/TerrariaServer.bin.x86_64 ]]; then
    dk_log "terraria $version already installed"
    exit 0
fi

dk_log "installing terraria $version"
hash=
[[ $version == "$TERRARIA_DEFAULT_VERSION" ]] && hash="sha256:$TERRARIA_DEFAULT_SHA256"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
dk_fetch "https://terraria.org/api/download/pc-dedicated-server/terraria-server-$code.zip" "$tmp/server.zip" "$hash"
unzip -q "$tmp/server.zip" "$code/Linux/*" -d "$tmp"

rm -rf "$dir"
mkdir -p "$dir"
cp -a "$tmp/$code/Linux/." "$dir/"
chmod 0755 "$dir/TerrariaServer.bin.x86_64" "$dir/TerrariaServer" 2>/dev/null || true
dk_stamp terraria-version "$version"
