#!/usr/bin/env bash
. /opt/deploykit/lib/common.sh
. /opt/deploykit/lib/steam.sh

map=${DK_VAR_MAP:-de_dust2}
[[ $map =~ ^[A-Za-z0-9_]{1,64}$ ]] || dk_die "DK_VAR_MAP: letters, digits and _ only"
players=${DK_VAR_MAX_PLAYERS:-10}
[[ $players =~ ^[0-9]+$ ]] && ((players >= 2 && players <= 64)) || dk_die "DK_VAR_MAX_PLAYERS must be 2-64"
case ${DK_VAR_GAME_MODE:-competitive} in
    competitive) type=0 mode=1 ;;
    casual) type=0 mode=0 ;;
    wingman) type=0 mode=2 ;;
    deathmatch) type=1 mode=2 ;;
    *) dk_die "DK_VAR_GAME_MODE: competitive, casual, wingman or deathmatch" ;;
esac

args=(-dedicated -console -usercon -port "${DK_PORT_GAME:-27015}" -maxplayers "$players"
    +game_type "$type" +game_mode "$mode" +exec dk_server.cfg +map "$map")
gslt=${DK_VAR_GSLT:-}
if [[ -n $gslt ]]; then
    [[ $gslt =~ ^[A-Fa-f0-9]{32}$ ]] || dk_die "DK_VAR_GSLT must be a 32-character token"
    args+=(+sv_setsteamaccount "$gslt")
else
    dk_log "no Game Server Login Token: the server is reachable on LAN only"
    args+=(+sv_lan 1)
fi

dk_steam_sdk_link
cd "$DK_SERVER_DIR/cs2/game/bin/linuxsteamrt64" || exit 1
exec ./cs2 "${args[@]}"
