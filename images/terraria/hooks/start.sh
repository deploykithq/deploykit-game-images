#!/usr/bin/env bash
. /opt/deploykit/lib/common.sh
cd "$DK_SERVER_DIR/terraria" || exit 1
exec ./TerrariaServer.bin.x86_64 -config "$DK_DATA_DIR/serverconfig.txt"
