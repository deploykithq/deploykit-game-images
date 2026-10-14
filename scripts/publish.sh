#!/usr/bin/env bash
# Build, push and sign the images of released components.
#
#   scripts/publish.sh COMPONENT...
#
# Components: supervisor, base, terraria, minecraft-java, tmodloader, valheim,
# cs2, or all. A supervisor or base release republishes every image under its
# current version, because every image is built on the base.
#
# Tags for an image at version X.Y.Z:  X.Y.Z  X.Y  X  latest
# Minecraft, per Java variant N:       X.Y.Z-javaN  X.Y-javaN  X-javaN  javaN
#                                      (java25 also gets the plain tags)
# Bases:                               game-base:<flavor>  game-base:<flavor>-<base version>
#
# Env: REGISTRY (default ghcr.io/deploykithq), DRY_RUN=1 prints the plan.
set -euo pipefail
cd "$(dirname "$0")/.."

REGISTRY=${REGISTRY:-ghcr.io/deploykithq}
games=(terraria minecraft-java tmodloader valheim cs2)
javas=(8 17 21 25)

[[ $# -gt 0 ]] || { echo "usage: publish.sh COMPONENT..." >&2; exit 2; }
declare -A want=()
for c in "$@"; do
    case $c in
        all | supervisor | base)
            want[bases]=1
            for g in "${games[@]}"; do want[$g]=1; done
            ;;
        terraria | minecraft-java | tmodloader | valheim | cs2) want[$c]=1 ;;
        *) echo "unknown component: $c" >&2; exit 2 ;;
    esac
done

version() { tr -d '[:space:]' <"$1/version.txt"; }
export SUPERVISOR_VERSION
SUPERVISOR_VERSION=$(version supervisor)

targets=()
sets=()
add() { # TARGET TAG...
    local t=$1
    shift
    targets+=("$t")
    local IFS=,
    sets+=(--set "$t.tags=$*")
}
semver() { # IMAGE VERSION SUFFIX -> X.Y.Z X.Y X (with suffix)
    local v=$2
    echo "$1:$v$3" "$1:${v%.*}$3" "$1:${v%%.*}$3"
}

if [[ -n ${want[bases]:-} ]]; then
    bv=$(version base)
    add base-debian "$REGISTRY/game-base:debian" "$REGISTRY/game-base:debian-$bv"
    add base-steamcmd "$REGISTRY/game-base:steamcmd" "$REGISTRY/game-base:steamcmd-$bv"
    for j in "${javas[@]}"; do
        add "base-java$j" "$REGISTRY/game-base:java$j" "$REGISTRY/game-base:java$j-$bv"
    done
fi

for g in "${games[@]}"; do
    [[ -n ${want[$g]:-} ]] || continue
    v=$(version "images/$g")
    img=$REGISTRY/game-$g
    if [[ $g == minecraft-java ]]; then
        for j in "${javas[@]}"; do
            read -r -a tags <<<"$(semver "$img" "$v" "-java$j") $img:java$j"
            if [[ $j == "${javas[-1]}" ]]; then
                read -r -a plain <<<"$(semver "$img" "$v" "") $img:latest"
                tags+=("${plain[@]}")
            fi
            add "minecraft-java$j" "${tags[@]}"
        done
    else
        # shellcheck disable=SC2046
        add "$g" $(semver "$img" "$v" "") "$img:latest"
    fi
done

echo "supervisor $SUPERVISOR_VERSION; targets: ${targets[*]}"
if [[ ${DRY_RUN:-0} == 1 ]]; then
    printf '  %s\n' "${sets[@]}" | grep -v '^  --set$'
    exit 0
fi

meta=$(mktemp)
docker buildx bake "${targets[@]}" "${sets[@]}" \
    --set '*.attest=type=sbom' --set '*.attest=type=provenance,mode=max' \
    --push --metadata-file "$meta"

# Keyless signature (GitHub OIDC) of every pushed digest.
if command -v cosign >/dev/null; then
    for t in "${targets[@]}"; do
        digest=$(jq -r --arg t "$t" '.[$t]["containerimage.digest"]' "$meta")
        repo=$(jq -r --arg t "$t" '.[$t]["image.name"]' "$meta" | cut -d, -f1)
        repo=${repo%:*}
        cosign sign --yes "$repo@$digest"
    done
fi
rm -f "$meta"
