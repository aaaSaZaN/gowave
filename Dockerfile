# syntax=docker/dockerfile:1

# Stage 1: Build frontend
# Frontend output is architecture-independent: build it once, natively.
ARG BUN_VERSION=1.4.2
FROM --platform=$BUILDPLATFORM docker.io/oven/bun:${BUN_VERSION}-alpine AS frontend

WORKDIR /src/frontend
COPY frontend/package.json frontend/bun.lock ./
RUN bun install --frozen-lockfile

COPY frontend/ ./
# The Xray validator runtime is fetched (hash-pinned) and shipped precompressed.
RUN bun run editor:download && bun run build && bun run editor:compress

# Stage 2: Build Go backend (same entrypoint and version flag as release.yml).
# Cross-compile natively instead of running the compiler under QEMU.
FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS backend

ARG VERSION=dev
# CHANNEL=dev marks builds of the dev channel (Docker tag "dev"); see dev.yml.
ARG CHANNEL=stable
ARG COMMIT_SHA=
ARG TARGETOS TARGETARCH
WORKDIR /src

COPY backend-go/go.mod backend-go/go.sum ./
RUN go mod download

COPY backend-go/ ./
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath \
    -ldflags="-s -w -X gowave/internal/system.AppVersion=${VERSION} -X gowave/internal/system.Channel=${CHANNEL} -X gowave/internal/system.CommitSha=${COMMIT_SHA}" \
    -o /out/gowave ./cmd/server

# Stage 3: Minimal runtime
FROM alpine:3.24

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S -g 10001 gowave \
    && adduser -S -D -H -u 10001 -G gowave gowave

WORKDIR /app

COPY --from=backend /out/gowave /app/gowave
COPY --from=frontend /src/frontend/dist /app/frontend/dist

ENV APP_HOST=0.0.0.0 \
    APP_PORT=3000 \
    METRICS_PORT=3001 \
    IN_DOCKER=true \
    TZ=UTC

EXPOSE 3000 3001

USER 10001:10001

ENTRYPOINT ["/app/gowave"]
