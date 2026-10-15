#!/usr/bin/env bash
# Install or update the Project Zomboid dedicated server. SteamCMD only
# downloads what changed, so this runs on every start and keeps the server
# current. DK_GAME_VERSION picks the build line:
#   b42 (default)  Steam's public branch
#   b41            the legacy41 branch (Build 41.78)
. /opt/deploykit/lib/common.sh
. /opt/deploykit/lib/steam.sh

case ${DK_GAME_VERSION:-b42} in
    b42 | B42 | LATEST | latest | "") build=b42 branch= ;;
    b41 | B41) build=b41 branch=legacy41 ;;
    *) dk_die "DK_GAME_VERSION must be b42 or b41 (got '$DK_GAME_VERSION')" ;;
esac

dir=$DK_SERVER_DIR/pz
# Switching branch over an install of the other one must download it.
if [[ -x $dir/start-server.sh && $(dk_stamp build) == "$build" && $(dk_bool "${DK_VAR_SKIP_UPDATE:-false}") == true ]]; then
    dk_log "project zomboid $build already installed (update skipped)"
    exit 0
fi
dk_log "installing/updating project zomboid $build (Steam app 380870${branch:+, branch $branch})"
validate=
[[ $(dk_bool "${DK_VAR_VALIDATE:-false}") == true || ($(dk_stamp build) != "" && $(dk_stamp build) != "$build") ]] && validate=validate
dk_steam_app_update 380870 "$dir" "$validate" "$branch"
[[ -x $dir/start-server.sh ]] || dk_die "SteamCMD finished but start-server.sh is missing"
dk_stamp build "$build"
dk_log "project zomboid $build already installed (up to date)"
