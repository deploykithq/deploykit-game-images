# Build graph for every image. A game's base is a bake target passed as a
# named build context, so one `bake` builds bases and games together:
#
#   docker buildx bake terraria --set '*.platform=linux/amd64' --load
#   docker buildx bake                       # everything, default platforms
#
# Tags here are the development defaults; scripts/publish.sh overrides them
# with release versions.

variable "REGISTRY" {
  default = "ghcr.io/deploykithq"
}

variable "SUPERVISOR_VERSION" {
  default = "dev"
}

variable "JAVA_VERSIONS" {
  default = ["8", "17", "21", "25"]
}

group "default" {
  targets = ["bases", "games"]
}

group "bases" {
  targets = ["base-debian", "base-java", "base-steamcmd"]
}

group "games" {
  targets = ["terraria", "minecraft-java", "tmodloader", "valheim", "cs2", "eco"]
}

target "_common" {
  context = "."
  labels = {
    "org.opencontainers.image.source"   = "https://github.com/deploykithq/deploykit-game-images"
    "org.opencontainers.image.licenses" = "MIT"
    "io.deploykit.contract"             = "1"
  }
}

# --- bases ----------------------------------------------------------------------

target "base-debian" {
  inherits   = ["_common"]
  dockerfile = "base/Dockerfile"
  args       = { FROM = "debian:bookworm-slim", VERSION = SUPERVISOR_VERSION }
  platforms  = ["linux/amd64", "linux/arm64"]
  tags       = ["${REGISTRY}/game-base:debian"]
}

target "base-java" {
  name       = "base-java${j}"
  matrix     = { j = JAVA_VERSIONS }
  inherits   = ["_common"]
  dockerfile = "base/Dockerfile"
  args       = { FROM = "eclipse-temurin:${j}-jre-noble", VERSION = SUPERVISOR_VERSION }
  platforms  = ["linux/amd64", "linux/arm64"]
  tags       = ["${REGISTRY}/game-base:java${j}"]
}

# Steam only ships x86 dedicated servers.
target "base-steamcmd" {
  inherits   = ["_common"]
  dockerfile = "base/steamcmd/Dockerfile"
  contexts   = { dk-base = "target:base-debian" }
  args       = { BASE = "dk-base" }
  platforms  = ["linux/amd64"]
  tags       = ["${REGISTRY}/game-base:steamcmd"]
}

# --- games ----------------------------------------------------------------------

target "terraria" {
  inherits   = ["_common"]
  dockerfile = "images/terraria/Dockerfile"
  contexts   = { dk-base = "target:base-debian" }
  args       = { BASE = "dk-base" }
  platforms  = ["linux/amd64"]
  tags       = ["${REGISTRY}/game-terraria:edge"]
}

target "minecraft-java" {
  name       = "minecraft-java${j}"
  matrix     = { j = JAVA_VERSIONS }
  inherits   = ["_common"]
  dockerfile = "images/minecraft-java/Dockerfile"
  contexts   = { dk-base = "target:base-java${j}" }
  args       = { BASE = "dk-base" }
  platforms  = ["linux/amd64", "linux/arm64"]
  tags       = ["${REGISTRY}/game-minecraft-java:java${j}-edge"]
}

target "tmodloader" {
  inherits   = ["_common"]
  dockerfile = "images/tmodloader/Dockerfile"
  contexts   = { dk-base = "target:base-steamcmd" }
  args       = { BASE = "dk-base" }
  platforms  = ["linux/amd64"]
  tags       = ["${REGISTRY}/game-tmodloader:edge"]
}

target "valheim" {
  inherits   = ["_common"]
  dockerfile = "images/valheim/Dockerfile"
  contexts   = { dk-base = "target:base-steamcmd" }
  args       = { BASE = "dk-base" }
  platforms  = ["linux/amd64"]
  tags       = ["${REGISTRY}/game-valheim:edge"]
}

target "cs2" {
  inherits   = ["_common"]
  dockerfile = "images/cs2/Dockerfile"
  contexts   = { dk-base = "target:base-steamcmd" }
  args       = { BASE = "dk-base" }
  platforms  = ["linux/amd64"]
  tags       = ["${REGISTRY}/game-cs2:edge"]
}

target "eco" {
  inherits   = ["_common"]
  dockerfile = "images/eco/Dockerfile"
  contexts   = { dk-base = "target:base-steamcmd" }
  args       = { BASE = "dk-base" }
  platforms  = ["linux/amd64"]
  tags       = ["${REGISTRY}/game-eco:edge"]
}
