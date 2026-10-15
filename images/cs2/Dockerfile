# syntax=docker/dockerfile:1.7
# ghcr.io/deploykithq/game-cs2 — Counter-Strike 2 dedicated server (Steam app
# 730, ~60 GB), installed and updated through SteamCMD on start into /server.

ARG BASE=ghcr.io/deploykithq/game-base:steamcmd
FROM ${BASE}

RUN apt-get update \
    && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends libicu72 lib32z1 \
    && rm -rf /var/lib/apt/lists/*

ENV DK_PORT_GAME=27015

COPY images/cs2/game.yaml /etc/deploykit/game.yaml
COPY images/cs2/hooks/ /opt/deploykit/hooks/
RUN chmod 0755 /opt/deploykit/hooks/*.sh

LABEL io.deploykit.game="cs2" \
      io.deploykit.console="rcon" \
      org.opencontainers.image.title="DeployKit Counter-Strike 2 server" \
      org.opencontainers.image.source="https://github.com/deploykithq/deploykit-game-images" \
      org.opencontainers.image.licenses="MIT"
EXPOSE 27015/udp
