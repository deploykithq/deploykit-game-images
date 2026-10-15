#!/usr/bin/env bash
# Render the managed keys of Zomboid/Server/servertest.ini (rewritten every
# start; every other line is the operator's and is kept) and size the JVM.
#
# Everything the server writes lives under /data/Zomboid (-cachedir): the
# settings in Server/, the world in Saves/Multiplayer/servertest, the player
# database in db/. The save is always "servertest", Project Zomboid's
# default, so a save from another host can be uploaded as is.
. /opt/deploykit/lib/common.sh

zomboid=$DK_DATA_DIR/Zomboid
ini=$zomboid/Server/servertest.ini
mkdir -p "$zomboid/Server" "$zomboid/mods"

name=${DK_VAR_SERVER_NAME:-DeployKit Project Zomboid}
((${#name} >= 1 && ${#name} <= 64)) || dk_die "DK_VAR_SERVER_NAME must be 1-64 characters"
pass=${DK_VAR_SERVER_PASS:-}
((${#pass} <= 64)) || dk_die "DK_VAR_SERVER_PASS must be at most 64 characters"
players=${DK_VAR_MAX_PLAYERS:-16}
[[ $players =~ ^[0-9]+$ ]] && ((players >= 1 && players <= 100)) || dk_die "DK_VAR_MAX_PLAYERS must be 1-100"
game=${DK_PORT_GAME:-16261}
direct=${DK_PORT_DIRECT:-16262}
[[ $game != "$direct" ]] || dk_die "the game and direct ports must differ"

dk_set_prop "$ini" PublicName "$name"
dk_set_prop "$ini" Password "$pass"
dk_set_prop "$ini" MaxPlayers "$players"
dk_set_prop "$ini" PVP "$(dk_bool "${DK_VAR_PVP:-true}")"
dk_set_prop "$ini" Public "$(dk_bool "${DK_VAR_PUBLIC:-false}")"
dk_set_prop "$ini" DefaultPort "$game"
dk_set_prop "$ini" UDPPort "$direct"
# The ports are published as they are; UPnP would try to map them again.
dk_set_prop "$ini" UPnP false
# RCON listens on every interface and the console is stdin: an empty
# password keeps it off.
dk_set_prop "$ini" RCONPassword ""

# The launcher reads its JVM options from ProjectZomboid64.json, which ships
# with -Xmx8g. SteamCMD restores the file on update, so it is set every start.
json=$DK_SERVER_DIR/pz/ProjectZomboid64.json
heap=${DK_HEAP_MB:-4096}
[[ $heap =~ ^[0-9]+$ ]] || dk_die "DK_HEAP_MB must be a number"
jq --arg xmx "-Xmx${heap}m" --arg xms "-Xms${heap}m" '
    .vmArgs = ([.vmArgs[] | select(startswith("-Xmx") or startswith("-Xms") | not)] + [$xms, $xmx])
' "$json" >"$json.dk-tmp" || dk_die "ProjectZomboid64.json is not valid JSON"
mv "$json.dk-tmp" "$json"
dk_log "jvm heap ${heap} MB"
