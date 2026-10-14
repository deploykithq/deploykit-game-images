# syntax=docker/dockerfile:1.7
# ghcr.io/deploykithq/game-valheim — Valheim dedicated server (Steam app
# 896660), installed and updated through SteamCMD on start into /server.

ARG BASE=ghcr.io/deploykithq/game-base:steamcmd
FROM ${BASE}

RUN apt-get update \
    && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends libatomic1 libpulse0 \
    && rm -rf /var/lib/apt/lists/*

ENV DK_PORT_GAME=2456

COPY images/valheim/game.yaml /etc/deploykit/game.yaml
COPY images/valheim/hooks/ /opt/deploykit/hooks/
RUN chmod 0755 /opt/deploykit/hooks/*.sh

LABEL io.deploykit.game="valheim" \
      io.deploykit.console="none" \
      org.opencontainers.image.title="DeployKit Valheim server" \
      org.opencontainers.image.source="https://github.com/deploykithq/deploykit-game-images" \
      org.opencontainers.image.licenses="MIT"
EXPOSE 2456/udp 2457/udp
