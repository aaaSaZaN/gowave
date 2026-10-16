# syntax=docker/dockerfile:1

# Stage 1: Build frontend
FROM node:22-alpine AS frontend

WORKDIR /src/frontend
RUN npm install -g pnpm@11

COPY frontend/package.json frontend/pnpm-lock.yaml frontend/pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile

COPY frontend/ ./
RUN pnpm run build

# Stage 2: Build Go backend (same entrypoint and version flag as release.yml)
FROM golang:1.27-alpine AS backend

ARG VERSION=dev
WORKDIR /src

COPY backend-go/go.mod backend-go/go.sum ./
RUN go mod download

COPY backend-go/ ./
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags="-s -w -X gowave/internal/system.AppVersion=${VERSION}" \
    -o /out/gowave ./cmd/server

# Stage 3: Minimal runtime
FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=backend /out/gowave /app/gowave
COPY --from=frontend /src/frontend/dist /app/frontend/dist

ENV APP_PORT=3000 \
    METRICS_PORT=3001 \
    IN_DOCKER=true \
    TZ=UTC

EXPOSE 3000 3001

ENTRYPOINT ["/app/gowave"]
