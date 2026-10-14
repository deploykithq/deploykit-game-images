# syntax=docker/dockerfile:1.7
#
# The DeployKit game base: dk-entrypoint (PID 1), the shared hook library and
# the unprivileged `dk` user (1000:1000), on top of any apt-based image.
#
#   docker buildx build -f base/Dockerfile --build-arg FROM=debian:bookworm-slim -t game-base:debian .
#   docker buildx build -f base/Dockerfile --build-arg FROM=eclipse-temurin:21-jre-noble -t game-base:java21 .
#
# Build context is the repository root.

ARG FROM=debian:bookworm-slim

FROM --platform=$BUILDPLATFORM golang:1.23-bookworm AS supervisor
WORKDIR /src
COPY supervisor/go.mod supervisor/go.sum ./
RUN go mod download
COPY supervisor/ ./
ARG TARGETOS TARGETARCH
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags "-s -w -X main.Version=${VERSION}" -o /out/dk-entrypoint ./cmd/dk-entrypoint

FROM ${FROM}
COPY base/rootfs/ /
RUN sh /opt/deploykit/install-base.sh
COPY --from=supervisor /out/dk-entrypoint /usr/local/bin/dk-entrypoint

ENV DK_RUN_DIR=/run/deploykit \
    LANG=C.UTF-8
LABEL io.deploykit.contract="1"
WORKDIR /data
HEALTHCHECK --interval=30s --timeout=5s --start-period=10m --retries=3 CMD ["dk-entrypoint", "health"]
ENTRYPOINT ["dk-entrypoint"]
