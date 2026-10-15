#!/usr/bin/env bash
# Install the server (and the modpack, which decides version and loader).
# Writes DK_STATE_DIR/launch.env for start.sh and mods.sh.
. /opt/deploykit/lib/common.sh

data=$DK_DATA_DIR
state=$DK_STATE_DIR

[[ $(dk_bool "${DK_VAR_EULA:-false}") == true ]] ||
    dk_die "the Minecraft EULA (https://aka.ms/MinecraftEULA) must be accepted: set DK_VAR_EULA=true"
printf 'eula=true\n' >"$data/eula.txt"

type=${DK_VAR_TYPE:-paper}
type=${type,,}
version=${DK_GAME_VERSION:-LATEST}
loader_version=${DK_VAR_LOADER_VERSION:-}

# --- modpack -----------------------------------------------------------------
# Always called: an empty project uninstalls a previously installed pack.
dk-entrypoint mods modpack \
    --source "${DK_MODPACK_SOURCE:-}" --project "${DK_MODPACK_PROJECT:-}" --version "${DK_MODPACK_VERSION:-}" \
    --dir "$data" --state "$state/modpack.json" --env-out "$state/modpack.env"
DK_RESOLVED_LOADER=
. "$state/modpack.env"
if [[ -n $DK_RESOLVED_LOADER ]]; then
    type=$DK_RESOLVED_LOADER
    version=$DK_RESOLVED_GAME_VERSION
    loader_version=$DK_RESOLVED_LOADER_VERSION
    dk_log "modpack wants $type ${loader_version:-} on Minecraft $version"
fi

case $type in
    vanilla | paper | fabric | forge | neoforge) ;;
    *) dk_die "DK_VAR_TYPE must be vanilla, paper, fabric, forge or neoforge (got '$type')" ;;
esac

# --- Minecraft version ---------------------------------------------------------
manifest=$(dk_get https://piston-meta.mojang.com/mc/game/version_manifest_v2.json)
case ${version^^} in
    LATEST) version=$(jq -r .latest.release <<<"$manifest") ;;
    SNAPSHOT) version=$(jq -r .latest.snapshot <<<"$manifest") ;;
esac
version_url=$(jq -r --arg v "$version" '.versions[] | select(.id == $v) | .url' <<<"$manifest")
[[ -n $version_url ]] || dk_die "unknown Minecraft version '$version'"
version_json=$(dk_get "$version_url")

# Fail early and clearly instead of with a stack trace from the JVM.
need_java=$(jq -r '.javaVersion.majorVersion // 8' <<<"$version_json")
have_java=$(java -XshowSettings:properties -version 2>&1 | awk -F'= ' '/java.specification.version/ {print $2}')
have_java=${have_java#1.}
if ((have_java < need_java)); then
    dk_die "Minecraft $version needs Java $need_java but this image has Java $have_java: use the java$need_java image"
fi

# --- server -------------------------------------------------------------------
launch_kind=jar
launch_target=
mc_major_minor() { # 1.21.4 -> 21.4 ; 1.21 -> 21.0 ; 26.1 -> 26.1
    local v=$1
    if [[ $v == 1.* ]]; then
        v=${v#1.}
        [[ $v == *.* ]] && echo "$v" || echo "$v.0"
    else
        echo "$v"
    fi
}

stamp_key=
install_vanilla() {
    local url sha1
    url=$(jq -r .downloads.server.url <<<"$version_json")
    sha1=$(jq -r .downloads.server.sha1 <<<"$version_json")
    [[ $url != null ]] || dk_die "Minecraft $version has no server download"
    stamp_key="vanilla $version"
    launch_target=$data/server.jar
    [[ $(dk_stamp server) == "$stamp_key" && -f $launch_target ]] && return 0
    dk_log "installing vanilla $version"
    dk_fetch "$url" "$launch_target" "sha1:$sha1"
}

install_paper() {
    local builds build url sha
    builds=$(dk_get "https://fill.papermc.io/v3/projects/paper/versions/$version/builds" 2>/dev/null) ||
        dk_die "Paper has no build for Minecraft $version"
    build=$(jq -c '(map(select(.channel == "STABLE")) + .)[0]' <<<"$builds")
    [[ $build != null ]] || dk_die "Paper has no build for Minecraft $version"
    url=$(jq -r '.downloads["server:default"].url' <<<"$build")
    sha=$(jq -r '.downloads["server:default"].checksums.sha256' <<<"$build")
    stamp_key="paper $version $(jq -r .id <<<"$build")"
    launch_target=$data/paper.jar
    [[ $(dk_stamp server) == "$stamp_key" && -f $launch_target ]] && return 0
    dk_log "installing paper $version build $(jq -r .id <<<"$build")"
    dk_fetch "$url" "$launch_target" "sha256:$sha"
}

install_fabric() {
    local installer
    if [[ -z $loader_version ]]; then
        loader_version=$(dk_get "https://meta.fabricmc.net/v2/versions/loader/$version" |
            jq -r '(map(select(.loader.stable)) + .)[0].loader.version')
        [[ $loader_version != null ]] || dk_die "Fabric does not support Minecraft $version"
    fi
    installer=$(dk_get https://meta.fabricmc.net/v2/versions/installer | jq -r '(map(select(.stable)) + .)[0].version')
    stamp_key="fabric $version $loader_version"
    launch_target=$data/fabric-server-launch.jar
    [[ $(dk_stamp server) == "$stamp_key" && -f $launch_target ]] && return 0
    dk_log "installing fabric loader $loader_version for $version"
    dk_fetch "https://meta.fabricmc.net/v2/versions/loader/$version/$loader_version/$installer/server/jar" "$launch_target"
}

# run_installer URL SHA1 — Forge/NeoForge installers write libraries/ and
# their launch files into the data directory. Without the maven .sha1 the
# installer is not run, unless DK_VAR_ALLOW_UNVERIFIED=true.
run_installer() {
    local tmp
    if [[ ! $2 =~ ^[0-9a-fA-F]{40}$ ]]; then
        [[ $(dk_bool "${DK_VAR_ALLOW_UNVERIFIED:-false}") == true ]] ||
            dk_die "no sha1 published for $1; retry later or set DK_VAR_ALLOW_UNVERIFIED=true"
        dk_log "warning: running the installer without hash verification (DK_VAR_ALLOW_UNVERIFIED)"
        set -- "$1" ""
    fi
    tmp=$(mktemp -d)
    dk_fetch "$1" "$tmp/installer.jar" "${2:+sha1:$2}"
    (cd "$data" && java -jar "$tmp/installer.jar" --installServer "$data" >"$state/installer.log" 2>&1) || {
        tail -n 30 "$state/installer.log"
        dk_die "the installer failed (full log: .deploykit/installer.log)"
    }
    rm -rf "$tmp"
}

install_forge() {
    local full sha
    if [[ -z $loader_version ]]; then
        local promos
        promos=$(dk_get https://files.minecraftforge.net/net/minecraftforge/forge/promotions_slim.json)
        loader_version=$(jq -r --arg v "$version" '.promos[$v + "-recommended"] // .promos[$v + "-latest"] // empty' <<<"$promos")
        [[ -n $loader_version ]] || dk_die "Forge does not support Minecraft $version"
    fi
    full=$version-$loader_version
    stamp_key="forge $full"
    if ! [[ $(dk_stamp server) == "$stamp_key" ]]; then
        dk_log "installing forge $full"
        local base=https://maven.minecraftforge.net/net/minecraftforge/forge/$full/forge-$full-installer.jar
        sha=$(dk_get "$base.sha1" 2>/dev/null | cut -c1-40 || true)
        run_installer "$base" "$sha"
    fi
    if [[ -f $data/libraries/net/minecraftforge/forge/$full/unix_args.txt ]]; then
        launch_kind=args
        launch_target=libraries/net/minecraftforge/forge/$full/unix_args.txt
    else
        launch_target=$(find "$data" -maxdepth 1 -name "forge-$full*.jar" ! -name '*installer*' | head -n1)
        [[ -n $launch_target ]] || dk_die "the Forge installer produced no server jar"
    fi
}

install_neoforge() {
    local prefix sha
    if [[ -z $loader_version ]]; then
        prefix=$(mc_major_minor "$version").
        loader_version=$(dk_get https://maven.neoforged.net/releases/net/neoforged/neoforge/maven-metadata.xml |
            grep -o '<version>[^<]*</version>' | sed 's/<[^>]*>//g' | grep "^${prefix//./\\.}" |
            { grep -v beta || true; } | tail -n1)
        [[ -n $loader_version ]] || dk_die "NeoForge does not support Minecraft $version"
    fi
    stamp_key="neoforge $loader_version"
    if ! [[ $(dk_stamp server) == "$stamp_key" ]]; then
        dk_log "installing neoforge $loader_version"
        local base=https://maven.neoforged.net/releases/net/neoforged/neoforge/$loader_version/neoforge-$loader_version-installer.jar
        sha=$(dk_get "$base.sha1" 2>/dev/null | cut -c1-40 || true)
        run_installer "$base" "$sha"
    fi
    launch_kind=args
    launch_target=libraries/net/neoforged/neoforge/$loader_version/unix_args.txt
    [[ -f $data/$launch_target ]] || dk_die "the NeoForge installer produced no launch arguments"
}

"install_$type"
if [[ $(dk_stamp server) == "$stamp_key" ]]; then
    dk_log "minecraft server already installed ($stamp_key)"
else
    dk_stamp server "$stamp_key"
fi

cat >"$state/launch.env" <<EOF
MC_TYPE=$type
MC_VERSION=$version
MC_LAUNCH_KIND=$launch_kind
MC_LAUNCH_TARGET=$launch_target
EOF
