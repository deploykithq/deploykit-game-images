#!/usr/bin/env bash
. /opt/deploykit/lib/common.sh
. /opt/deploykit/lib/steam.sh

name=${DK_VAR_SERVER_NAME:-DeployKit Valheim}
world=${DK_VAR_WORLD_NAME:-Dedicated}
pass=${DK_VAR_SERVER_PASS:-}
port=${DK_PORT_GAME:-2456}

[[ $world =~ ^[A-Za-z0-9_-]{1,40}$ ]] || dk_die "DK_VAR_WORLD_NAME: letters, digits, - and _ only"
# Valheim refuses both of these at start, with an error far less obvious.
((${#pass} >= 5)) || dk_die "DK_VAR_SERVER_PASS must be at least 5 characters"
[[ $name != *"$pass"* ]] || dk_die "DK_VAR_SERVER_PASS must not appear in the server name"
public=0
[[ $(dk_bool "${DK_VAR_SERVER_PUBLIC:-true}") == true ]] && public=1

dk_steam_sdk_link
cd "$DK_SERVER_DIR/valheim" || exit 1
export LD_LIBRARY_PATH=./linux64${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}
export SteamAppId=892970
exec ./valheim_server.x86_64 -nographics -batchmode \
    -name "$name" -port "$port" -world "$world" -password "$pass" -public "$public" \
    -savedir "$DK_DATA_DIR"
