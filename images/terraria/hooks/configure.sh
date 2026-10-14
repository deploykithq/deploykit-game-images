#!/usr/bin/env bash
# Render serverconfig.txt. The keys below are managed (rewritten every start);
# anything else in the file is the user's and is kept.
. /opt/deploykit/lib/common.sh

cfg=$DK_DATA_DIR/serverconfig.txt
worlds=$DK_DATA_DIR/worlds
mkdir -p "$worlds"

world=${DK_VAR_WORLD_NAME:-World}
[[ $world =~ ^[A-Za-z0-9_-]{1,40}$ ]] || dk_die "DK_VAR_WORLD_NAME: letters, digits, - and _ only"
size=${DK_VAR_WORLD_SIZE:-2}
[[ $size =~ ^[123]$ ]] || dk_die "DK_VAR_WORLD_SIZE must be 1, 2 or 3"
difficulty=${DK_VAR_DIFFICULTY:-0}
[[ $difficulty =~ ^[0-3]$ ]] || dk_die "DK_VAR_DIFFICULTY must be 0-3"
players=${DK_VAR_MAX_PLAYERS:-8}
[[ $players =~ ^[0-9]+$ ]] && ((players >= 1 && players <= 255)) || dk_die "DK_VAR_MAX_PLAYERS must be 1-255"

dk_set_prop "$cfg" world "$worlds/$world.wld"
dk_set_prop "$cfg" worldpath "$worlds"
dk_set_prop "$cfg" worldname "$world"
dk_set_prop "$cfg" autocreate "$size"
dk_set_prop "$cfg" difficulty "$difficulty"
dk_set_prop "$cfg" maxplayers "$players"
dk_set_prop "$cfg" port "${DK_PORT_GAME:-7777}"
dk_set_prop "$cfg" password "${DK_VAR_PASSWORD:-}"
[[ -n ${DK_VAR_MOTD:-} ]] && dk_set_prop "$cfg" motd "$DK_VAR_MOTD"
[[ -n ${DK_VAR_SEED:-} ]] && dk_set_prop "$cfg" seed "$DK_VAR_SEED"
exit 0
