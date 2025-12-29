# syntax=docker/dockerfile:1

ARG GO_VERSION=1.25.3
ARG NODE_VERSION=20.18.0
ARG ALPINE_VERSION=3.20
ARG PNPM_VERSION=9.12.2

FROM node:${NODE_VERSION}-alpine AS ui-builder
WORKDIR /app/ui
RUN npm install -g pnpm@${PNPM_VERSION}
COPY ui/package.json ui/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY ui/ ./
RUN pnpm build

FROM golang:${GO_VERSION}-alpine AS go-builder
ARG VERSION=dev
WORKDIR /app
RUN apk add --no-cache gcc musl-dev
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=ui-builder /app/public ./public
RUN go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /app/bin/valora ./cmd/valora

FROM alpine:${ALPINE_VERSION}
RUN apk add --no-cache ca-certificates tzdata
RUN adduser -D -u 10001 valora
WORKDIR /app
COPY --from=go-builder --chown=valora:valora /app/bin/valora /app/valora
COPY --from=go-builder --chown=valora:valora /app/public /app/public
ENV GIN_MODE=release
EXPOSE 8080
USER valora
ENTRYPOINT ["/app/valora"]
