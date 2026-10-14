#!/usr/bin/env bash
# Render server.properties. Managed keys are rewritten on every start; every
# other property the user edits is kept.
. /opt/deploykit/lib/common.sh

props=$DK_DATA_DIR/server.properties

players=${DK_VAR_MAX_PLAYERS:-20}
[[ $players =~ ^[0-9]+$ ]] || dk_die "DK_VAR_MAX_PLAYERS must be a number"
difficulty=${DK_VAR_DIFFICULTY:-normal}
case $difficulty in peaceful | easy | normal | hard) ;; *) dk_die "DK_VAR_DIFFICULTY: peaceful, easy, normal or hard" ;; esac

dk_set_prop "$props" server-port "${DK_PORT_GAME:-25565}"
dk_set_prop "$props" max-players "$players"
dk_set_prop "$props" difficulty "$difficulty"
dk_set_prop "$props" online-mode "$(dk_bool "${DK_VAR_ONLINE_MODE:-true}")"
dk_set_prop "$props" motd "${DK_VAR_MOTD:-A Minecraft server on DeployKit}"
# The console is the server's own stdin; RCON and query stay closed.
dk_set_prop "$props" enable-rcon false
dk_set_prop "$props" enable-query false
# Only meaningful before the world exists, so never overwrite it with blank.
[[ -n ${DK_VAR_SEED:-} ]] && dk_set_prop "$props" level-seed "$DK_VAR_SEED"
exit 0
