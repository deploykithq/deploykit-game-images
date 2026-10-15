# DeployKit game images

Game server images built for [DeployKit](https://github.com/deploykithq/deploykit).
They all follow one contract — [CONTRACT.md](CONTRACT.md) — so a DeployKit
game definition only passes values. Every image has the same entrypoint, takes
its console on stdin, uses the same `DK_*` variables, stores data in `/data`,
runs as uid 1000 and prints the same `[deploykit] ready` line, whatever the
game does underneath.

Game binaries are **never baked into an image**. They are downloaded from the
official source on first start (SteamCMD, Mojang/PaperMC/Fabric/Forge/NeoForge,
terraria.org, the tModLoader GitHub release) into `/server` or `/data`, and
reused on every later start.

## Images

| Image | Game | Console | Platforms |
|---|---|---|---|
| `ghcr.io/deploykithq/game-terraria` | Terraria (vanilla) | stdin | amd64 |
| `ghcr.io/deploykithq/game-minecraft-java:java{8,17,21,25}` | Minecraft: Java Edition — vanilla, Paper, Fabric, Forge, NeoForge | stdin | amd64, arm64 |
| `ghcr.io/deploykithq/game-tmodloader` | Terraria with tModLoader | stdin | amd64 |
| `ghcr.io/deploykithq/game-valheim` | Valheim | none | amd64 |
| `ghcr.io/deploykithq/game-cs2` | Counter-Strike 2 | RCON (game port, TCP) | amd64 |
| `ghcr.io/deploykithq/game-eco` | Eco | RCON (container-local) | amd64 |
| `ghcr.io/deploykithq/game-project-zomboid` | Project Zomboid (Build 42 / 41) | stdin | amd64 |

### Variables per image

`DK_VAR_*` are the game's form fields. Defaults are in brackets.

**terraria** — `DK_GAME_VERSION` [1.4.5.8], `DK_PORT_GAME` [7777].
`WORLD_NAME` [World], `WORLD_SIZE` 1-3 [2], `DIFFICULTY` 0-3 [0], `SEED`,
`MAX_PLAYERS` [8], `PASSWORD`, `MOTD`. The world is `/data/worlds/<name>.wld`
and the config file is `/data/serverconfig.txt`; unmanaged keys in the config
are kept.

**minecraft-java** — `DK_VAR_EULA` (**required**, `true`),
`DK_VAR_TYPE` vanilla|paper|fabric|forge|neoforge [paper],
`DK_GAME_VERSION` LATEST|SNAPSHOT|x.y.z [LATEST], `DK_VAR_LOADER_VERSION`
[newest stable], `DK_PORT_GAME` [25565].
`MAX_PLAYERS` [20], `MOTD`, `DIFFICULTY` [normal], `ONLINE_MODE` [true],
`SEED`, `JVM_OPTS`. Mods come from `DK_MODS_MODRINTH` and
`DK_MODS_CURSEFORGE`, with required dependencies resolved, and go to `mods/`
or `plugins/` (Paper). A modpack from Modrinth (`.mrpack`) or CurseForge
decides the version and the loader. The Java tag must match the Minecraft
version, and the server says which one is needed if it doesn't. A Forge or
NeoForge installer without a published sha1 is not run unless
`DK_VAR_ALLOW_UNVERIFIED=true`.

**tmodloader** — `DK_GAME_VERSION` tag|latest [latest], `DK_PORT_GAME` [7777],
the same world variables as terraria. Workshop mods come from
`DK_MODS_STEAM` / `DK_MODS_STEAM_ENABLED`. Worlds are in
`/data/tModLoader/Worlds`. A `.tmod` uploaded by hand into
`/data/tModLoader/Mods` stays enabled. The download is checked against the
sha256 GitHub publishes; if GitHub's API does not answer, the install stops
unless `DK_VAR_ALLOW_UNVERIFIED=true`.

**valheim** — `DK_PORT_GAME` [2456] (query port is game + 1),
`SERVER_NAME`, `WORLD_NAME` [Dedicated], `SERVER_PASS` (≥ 5 characters, not
part of the name), `SERVER_PUBLIC` [true], `VALIDATE`, `SKIP_UPDATE`. Worlds
are in `/data/worlds_local`.

**cs2** — `DK_PORT_GAME` [27015] (UDP game, TCP RCON: publish only the UDP side), `SERVER_NAME`,
`GAME_MODE` competitive|casual|wingman|deathmatch, `MAP` [de_dust2],
`MAX_PLAYERS` [10], `GSLT` (without one the server is LAN-only),
`SERVER_PASSWORD`, `VALIDATE`, `SKIP_UPDATE`. Your cvars go in `/data/cfg/*.cfg`
(`server.cfg`, `gamemode_*_server.cfg`); they are linked into the game. The
first start downloads about 60 GB.

**eco** — `DK_PORT_GAME` [3000] (UDP), `DK_PORT_WEB` [3001] (TCP, the web
map and stats), `SERVER_TOKEN` (from play.eco → Account → Server
Authentication; without one the server starts offline), `SERVER_NAME`,
`DESCRIPTION`, `PASSWORD`, `PUBLIC` [false], `VALIDATE`, `SKIP_UPDATE`. The
world is `/data/Storage`, the configs `/data/Configs/*.eco` (the matching
`*.eco.template` files link to the installed defaults) and mods
`/data/Mods/UserCode`. The listed keys of `Network.eco` are rewritten every
start; everything else in it, and every other config, is yours. Console
lines are Eco's chat commands without the slash (`manage players`).

**project-zomboid** — `DK_GAME_VERSION` b42|b41 [b42] (b41 is Steam's
`legacy41` branch), `DK_PORT_GAME` [16261] and `DK_PORT_DIRECT` [16262] (both
UDP), `SERVER_NAME`, `ADMIN_PASSWORD` (**required**; the `admin` user's
password, applied when the player database is created), `SERVER_PASS`,
`MAX_PLAYERS` [16], `PVP` [true], `PUBLIC` [false], `VALIDATE`, `SKIP_UPDATE`.
Workshop mods come from `DK_MODS_STEAM` / `DK_MODS_STEAM_ENABLED`. Everything
lives under `/data/Zomboid` (the save is always `servertest`): settings in
`Server/servertest.ini`, the world in `Saves/Multiplayer/servertest`, players
in `db/servertest.db`. The listed keys of the `.ini`, plus `WorkshopItems` and
`Mods`, are rewritten every start; everything else is yours.

## Layout

```
supervisor/        dk-entrypoint (Go): PID 1, phases, console, readiness, stop, mods
base/              game-base images: debian, java8/17/21/25, steamcmd
images/<game>/     Dockerfile, game.yaml, hooks/, test/smoke.sh, version.txt
scripts/           smoke-run.sh (runs under DeployKit's hardening), publish.sh
docker-bake.hcl    build graph: bases are named contexts of the games
```

## Developing

```sh
# Supervisor
cd supervisor && go test -race ./...

# Build one image (and its base) into the local daemon
docker buildx bake terraria --set '*.platform=linux/amd64' --set terraria.tags=game-terraria:dev --load

# Start it twice under DeployKit's exact restrictions: ready, console, clean
# stop, and reload without reinstalling
bash images/terraria/test/smoke.sh game-terraria:dev
```

Adding a game means a new `images/<game>/` with a `game.yaml` (phases, start
command, console adapter, ready pattern, stop, `/dk` verbs) and hooks that
read `DK_*`. The image must pass `scripts/smoke-run.sh` before it ships.

## Releases

Pull requests are squash-merged with a Conventional Commit title. On every
push to `master`, release-please keeps one release PR per changed component
(`supervisor`, `base`, each image). Merging it publishes the released images
to GHCR (`X.Y.Z`, `X.Y`, `X`, `latest`), with SBOM and provenance attestations,
signed with cosign (keyless). A `supervisor` or `base` release republishes
every image. **Release → Run workflow** republishes by hand, for example `all`
after an upstream security update.

CI runs the Go tests, govulncheck, shellcheck and hadolint, scans the base
images with Trivy, builds everything, and smoke
tests Terraria, Minecraft and tModLoader on every pull request. Valheim runs
nightly. CS2 needs about 60 GB of disk, so its smoke test only runs when the
`CS2_RUNNER` repository variable names a self-hosted runner that has it.

## Security

- Game processes, hooks and mods run as uid 1000; only `dk-entrypoint`
  starts as root, to prepare `/data`, `/server` and `/run/deploykit`.
  As root it ignores `DK_CONFIG` and `DK_RUN_DIR`.
- The game process does not see `DK_CURSEFORGE_API_KEY`, `DK_MODS_*` or
  `DK_MODPACK_*`: only the install and mods phases need them.
- Mods and modpacks are only downloaded from fixed hosts, always checked
  against a hash, and never written outside the data directory or into
  `/data/.deploykit`.
- Values written into config files (`MOTD`, passwords, names) must not contain
  line breaks.

Images are signed with cosign (keyless, GitHub OIDC). Verify one, preferably
pinned by digest, with:

```sh
cosign verify ghcr.io/deploykithq/game-terraria@sha256:<digest> \
  --certificate-identity-regexp '^https://github.com/deploykithq/deploykit-game-images/\.github/workflows/release\.yml@' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

The release workflow also rebuilds and republishes every image each week, so
Debian, Java and .NET security updates reach the images without waiting for a
release.

## License

MIT
