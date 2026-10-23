# syntax=docker/dockerfile:1

# 构建阶段互不依赖，server-build 与 web-build 由 buildkit 并行执行，
# 整体构建时长 ≈ max(Go 编译, 前端构建) 而不是两者之和。

FROM golang:1.27.3-alpine AS server-build
# cgo（go-sqlite3）需要 gcc。golang:*-alpine 不自带 build-base，这里的 apk add 是必需的：
# 删掉它 CI 会在 go build 阶段报 C compiler "gcc" not found。
# 只装 build-base 一个包，运行阶段不装编译器。
RUN apk add --no-cache --no-progress build-base
WORKDIR /src
ENV CGO_ENABLED=1

COPY server/go.mod server/go.sum ./
# --mount=type=cache 挂在 buildkit 的持久缓存上（跨构建保留），
# 依赖没变时 go mod download 直接命中缓存、不再走网络。
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download

COPY server/ ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -ldflags="-s -w" -o /out/biodays ./cmd/biodays

FROM oven/bun:1.4.4 AS web-build
WORKDIR /app/web

COPY web/package.json web/bun.lock ./
# bun 的全局模块缓存 + Next.js 的构建缓存都挂到 buildkit 缓存上
RUN --mount=type=cache,target=/root/.bun/install/cache \
    bun install --frozen-lockfile

COPY web/ ./
ENV NEXT_TELEMETRY_DISABLED=1
ENV BIODAYS_API_URL=http://127.0.0.1:8080
ENV NEXT_PUBLIC_BIODAYS_DOCS_URL=/swagger/index.html
ARG BIODAYS_SITE_URL=http://localhost:8080
ENV BIODAYS_SITE_URL=$BIODAYS_SITE_URL
# .next/cache（webpack/turbopack 增量缓存）跨构建复用，改少量文件时不必全量重编
RUN --mount=type=cache,target=/app/web/.next/cache \
    bun run build

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

# GIN_MODE 必须显式设为 release：gin 默认以 debug 模式启动，会打印
# [GIN-debug] 警告并开启调试输出。配置读取只认环境变量 GIN_MODE，
# 项目自身没有 set 过它，所以在镜像里固定下来，compose 仍可按需覆盖。
ENV NODE_ENV=production \
    GIN_MODE=release \
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
