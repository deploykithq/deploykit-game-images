#!/usr/bin/env bash
. /opt/deploykit/lib/common.sh
cd "$DK_SERVER_DIR/tmodloader" || exit 1
exec dotnet tModLoader.dll -server -nosteam \
    -config "$DK_DATA_DIR/serverconfig.txt" \
    -tmlsavedirectory "$DK_DATA_DIR/tModLoader" \
    -steamworkshopfolder "$DK_SERVER_DIR/workshop/steamapps/workshop"
