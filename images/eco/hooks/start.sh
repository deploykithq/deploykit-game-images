#!/usr/bin/env bash
. /opt/deploykit/lib/common.sh
. /opt/deploykit/lib/steam.sh

args=()
token=${DK_VAR_SERVER_TOKEN:-}
if [[ -n $token ]]; then
    [[ $token =~ ^[A-Za-z0-9._=+/-]{8,512}$ ]] || dk_die "DK_VAR_SERVER_TOKEN does not look like a server token"
    args+=("-userToken=$token")
else
    dk_log "no server token: starting in offline mode (not authenticated with Strange Cloud, not listed)"
    args+=(-offline)
fi

dk_steam_sdk_link
# EcoServer is a single-file .NET app that unpacks its native libraries on
# start; keep them with the install instead of re-extracting into $HOME.
export DOTNET_BUNDLE_EXTRACT_BASE_DIR=$DK_SERVER_DIR/.net
cd "$DK_SERVER_DIR/run" || exit 1
exec ./EcoServer "${args[@]}"
