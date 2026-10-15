#!/usr/bin/env bash
# What start-server.sh does, but exec'd: the script runs the server as a
# child and always exits 0, which would hide the game's exit code.
. /opt/deploykit/lib/common.sh
. /opt/deploykit/lib/steam.sh

admin=${DK_VAR_ADMIN_PASSWORD:-}
[[ -n $admin ]] || dk_die "DK_VAR_ADMIN_PASSWORD is required (the in-game admin user's password)"
[[ $admin =~ ^[^[:space:]\"]{6,64}$ ]] || dk_die "DK_VAR_ADMIN_PASSWORD: 6-64 characters, no spaces or quotes"

dk_steam_sdk_link
dir=$DK_SERVER_DIR/pz
cd "$dir" || exit 1
export PATH=$dir/jre64/bin:$PATH
export LD_LIBRARY_PATH=$dir/linux64:$dir:$dir/jre64/lib:$dir/jre64/lib/amd64${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}
export LD_PRELOAD=libjsig.so${LD_PRELOAD:+:$LD_PRELOAD}
# -adminpassword only matters when the player database is first created;
# without it the server would stop and ask for one on stdin.
exec ./ProjectZomboid64 -servername servertest -cachedir="$DK_DATA_DIR/Zomboid" \
    -adminusername admin -adminpassword "$admin"
