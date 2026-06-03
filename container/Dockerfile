# ── Build stage ──────────────────────────────────────────────────────────────
FROM golang:1.25 AS builder

WORKDIR /app
ENV CGO_ENABLED=1

COPY go.mod go.sum ./
RUN go mod download

COPY admin       ./admin
COPY compression ./compression
COPY core        ./core
COPY server      ./server
COPY storage     ./storage
COPY transport   ./transport
COPY main.go     .

RUN go build -o /app/mobile-db .

# ── Runtime stage ─────────────────────────────────────────────────────────────
FROM debian:bookworm-slim

# Check https://github.com/vlcn-io/cr-sqlite/releases for available versions.
ARG CRSQLITE_VERSION=0.16.1
ARG TARGETARCH

RUN apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates \
      curl \
      unzip \
    && rm -rf /var/lib/apt/lists/*

RUN mkdir -p /app

# Download the cr-sqlite shared extension for the target architecture.
RUN set -eux; \
    case "${TARGETARCH}" in \
      amd64) _ARCH="x86_64"  ;; \
      arm64) _ARCH="aarch64" ;; \
      *)     echo "Unsupported TARGETARCH: ${TARGETARCH}" >&2; exit 1 ;; \
    esac; \
    curl -fsSL \
      "https://github.com/vlcn-io/cr-sqlite/releases/download/v${CRSQLITE_VERSION}/crsqlite-linux-${_ARCH}.zip" \
      -o /tmp/crsqlite.zip; \
    unzip /tmp/crsqlite.zip -d /tmp/crsqlite; \
    find /tmp/crsqlite -name "crsqlite.so" -exec mv {} /app/crsqlite.so \;; \
    rm -rf /tmp/crsqlite /tmp/crsqlite.zip

# Remove curl/unzip — no longer needed at runtime.
RUN apt-get purge -y curl unzip && apt-get autoremove -y && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=builder /app/mobile-db /app/mobile-db
RUN mkdir -p /data

ENV CRSQLITE_EXT_PATH=/app/crsqlite.so
ENV DATA_DIR=/data
ENV LISTEN_ADDR=:8443

EXPOSE 8443

ENTRYPOINT ["/app/mobile-db"]
