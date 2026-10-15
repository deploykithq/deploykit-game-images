#!/usr/bin/env bash
# Install or update the Eco dedicated server. SteamCMD only downloads what
# changed, so this runs on every start and keeps the server current.
. /opt/deploykit/lib/common.sh
. /opt/deploykit/lib/steam.sh

dir=$DK_SERVER_DIR/eco
if [[ -x $dir/EcoServer && $(dk_bool "${DK_VAR_SKIP_UPDATE:-false}") == true ]]; then
    dk_log "eco already installed (update skipped)"
    exit 0
fi
dk_log "installing/updating eco (Steam app 739590)"
dk_steam_app_update 739590 "$dir" "$([[ $(dk_bool "${DK_VAR_VALIDATE:-false}") == true ]] && echo validate)"
[[ -x $dir/EcoServer ]] || dk_die "SteamCMD finished but EcoServer is missing"
dk_log "eco already installed (up to date)"
