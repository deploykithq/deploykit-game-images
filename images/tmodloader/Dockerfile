# syntax=docker/dockerfile:1.7
# ghcr.io/deploykithq/game-tmodloader — modded Terraria (tModLoader) with
# Steam Workshop mods. tModLoader itself is downloaded from its GitHub
# release on first start; the .NET runtime it needs is part of the image
# (with libicu: its absence is what broke the third-party image).

ARG BASE=ghcr.io/deploykithq/game-base:steamcmd
ARG DOTNET=8.0
FROM mcr.microsoft.com/dotnet/runtime:${DOTNET}-bookworm-slim AS dotnet

FROM ${BASE}
RUN apt-get update \
    && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends libicu72 libssl3 zlib1g \
    && rm -rf /var/lib/apt/lists/*
COPY --from=dotnet /usr/share/dotnet /usr/share/dotnet
RUN ln -s /usr/share/dotnet/dotnet /usr/local/bin/dotnet && dotnet --list-runtimes

ENV DOTNET_ROOT=/usr/share/dotnet \
    DOTNET_CLI_TELEMETRY_OPTOUT=1 \
    DK_PORT_GAME=7777

COPY images/tmodloader/game.yaml /etc/deploykit/game.yaml
COPY images/tmodloader/hooks/ /opt/deploykit/hooks/
RUN chmod 0755 /opt/deploykit/hooks/*.sh

LABEL io.deploykit.game="tmodloader" \
      io.deploykit.console="stdin" \
      org.opencontainers.image.title="DeployKit tModLoader server" \
      org.opencontainers.image.source="https://github.com/deploykithq/deploykit-game-images" \
      org.opencontainers.image.licenses="MIT"
EXPOSE 7777/tcp
