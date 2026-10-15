# syntax=docker/dockerfile:1.7
# ghcr.io/deploykithq/game-project-zomboid — Project Zomboid dedicated server
# (Steam app 380870), installed and updated through SteamCMD on start into
# /server. The server ships its own JRE.

ARG BASE=ghcr.io/deploykithq/game-base:steamcmd
FROM ${BASE}

ENV DK_PORT_GAME=16261 \
    DK_PORT_DIRECT=16262

COPY images/project-zomboid/game.yaml /etc/deploykit/game.yaml
COPY images/project-zomboid/hooks/ /opt/deploykit/hooks/
RUN chmod 0755 /opt/deploykit/hooks/*.sh

LABEL io.deploykit.game="project-zomboid" \
      io.deploykit.console="stdin" \
      org.opencontainers.image.title="DeployKit Project Zomboid server" \
      org.opencontainers.image.source="https://github.com/deploykithq/deploykit-game-images" \
      org.opencontainers.image.licenses="MIT"
EXPOSE 16261/udp 16262/udp
