#!/usr/bin/env bash
# Sync DK_MODS_MODRINTH / DK_MODS_CURSEFORGE into mods/ (Fabric, Forge,
# NeoForge) or plugins/ (Paper). Every list is synced even when empty: that is
# how a mod removed in DeployKit disappears from the server.
. /opt/deploykit/lib/common.sh
. "$DK_STATE_DIR/launch.env"

case $MC_TYPE in
    fabric) loaders=fabric dir=mods ;;
    forge) loaders=forge dir=mods ;;
    neoforge) loaders=neoforge dir=mods ;;
    paper) loaders=paper,bukkit,spigot dir=plugins ;;
    vanilla) loaders='' dir='' ;;
esac

modrinth=${DK_MODS_MODRINTH:-}
curseforge=${DK_MODS_CURSEFORGE:-}
if [[ -z $dir ]]; then
    [[ -z $modrinth$curseforge ]] || dk_log "vanilla servers do not load mods; ignoring the mod list"
    modrinth='' curseforge='' dir=mods
fi

dk-entrypoint mods sync --source modrinth --list "$modrinth" \
    --dir "$DK_DATA_DIR/$dir" --state "$DK_STATE_DIR/mods-modrinth.json" \
    --loader "$loaders" --game-version "$MC_VERSION"
dk-entrypoint mods sync --source curseforge --list "$curseforge" \
    --dir "$DK_DATA_DIR/$dir" --state "$DK_STATE_DIR/mods-curseforge.json" \
    --loader "$loaders" --game-version "$MC_VERSION"
