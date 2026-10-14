# syntax=docker/dockerfile:1.7
#
# game-base:steamcmd — game-base:debian plus SteamCMD, for games whose
# dedicated server is distributed through Steam. amd64 only (Steam).
#
#   docker buildx build -f base/steamcmd/Dockerfile --build-arg BASE=game-base:debian -t game-base:steamcmd .

ARG BASE=ghcr.io/deploykithq/game-base:debian
FROM ${BASE}

RUN dpkg --add-architecture i386 \
    && apt-get update \
    && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
        lib32gcc-s1 lib32stdc++6 libc6-i386 \
    && rm -rf /var/lib/apt/lists/*

COPY base/steamcmd/steam.sh /opt/deploykit/lib/steam.sh

# SteamCMD updates itself on first run; doing it here keeps that off every
# container start. It runs as dk because it rewrites its own directory.
RUN mkdir -p /opt/steamcmd && chown dk:dk /opt/steamcmd \
    && chmod 0755 /opt/deploykit/lib/steam.sh \
    && su dk -s /bin/sh -c 'curl -fsSL https://steamcdn-a.akamaihd.net/client/installer/steamcmd_linux.tar.gz | tar -xz -C /opt/steamcmd \
        && /opt/steamcmd/steamcmd.sh +quit || true' \
    && test -x /opt/steamcmd/linux32/steamcmd
