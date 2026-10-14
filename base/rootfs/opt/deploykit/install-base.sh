#!/bin/sh
# Runs once at image build time on any apt-based distribution.
set -eu

export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get install -y --no-install-recommends \
    bash ca-certificates coreutils curl jq procps tzdata unzip
rm -rf /var/lib/apt/lists/*

# Ubuntu-based images (eclipse-temurin) ship an `ubuntu` user on 1000.
if getent passwd 1000 >/dev/null; then
    userdel -r "$(getent passwd 1000 | cut -d: -f1)" 2>/dev/null || true
fi
if getent group 1000 >/dev/null; then
    groupdel "$(getent group 1000 | cut -d: -f1)" 2>/dev/null || true
fi
groupadd -g 1000 dk
useradd -u 1000 -g 1000 -m -d /home/dk -s /bin/bash dk

mkdir -p /data /server /run/deploykit /etc/deploykit /opt/deploykit/hooks
chown dk:dk /data /server /run/deploykit
chmod 0755 /opt/deploykit/lib/*.sh
