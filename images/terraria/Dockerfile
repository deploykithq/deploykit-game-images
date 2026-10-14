# syntax=docker/dockerfile:1.7
# ghcr.io/deploykithq/game-terraria — vanilla Terraria dedicated server.
# The server itself is downloaded from terraria.org on first start into
# /server (DK_GAME_VERSION), never baked into the image.

ARG BASE=ghcr.io/deploykithq/game-base:debian
FROM ${BASE}

# The default version, and the sha256 its download is checked against. Other
# versions (DK_GAME_VERSION) are fetched from the same official URL.
ENV TERRARIA_DEFAULT_VERSION=1.4.5.8 \
    TERRARIA_DEFAULT_SHA256=f513a4ac9789d34af766291ae217c9cd7d9472e13782a0e2b17512f70d7a8334 \
    DK_PORT_GAME=7777

COPY images/terraria/game.yaml /etc/deploykit/game.yaml
COPY images/terraria/hooks/ /opt/deploykit/hooks/
RUN chmod 0755 /opt/deploykit/hooks/*.sh

LABEL io.deploykit.game="terraria" \
      io.deploykit.console="stdin" \
      org.opencontainers.image.title="DeployKit Terraria server" \
      org.opencontainers.image.source="https://github.com/deploykithq/deploykit-game-images" \
      org.opencontainers.image.licenses="MIT"
EXPOSE 7777/tcp
