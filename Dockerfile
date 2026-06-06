# syntax=docker/dockerfile:1

# --- Stage 1: build the web frontend ---
FROM node:20-alpine AS web
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# --- Stage 2: build the Go backend (CGO for go-sqlite3) ---
FROM golang:1.25-bookworm AS backend
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
ENV CGO_ENABLED=1
RUN go build -ldflags="-s -w" -o /out/unidisk ./cmd/unidisk

# --- Stage 3: minimal runtime ---
FROM debian:bookworm-slim
LABEL org.opencontainers.image.title="UniDisk" \
      org.opencontainers.image.description="Self-hosted storage aggregation — combine multiple cloud accounts into one pool." \
      org.opencontainers.image.source="https://github.com/Kayamii/UniDisk" \
      org.opencontainers.image.licenses="MIT"

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=backend /out/unidisk /app/unidisk
COPY --from=web /web/dist /app/web/dist
ENV UNIDISK_DATA_DIR=/data \
    UNIDISK_WEB_DIR=/app/web/dist
VOLUME /data
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD curl -fsS http://localhost:8080/api/health || exit 1
ENTRYPOINT ["/app/unidisk"]
