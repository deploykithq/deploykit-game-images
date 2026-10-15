# syntax=docker/dockerfile:1.7
# ghcr.io/deploykithq/game-eco — Eco dedicated server (Steam app 739590),
# installed and updated through SteamCMD on start into /server.

ARG BASE=ghcr.io/deploykithq/game-base:steamcmd
FROM ${BASE}

# The server is a self-contained .NET single-file app: it needs the native
# libraries .NET does (libicu, libssl), plus libgdiplus for its map renderer.
RUN apt-get update \
    && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
        libicu72 libssl3 zlib1g libgdiplus \
    && rm -rf /var/lib/apt/lists/*

ENV DK_PORT_GAME=3000 \
    DK_PORT_WEB=3001 \
    DOTNET_CLI_TELEMETRY_OPTOUT=1

COPY images/eco/game.yaml /etc/deploykit/game.yaml
COPY images/eco/hooks/ /opt/deploykit/hooks/
RUN chmod 0755 /opt/deploykit/hooks/*.sh

LABEL io.deploykit.game="eco" \
      io.deploykit.console="rcon" \
      org.opencontainers.image.title="DeployKit Eco server" \
      org.opencontainers.image.source="https://github.com/deploykithq/deploykit-game-images" \
      org.opencontainers.image.licenses="MIT"
EXPOSE 3000/udp 3001/tcp
