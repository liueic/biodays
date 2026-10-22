# syntax=docker/dockerfile:1

FROM golang:1.27.3-alpine AS build
RUN apk add --no-cache gcc musl-dev
WORKDIR /src
ENV CGO_ENABLED=1

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -trimpath -ldflags="-s -w" -o /out/biodays-server ./cmd/server \
    && go build -trimpath -ldflags="-s -w" -o /out/biodays-seed ./cmd/seed

FROM alpine:3.22

RUN addgroup -S biodays \
    && adduser -S -G biodays biodays \
    && mkdir -p /data \
    && chown biodays:biodays /data

COPY --from=build /out/biodays-server /usr/local/bin/biodays-server
COPY --from=build /out/biodays-seed /usr/local/bin/biodays-seed

ENV BIODAYS_DB_PATH=/data/biodays.db
WORKDIR /data
USER biodays
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=10s \
    CMD wget -qO- http://127.0.0.1:8080/readyz || exit 1

ENTRYPOINT ["/usr/local/bin/biodays-server"]
