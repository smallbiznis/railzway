# syntax=docker/dockerfile:1.5

ARG GO_VERSION=1.25.3
ARG NODE_VERSION=20.18.0
ARG ALPINE_VERSION=3.20
ARG PNPM_VERSION=9.12.2

# =========================
# UI BUILDER
# =========================
FROM node:${NODE_VERSION}-alpine AS ui-builder
WORKDIR /app/ui

RUN npm install -g pnpm@${PNPM_VERSION}

# copy lockfile first (cache-friendly)
COPY ui/package.json ui/pnpm-lock.yaml ./

# 🔥 pnpm cache (ARM64 critical)
RUN --mount=type=cache,id=pnpm-store,target=/root/.local/share/pnpm/store \
    pnpm install --frozen-lockfile

# copy source last
COPY ui/ ./

RUN pnpm build

# =========================
# GO BUILDER
# =========================
FROM golang:${GO_VERSION}-alpine AS go-builder
ARG VERSION=dev

WORKDIR /app
RUN apk add --no-cache gcc musl-dev

# go deps cache
COPY go.mod go.sum ./
RUN go mod download

# copy backend source only
COPY cmd ./cmd
COPY internal ./internal
COPY pkg ./pkg
COPY configs ./configs
COPY main.go ./

# copy UI build output
COPY --from=ui-builder /app/ui/public ./public

RUN go build \
    -trimpath \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /app/bin/valora ./cmd/valora

# =========================
# RUNTIME
# =========================
FROM alpine:${ALPINE_VERSION}

RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -u 10001 valora

WORKDIR /app

COPY --from=go-builder --chown=valora:valora /app/bin/valora /app/valora
COPY --from=go-builder --chown=valora:valora /app/public /app/public

ENV GIN_MODE=release
EXPOSE 8080

USER valora
ENTRYPOINT ["/app/valora"]
