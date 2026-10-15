#!/usr/bin/env bash
# SteamCMD helpers. Source after common.sh:  . /opt/deploykit/lib/steam.sh

STEAMCMD=/opt/steamcmd/steamcmd.sh

# dk_steam_app_update APP_ID DIR [validate] [BRANCH] — install or update a
# dedicated server anonymously, from BRANCH when one is given (empty: the
# default "public" branch). SteamCMD fails transiently (state 0x202, 0x602…),
# so it is retried a few times before giving up.
dk_steam_app_update() {
    local app=$1 dir=$2 validate=${3:-} branch=${4:-}
    local attempt
    [[ -z $branch || $branch =~ ^[A-Za-z0-9._-]+$ ]] || dk_die "invalid Steam branch: $branch"
    mkdir -p "$dir"
    for attempt in 1 2 3; do
        if "$STEAMCMD" +force_install_dir "$dir" +login anonymous \
            +app_update "$app" ${branch:+-beta "$branch"} ${validate:+validate} +quit; then
            return 0
        fi
        dk_log "steamcmd app_update $app failed (attempt $attempt)"
        sleep 5
    done
    dk_die "could not install Steam app $app"
}

# dk_steam_workshop APP_ID DIR ID... — download workshop items anonymously
# into DIR/steamapps/workshop/content/APP_ID/<id>.
dk_steam_workshop() {
    local app=$1 dir=$2
    shift 2
    [[ $# -gt 0 ]] || return 0
    local args=() id attempt
    for id in "$@"; do
        [[ $id =~ ^[0-9]+$ ]] || dk_die "invalid workshop id: $id"
        args+=(+workshop_download_item "$app" "$id")
    done
    mkdir -p "$dir"
    for attempt in 1 2 3; do
        if "$STEAMCMD" +force_install_dir "$dir" +login anonymous "${args[@]}" +quit; then
            return 0
        fi
        dk_log "steamcmd workshop download failed (attempt $attempt)"
        sleep 5
    done
    dk_die "could not download workshop items for app $app"
}

# dk_steam_sdk_link — dedicated servers look for steamclient.so here.
dk_steam_sdk_link() {
    mkdir -p "$HOME/.steam/sdk64" "$HOME/.steam/sdk32"
    ln -sf /opt/steamcmd/linux64/steamclient.so "$HOME/.steam/sdk64/steamclient.so"
    ln -sf /opt/steamcmd/linux32/steamclient.so "$HOME/.steam/sdk32/steamclient.so"
}
