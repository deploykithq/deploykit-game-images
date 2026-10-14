#!/usr/bin/env bash
# Install or update CS2. The first run downloads about 60 GB.
. /opt/deploykit/lib/common.sh
. /opt/deploykit/lib/steam.sh

dir=$DK_SERVER_DIR/cs2
bin=$dir/game/bin/linuxsteamrt64/cs2
if [[ -x $bin && $(dk_bool "${DK_VAR_SKIP_UPDATE:-false}") == true ]]; then
    dk_log "cs2 already installed (update skipped)"
    exit 0
fi
dk_log "installing/updating cs2 (Steam app 730)"
dk_steam_app_update 730 "$dir" "$([[ $(dk_bool "${DK_VAR_VALIDATE:-false}") == true ]] && echo validate)"
[[ -x $bin ]] || dk_die "SteamCMD finished but $bin is missing"
dk_log "cs2 already installed (up to date)"
