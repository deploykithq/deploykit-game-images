# syntax=docker/dockerfile:1.7
# ghcr.io/deploykithq/game-minecraft-java:<java8|java17|java21|java25>
#
# Minecraft: Java Edition — vanilla, Paper, Fabric, Forge or NeoForge — with
# Modrinth/CurseForge mods and modpacks. The server jar is downloaded from the
# official sources on first start, never baked into the image.

ARG BASE=ghcr.io/deploykithq/game-base:java21
FROM ${BASE}

ENV DK_PORT_GAME=25565

COPY images/minecraft-java/game.yaml /etc/deploykit/game.yaml
COPY images/minecraft-java/hooks/ /opt/deploykit/hooks/
RUN chmod 0755 /opt/deploykit/hooks/*.sh

LABEL io.deploykit.game="minecraft-java" \
      io.deploykit.console="stdin" \
      org.opencontainers.image.title="DeployKit Minecraft: Java Edition server" \
      org.opencontainers.image.source="https://github.com/deploykithq/deploykit-game-images" \
      org.opencontainers.image.licenses="MIT"
EXPOSE 25565/tcp
