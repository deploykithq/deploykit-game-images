#!/usr/bin/env bash
# Steam Workshop mods (app 1281930).
#   DK_MODS_STEAM          every workshop item to keep downloaded
#   DK_MODS_STEAM_ENABLED  the subset to enable
# Items no longer listed are deleted; enabled.json is rewritten from the
# enabled subset plus any mod uploaded by hand into tModLoader/Mods.
. /opt/deploykit/lib/common.sh
. /opt/deploykit/lib/steam.sh

app=1281930
workshop=$DK_SERVER_DIR/workshop
content=$workshop/steamapps/workshop/content/$app
modsdir=$DK_DATA_DIR/tModLoader/Mods
mkdir -p "$content" "$modsdir"

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
    dk_steam_workshop "$app" "$workshop" "${all[@]}"
fi

# Workshop folders hold one .tmod per tModLoader version line; the mod's
# internal name is the file name, the same in every one of them.
names=()
for id in "${enabled[@]}"; do
    tmod=$(find "$content/$id" -name '*.tmod' -printf '%T@ %p\n' 2>/dev/null | sort -rn | head -n1 | cut -d' ' -f2-)
    [[ -n $tmod ]] || dk_die "workshop item $id has no .tmod file"
    name=$(basename "$tmod" .tmod)
    names+=("$name")
done
for tmod in "$modsdir"/*.tmod; do
    if [[ -f $tmod ]]; then names+=("$(basename "$tmod" .tmod)"); fi
done

# No mods at all is the default: an empty list, not a failed grep.
if ((${#names[@]} > 0)); then
    printf '%s\n' "${names[@]}" | sort -u | jq -R . | jq -s . >"$modsdir/enabled.json"
else
    echo '[]' >"$modsdir/enabled.json"
fi
dk_log "enabled mods: $(jq -r 'join(", ")' "$modsdir/enabled.json")"
