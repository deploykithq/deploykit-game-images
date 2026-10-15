#!/usr/bin/env bash
# Steam Workshop mods (app 108600).
#   DK_MODS_STEAM          every workshop item to keep downloaded
#   DK_MODS_STEAM_ENABLED  the subset to enable
# Items are downloaded where the server itself looks for them (its install's
# steamapps/workshop); items no longer listed are deleted. WorkshopItems and
# Mods in servertest.ini are rewritten: every item, and the mod ids of the
# enabled ones plus any mod uploaded by hand into /data/Zomboid/mods.
. /opt/deploykit/lib/common.sh
. /opt/deploykit/lib/steam.sh

app=108600
dir=$DK_SERVER_DIR/pz
content=$dir/steamapps/workshop/content/$app
ini=$DK_DATA_DIR/Zomboid/Server/servertest.ini
local_mods=$DK_DATA_DIR/Zomboid/mods
mkdir -p "$content" "$local_mods"

split() { tr ',' '\n' <<<"${1:-}" | sed 's/:.*//; s/[[:space:]]//g' | grep -v '^$' || true; }
mapfile -t all < <(split "${DK_MODS_STEAM:-}")
mapfile -t enabled < <(split "${DK_MODS_STEAM_ENABLED:-}")

for id in "${all[@]}" "${enabled[@]}"; do
    [[ $id =~ ^[0-9]+$ ]] || dk_die "invalid workshop id '$id'"
done

# Remove items that are no longer listed.
for path in "$content"/*; do
    [[ -d $path ]] || continue
    id=${path##*/}
    if ! printf '%s\n' "${all[@]}" | grep -qx "$id"; then
        dk_log "removing workshop item $id"
        rm -rf "$path"
    fi
done

if ((${#all[@]} > 0)); then
    dk_log "downloading ${#all[@]} workshop item(s)"
    dk_steam_workshop "$app" "$dir" "${all[@]}"
fi

# mod_ids DIR — the id= of every mod.info under DIR. Build 41 keeps it in
# mods/<name>/mod.info, Build 42 in mods/<name>/42/ (or common/); one item may
# hold several mods. Ids are de-duplicated, keeping the first seen.
mod_ids() {
    find "$1" -name mod.info -print0 2>/dev/null |
        xargs -0 -r grep -h -m1 -i '^id=' |
        sed 's/^[iI][dD]=//; s/[[:space:]]*$//' |
        awk 'NF && !seen[$0]++'
}

ids=()
for id in "${enabled[@]}"; do
    [[ -d $content/$id ]] || dk_die "workshop item $id was not downloaded"
    mapfile -t found < <(mod_ids "$content/$id")
    ((${#found[@]} > 0)) || dk_die "workshop item $id has no mod.info: it is not a Project Zomboid mod"
    ids+=("${found[@]}")
done
mapfile -t found < <(mod_ids "$local_mods")
ids+=("${found[@]}")

for id in "${ids[@]}"; do
    [[ $id != *";"* ]] || dk_die "mod id '$id' contains ';'"
done

# Build 42 separates mod ids with ";" and prefixes each with "\"; Build 41
# takes them bare.
prefix=
[[ $(dk_stamp build) == b41 ]] || prefix='\'
mods=$(printf '%s\n' "${ids[@]}" | awk -v p="$prefix" 'NF && !seen[$0]++ { printf "%s%s%s", (n++ ? ";" : ""), p, $0 }')
items=$(printf '%s\n' "${all[@]}" | awk 'NF && !seen[$0]++ { printf "%s%s", (n++ ? ";" : ""), $0 }')

dk_set_prop "$ini" WorkshopItems "$items"
dk_set_prop "$ini" Mods "$mods"
dk_log "workshop items: ${items:-none}"
dk_log "enabled mods: ${mods:-none}"
