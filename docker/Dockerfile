# =============================================================================
# Build Stage - Compile Go binary
# =============================================================================
FROM golang:alpine AS builder

RUN apk add --no-cache git bash

ARG TARGETARCH
ARG VERSION=dev
ARG BUILD_DATE
ARG COMMIT_ID
ARG OFFICIAL_SITE=""

WORKDIR /build

COPY go.mod ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build \
    -ldflags "-s -w -X 'main.Version=${VERSION}' -X 'main.CommitID=${COMMIT_ID}' -X 'main.BuildDate=${BUILD_DATE}' -X 'main.OfficialSite=${OFFICIAL_SITE}'" \
    -o /build/binary/cassonic ./src

RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build \
    -ldflags "-s -w -X 'main.Version=${VERSION}' -X 'main.CommitID=${COMMIT_ID}' -X 'main.BuildDate=${BUILD_DATE}' -X 'main.OfficialSite=${OFFICIAL_SITE}'" \
    -o /build/binary/cassonic-cli ./src/client

# =============================================================================
# Runtime Stage - Minimal Alpine image
# =============================================================================
FROM alpine:latest

ARG VERSION=dev
ARG BUILD_DATE
ARG COMMIT_ID
ARG LICENSE=MIT

# No LABEL blocks — all image metadata is applied as OCI annotations at build time.
# Pass --annotation flags to docker buildx build (or use docker/metadata-action
# annotations: output in GitHub Actions). See dockerfile_conventions.md OCI Annotations.

RUN apk add --no-cache \
    git \
    curl \
    bash \
    tini \
    tor

COPY --from=builder /build/binary/cassonic /usr/local/bin/cassonic
COPY --from=builder /build/binary/cassonic-cli /usr/local/bin/cassonic-cli

COPY docker/rootfs/ /

COPY docker/Dockerfile /root/Dockerfile

RUN chmod 755 /usr/local/bin/*

ENV MODE=development \
    TZ=America/New_York \
    DATABASE_DIR=/data/db/sqlite

EXPOSE 80

STOPSIGNAL SIGRTMIN+3

HEALTHCHECK --start-period=10m --interval=5m --timeout=15s --retries=3 \
    CMD /usr/local/bin/cassonic --status || exit 1

ENTRYPOINT [ "tini", "-p", "SIGTERM", "--", "/usr/local/bin/entrypoint.sh" ]
