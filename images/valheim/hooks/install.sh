#!/usr/bin/env bash
# Install or update the Valheim dedicated server. SteamCMD only downloads
# what changed, so this runs on every start and keeps the server current.
. /opt/deploykit/lib/common.sh
. /opt/deploykit/lib/steam.sh

dir=$DK_SERVER_DIR/valheim
if [[ -x $dir/valheim_server.x86_64 && $(dk_bool "${DK_VAR_SKIP_UPDATE:-false}") == true ]]; then
    dk_log "valheim already installed (update skipped)"
    exit 0
fi
dk_log "installing/updating valheim (Steam app 896660)"
dk_steam_app_update 896660 "$dir" "$([[ $(dk_bool "${DK_VAR_VALIDATE:-false}") == true ]] && echo validate)"
[[ -x $dir/valheim_server.x86_64 ]] || dk_die "SteamCMD finished but valheim_server.x86_64 is missing"
dk_log "valheim already installed (up to date)"
