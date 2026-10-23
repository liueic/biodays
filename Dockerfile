# syntax=docker/dockerfile:1

# CI publishes linux/amd64. Go/SQLite and Next.js build independently.
# Debian 13/glibc on both sides of CGO; Alpine/musl binaries are incompatible.
FROM golang:1.27.3-trixie AS server-build
WORKDIR /src
ENV CGO_ENABLED=1
COPY server/go.mod server/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download
COPY server/ ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -ldflags="-s -w" -o /out/biodays ./cmd/biodays

# Git-tracked JSON is the source of truth; SQLite is a build artifact.
FROM server-build AS data-build
COPY data/ /seed/
RUN mkdir -p /out/data \
    && BIODAYS_DATA_DIR=/seed BIODAYS_DB_PATH=/out/data/biodays.db \
       /out/biodays seed --strict

FROM oven/bun:1.4.4 AS web-build
WORKDIR /app/web
COPY web/package.json web/bun.lock ./
RUN --mount=type=cache,target=/root/.bun/install/cache \
    bun install --frozen-lockfile
COPY web/ ./
COPY data/ /app/data/
ENV NEXT_TELEMETRY_DISABLED=1
ENV NEXT_PUBLIC_BIODAYS_DOCS_URL=/swagger/index.html
ARG BIODAYS_SITE_URL=http://localhost:8080
ENV BIODAYS_SITE_URL=$BIODAYS_SITE_URL
RUN --mount=type=cache,target=/app/web/.next/cache \
    bun run build && mkdir -p /app/web/public

FROM gcr.io/distroless/nodejs24-debian13:nonroot
COPY --from=server-build /out/biodays /usr/local/bin/biodays
COPY --from=data-build --chown=65532:65532 /out/data/ /data/
COPY --from=web-build --chown=65532:65532 /app/web/.next/standalone/ /app/
COPY --from=web-build --chown=65532:65532 /app/web/.next/static/ /app/.next/static/
COPY --from=web-build --chown=65532:65532 /app/web/public/ /app/public/
COPY --chown=65532:65532 deploy/launcher.js deploy/healthcheck.js /app/
ENV NODE_ENV=production \
    GIN_MODE=release \
    HOSTNAME=0.0.0.0 \
    PORT=3000 \
    BIODAYS_PORT=8080 \
    BIODAYS_DB_PATH=/data/biodays.db \
    BIODAYS_API_URL=http://127.0.0.1:8080 \
    BIODAYS_CORS_ORIGINS=* \
    BIODAYS_TRUSTED_PROXIES=127.0.0.1,::1 \
    BIODAYS_SITE_URL=http://localhost:8080 \
    PATH=/nodejs/bin:/usr/local/bin:/usr/bin:/bin
WORKDIR /app
USER 65532:65532
EXPOSE 3000
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
    CMD ["/nodejs/bin/node", "/app/healthcheck.js"]
ENTRYPOINT ["/nodejs/bin/node", "/app/launcher.js"]
