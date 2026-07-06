# syntax=docker/dockerfile:1

# Stage 1: Build frontend
FROM node:22-alpine AS frontend

WORKDIR /app

# Copy workspace package files first for layer caching
COPY package.json package-lock.json ./
COPY webui/app/package.json webui/app/
COPY webui/components/package.json webui/components/
COPY webui/website/package.json webui/website/

RUN npm ci --workspaces

COPY webui/ webui/

RUN npm run build -w @hister/app

# Stage 2: Build Go binary
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=frontend /app/webui/app/build/ server/static/app/

RUN set -eux; \
    LISTEN_ADDRESS="0.0.0.0:4433"; \
    BASE_URL="http://localhost:4433"; \
    CGO_ENABLED=1 go build \
    -tags netgo,osusergo \
    -ldflags "\
      -linkmode external -extldflags '-static' -s -w \
      -X 'github.com/asciimoo/hister/config.DefaultServerAddress=$LISTEN_ADDRESS' \
      -X 'github.com/asciimoo/hister/config.DefaultServerBaseURL=$BASE_URL'" \
    -o hister .

# Stage 3: Download the latest yt-dlp release binary
FROM alpine:3.21 AS ytdlp

ARG TARGETARCH=amd64

RUN set -eux; \
    apk add --no-cache ca-certificates wget; \
    case "$TARGETARCH" in \
      amd64) asset="yt-dlp_musllinux" ;; \
      arm64) asset="yt-dlp_musllinux_aarch64" ;; \
      *) echo "unsupported TARGETARCH for yt-dlp: $TARGETARCH" >&2; exit 1 ;; \
    esac; \
    wget -O /tmp/yt-dlp "https://github.com/yt-dlp/yt-dlp/releases/latest/download/${asset}"; \
    wget -O /tmp/SHA2-256SUMS "https://github.com/yt-dlp/yt-dlp/releases/latest/download/SHA2-256SUMS"; \
    grep "  ${asset}$" /tmp/SHA2-256SUMS | sed "s/  ${asset}$/  \\/tmp\\/yt-dlp/" | sha256sum -c -; \
    mkdir -p /usr/local/bin; \
    mv /tmp/yt-dlp /usr/local/bin/yt-dlp; \
    chmod 0755 /usr/local/bin/yt-dlp

# Release stage (nonroot)
# latest & vx.x.x
FROM alpine:3.21 AS release

LABEL org.opencontainers.image.title="Hister" \
      org.opencontainers.image.description="Self-hosted browser history search engine" \
      org.opencontainers.image.source="https://github.com/asciimoo/hister" \
      org.opencontainers.image.licenses="AGPL-3.0"

WORKDIR /hister

RUN adduser -D -u 65532 hister && mkdir -p /hister/data && chown -R 65532:65532 /hister
COPY --from=ytdlp /usr/local/bin/yt-dlp /usr/local/bin/yt-dlp
COPY --chown=65532:65532 --from=builder /app/hister .
USER 65532:65532

ENV HISTER_DATA_DIR=/hister/data
ENV HISTER_CONFIG=/hister/data/config.yml

EXPOSE 4433

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO /dev/null http://localhost:4433/ || exit 1

ENTRYPOINT ["/hister/hister"]
CMD ["listen"]

# Release stage (root)
# latest-root & vx.x.x-root
FROM alpine:3.21 AS root
WORKDIR /hister

RUN mkdir -p /hister/data
COPY --from=ytdlp /usr/local/bin/yt-dlp /usr/local/bin/yt-dlp
COPY --from=builder /app/hister .

USER root

ENV HISTER_DATA_DIR=/hister/data
ENV HISTER_CONFIG=/hister/data/config.yml

EXPOSE 4433

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO /dev/null http://localhost:4433/ || exit 1

ENTRYPOINT ["/hister/hister"]
CMD ["listen"]

# Release stage (debug)
# latest-debug & vx.x.x-debug
FROM alpine:3.21 AS debug
WORKDIR /hister

RUN apk add --no-cache curl bash && mkdir -p /hister/data
COPY --from=ytdlp /usr/local/bin/yt-dlp /usr/local/bin/yt-dlp

COPY --from=builder /app/hister .

USER root

ENV HISTER_DATA_DIR=/hister/data
ENV HISTER_CONFIG=/hister/data/config.yml

EXPOSE 4433

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO /dev/null http://localhost:4433/ || exit 1

ENTRYPOINT ["/hister/hister"]
CMD ["listen"]
