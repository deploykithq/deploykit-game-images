# DeployKit game image contract — v1

Every image in this repository behaves the same way towards DeployKit. A
DeployKit game definition (`games/<id>/game.json` in `deploykit-templates`)
that targets a contract image only has to pass values; it never has to know
how the game is installed, how its console works or where it keeps its files.

Images declare the contract with the label `io.deploykit.contract=1`.

## Runtime constraints the image must survive

These are what DeployKit applies to every game container
(`apps/api/src/lib/game-isolation.ts`, `services/game-server.ts`,
`services/docker.ts`):

| Constraint | Consequence for the image |
|---|---|
| `no-new-privileges` | Privileges cannot be dropped through setuid binaries (`gosu`, `su`); the entrypoint drops them with syscalls. |
| Capabilities `NET_RAW`, `MKNOD`, `AUDIT_WRITE`, `SYS_CHROOT`, `SETFCAP`, `SETPCAP` dropped | `CHOWN`, `SETUID`, `SETGID` remain, which is exactly what the entrypoint needs. |
| `pids-limit 2048` | Fine for every game here. |
| No TTY, `OpenStdin` | The console is a pipe on PID 1's stdin. |
| Own network `deploykit-games` | Nothing else on it but the API. |
| Memory limit with swap equal to it | The heap is sized from the limit (`DK_HEAP_MB`). |

## PID 1: `dk-entrypoint`

1. Starts as root, creates `/data`, the server directory and `/run/deploykit`,
   and `chown`s them to `dk` (**1000:1000**) when their owner differs.
2. Drops to 1000:1000 (`setgroups`, `setgid`, `setuid`) — the supervisor and
   everything it starts run unprivileged.
3. Runs the image's phases (`install`, `configure`, `mods`… — hooks in
   `/opt/deploykit/hooks`), then starts the game in its own process group.
4. Reaps every orphan (it is PID 1) and exits with the game's exit code.

The image's behaviour is described by `/etc/deploykit/game.yaml`, which is an
implementation detail of the image and not part of the contract.

## Volumes

| Path | Holds | DeployKit treats it as |
|---|---|---|
| `/data` | World, configuration, mods, `.deploykit/` state | **primary**, browsable, backed up |
| `/server` | Game binaries (SteamCMD install, server jars…) — re-downloadable | not browsable, not backed up |

`/server` is optional: if it is not a mount but `/data` is, the binaries go to
`/data/.server`. Mount it for Steam games, so a backup does not carry a 60 GB
install.

`/data/.deploykit/` belongs to the image (install stamps, mod locks). Do not
edit it; deleting it forces a reinstall on the next start.

## Environment (inputs)

Everything DeployKit passes starts with `DK_`. Unset optional values take the
documented default of the image.

| Variable | Meaning |
|---|---|
| `DK_SERVER_NAME` | Display name of the server (informational). |
| `DK_MEMORY_MB` | Memory limit. Read from the cgroup when unset. |
| `DK_HEAP_MB` | Heap for JVM/.NET games. Default: 75 % of `DK_MEMORY_MB`, else 1024. |
| `DK_PORT_<NAME>` | Port the game must **listen on inside the container** for port `<name>` (`DK_PORT_GAME`…). DeployKit sets it to the host port for `matchHost` ports and to the container port otherwise. |
| `DK_GAME_VERSION` | Game/server version where the game has one (`LATEST`, `1.21.4`, `1.4.5.8`, `v2026.08.3.0`). |
| `DK_VAR_<KEY>` | A form field of the game definition, key upper-cased (`world_name` → `DK_VAR_WORLD_NAME`). |
| `DK_MODS_MODRINTH` | Comma-separated `project[:version]`. |
| `DK_MODS_CURSEFORGE` | Comma-separated `modId[:fileId]`. |
| `DK_MODS_STEAM` | Comma-separated Workshop ids to keep downloaded. |
| `DK_MODS_STEAM_ENABLED` | The subset of `DK_MODS_STEAM` to enable. |
| `DK_CURSEFORGE_API_KEY` | Required only when a CurseForge list or modpack is set. |
| `DK_GITHUB_TOKEN` | Optional. Images that look up a release on api.github.com (tModLoader) use it to avoid the anonymous rate limit of a shared IP. Any token, no scopes. |
| `DK_MODPACK_SOURCE` | `modrinth` or `curseforge`. Empty: no modpack (an installed one is removed). |
| `DK_MODPACK_PROJECT` / `DK_MODPACK_VERSION` | The pack, and its version/file id (empty: latest, pinned at install). |
| `DK_STOP_TIMEOUT` | Seconds (or a Go duration) the game gets to stop gracefully. Keep it below Docker's `StopTimeout`. |

The mod list format is the one `services/game-mods-env.ts` already renders.
**Every mod variable is meaningful even when empty:** an empty list removes
every mod the image installed before. Files a user uploaded by hand are never
touched — the image only removes what its lock file says it installed.

`DK_RCON_PASSWORD` is **not** an input. Images whose console is RCON generate
it at every boot; it never leaves the container.

## Console

Commands are written to **PID 1's stdin**, one per line — the `stdin` console
method DeployKit already implements (a hijacked `docker attach`). The
supervisor forwards each line to the game: through the game's own stdin, over
container-local RCON, or not at all (`[deploykit] console error: this game has
no console`). DeployKit never needs RCON or `docker exec` for a contract image.

Reserved lines start with `/dk`:

| Line | Effect |
|---|---|
| `/dk status` | Prints `[deploykit] status game=… state=… ready=… pid=… uptime=… console=…`. |
| `/dk save` | Flushes the world to disk (game-specific command). |
| `/dk save-off` / `/dk save-on` | Suspends / resumes autosave around a backup. |
| `/dk stop` | Graceful stop, as on SIGTERM. |

Each prints `[deploykit] verb <verb> ok` or `[deploykit] verb <verb> unsupported`,
so a backup can use the same `preCommands` (`/dk save-off`, `/dk save`) and
`postCommands` (`/dk save-on`) for every game.

## Logs

The game's output is passed through untouched. The supervisor adds lines with
the fixed prefix `[deploykit] `, always on a line of their own:

| Line | When |
|---|---|
| `[deploykit] game=<id> contract=1` | Supervisor started. |
| `[deploykit] phase=<name>` | A phase (`install`, `configure`, `mods`, `start`) begins. |
| `[deploykit] phase=<name> failed code=<n>` | A phase failed; the container exits with `<n>`. |
| `[deploykit] ready` | The game is accepting players. **Use `\[deploykit\] ready` as `readyPattern`.** |
| `[deploykit] stopping <why>` | Graceful stop started. |
| `[deploykit] stopped code=<n>` | Game exited; the container exits with `<n>`. |
| `[deploykit] error: …` | A hook rejected its input, with the reason. |

Readiness is also a file, `/run/deploykit/ready`, which the image's
`HEALTHCHECK` (`dk-entrypoint health`) reads.

## Stopping

On SIGTERM or SIGINT (or `/dk stop`) the supervisor asks the game to stop the
way it saves cleanly (`stop`, `exit`, `quit` through the console, or SIGINT
for Valheim), waits up to the stop timeout, then sends SIGTERM to the game's
process group and, 5 s later, SIGKILL. A stop during a phase (a long
download) aborts it immediately with exit code 143.

## Versioning

Breaking any rule above is a new contract version (`io.deploykit.contract=2`),
never a silent change to v1.
