# syntax=docker/dockerfile:1

# Backend-only image for split deployments. The default deployment uses the
# repository-root Dockerfile, which bundles the Next.js frontend and Go API.

FROM golang:1.27.3-alpine AS build
# 同根 Dockerfile：cgo 编译 go-sqlite3 需要 gcc/musl-dev，alpine 版 golang 镜像不自带。
RUN apk add --no-cache --no-progress build-base
WORKDIR /src
ENV CGO_ENABLED=1

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download

COPY . .
# 只编 linux/amd64 时保持原生编译；多架构构建请走根目录 Dockerfile（那边用 C 交叉编译）。
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -ldflags="-s -w" -o /out/biodays ./cmd/biodays

FROM alpine:3.22

RUN addgroup -S biodays \
    && adduser -S -G biodays biodays \
    && mkdir -p /data \
    && chown biodays:biodays /data

COPY --from=build /out/biodays /usr/local/bin/biodays

# gin 未设 GIN_MODE 时默认 debug 模式，镜像里固定为 release。
ENV BIODAYS_DB_PATH=/data/biodays.db \
    GIN_MODE=release
WORKDIR /data
USER biodays
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=10s \
    CMD wget -qO- http://127.0.0.1:8080/readyz || exit 1

ENTRYPOINT ["/usr/local/bin/biodays"]
CMD ["serve"]
