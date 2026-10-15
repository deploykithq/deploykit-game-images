#!/usr/bin/env bash
# Eco only runs with its install directory as the working directory, and
# keeps its config, world and mods inside it. The game files stay in
# /server/eco, where SteamCMD owns them; the server runs in /server/run, a
# tree of links rebuilt every start in which only what is the operator's
# points into /data:
#   /data/Configs        *.eco (the *.eco.template files link to the install)
#   /data/Storage        the world, and Eco's own Storage/Backup
#   /data/Mods/UserCode  the operator's mods
#   /data/Logs           Eco's log files
. /opt/deploykit/lib/common.sh

eco=$DK_SERVER_DIR/eco
run=$DK_SERVER_DIR/run
mkdir -p "$DK_DATA_DIR"/{Configs,Storage,Logs,Mods/UserCode}

# Links never followed by rm: this removes the tree, not what it points at.
rm -rf "$run"
mkdir -p "$run/Mods"
for entry in "$eco"/*; do
    name=${entry##*/}
    case $name in
        Configs | Storage | Logs | Mods | steamapps) ;;
        *) ln -s "$entry" "$run/$name" ;;
    esac
done
for name in Configs Storage Logs; do
    ln -s "$DK_DATA_DIR/$name" "$run/$name"
done
for entry in "$eco"/Mods/*; do
    [[ ${entry##*/} == UserCode ]] || ln -s "$entry" "$run/Mods/${entry##*/}"
done
ln -s "$DK_DATA_DIR/Mods/UserCode" "$run/Mods/UserCode"

# Templates follow the installed version; links to removed ones are dropped.
find "$DK_DATA_DIR/Configs" -maxdepth 1 -type l -name '*.template' -delete
for t in "$eco"/Configs/*.template; do
    ln -s "$t" "$DK_DATA_DIR/Configs/${t##*/}"
done

name=${DK_VAR_SERVER_NAME:-DeployKit Eco}
((${#name} >= 1 && ${#name} <= 64)) || dk_die "DK_VAR_SERVER_NAME must be 1-64 characters"
description=${DK_VAR_DESCRIPTION:-}
((${#description} <= 500)) || dk_die "DK_VAR_DESCRIPTION must be at most 500 characters"
password=${DK_VAR_PASSWORD:-}
((${#password} <= 64)) || dk_die "DK_VAR_PASSWORD must be at most 64 characters"
public=$(dk_bool "${DK_VAR_PUBLIC:-false}")
# The published ports are whatever the host allocated; Eco's own internal
# ones (RCON, Steam) sit where an allocation is unlikely to land.
for p in "${DK_PORT_GAME:-3000}" "${DK_PORT_WEB:-3001}"; do
    [[ $p != 47002 && $p != 47003 ]] || dk_die "ports 47002 and 47003 are used inside the container; pick another"
done

# The keys below are managed (rewritten every start); every other setting in
# Network.eco is the operator's and is kept. RCON listens on loopback only:
# the supervisor is its one client.
net=$DK_DATA_DIR/Configs/Network.eco
[[ -f $net ]] || cp "$eco/Configs/Network.eco.template" "$net"
umask 077
jq --arg name "$name" --arg desc "$description" --arg pass "$password" \
    --argjson public "$public" \
    --argjson game "${DK_PORT_GAME:-3000}" --argjson web "${DK_PORT_WEB:-3001}" \
    --arg rcon "${DK_RCON_PASSWORD:?}" '
    .Name = $name
    | .DetailedDescription = $desc
    | .Password = $pass
    | .PublicServer = $public
    | .GameServerPort = $game
    | .WebServerPort = $web
    | .RconServerPort = 47002
    | .SteamServerPort = 47003
    | .RconIPAddress = "127.0.0.1"
    | .RconPassword = $rcon
    | .UPnPEnabled = false
' "$net" >"$net.dk-tmp" || dk_die "Configs/Network.eco is not valid JSON"
mv "$net.dk-tmp" "$net"
