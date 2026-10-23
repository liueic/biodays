# syntax=docker/dockerfile:1

FROM golang:1.27.3-alpine AS server-build
RUN apk add --no-cache gcc musl-dev
WORKDIR /src
ENV CGO_ENABLED=1

COPY server/go.mod server/go.sum ./
RUN go mod download

COPY server/ ./
RUN go build -trimpath -ldflags="-s -w" -o /out/biodays ./cmd/biodays

FROM oven/bun:1.4.4 AS web-build
WORKDIR /app/web

COPY web/package.json web/bun.lock ./
RUN bun install --frozen-lockfile

COPY web/ ./
ENV NEXT_TELEMETRY_DISABLED=1
ENV BIODAYS_API_URL=http://127.0.0.1:8080
ENV NEXT_PUBLIC_BIODAYS_DOCS_URL=/swagger/index.html
ARG BIODAYS_SITE_URL=http://localhost:8080
ENV BIODAYS_SITE_URL=$BIODAYS_SITE_URL
RUN bun run build

FROM node:24-alpine
RUN apk add --no-cache ca-certificates \
    && addgroup -S biodays \
    && adduser -S -G biodays biodays \
    && mkdir -p /data /app /app/public \
    && chown biodays:biodays /data /app

COPY --from=server-build /out/biodays /usr/local/bin/biodays
COPY --from=web-build --chown=biodays:biodays /app/web/.next/standalone/ /app/
COPY --from=web-build --chown=biodays:biodays /app/web/.next/static/ /app/.next/static/
# web/public 是可选的静态资源目录，仓库里还没有它就跳过 COPY；
# 直接 COPY 会因源路径不存在让整个镜像构建失败（/app/public 已由上面的 mkdir 兜底）。
# 路径写成 glob 后，目录缺失时 buildkit 会退化成空拷贝而不会报错。
COPY --from=web-build --chown=biodays:biodays /app/web/public* /app/public/
COPY --chown=biodays:biodays deploy/launcher.js /app/launcher.js

ENV NODE_ENV=production \
    HOSTNAME=0.0.0.0 \
    PORT=3000 \
    BIODAYS_PORT=8080 \
    BIODAYS_DB_PATH=/data/biodays.db \
    BIODAYS_API_URL=http://127.0.0.1:8080 \
    BIODAYS_CORS_ORIGINS=* \
    BIODAYS_TRUSTED_PROXIES=127.0.0.1,::1 \
    BIODAYS_SITE_URL=http://localhost:8080

WORKDIR /app
USER biodays
EXPOSE 3000

HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
    CMD wget -qO- http://127.0.0.1:3000/readyz || exit 1

ENTRYPOINT ["node", "/app/launcher.js"]
