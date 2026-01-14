# syntax=docker/dockerfile:1.5

# =========================
# FRONTEND BUILDER (Admin Dashboard)
# =========================
FROM node:20-alpine AS frontend-builder
WORKDIR /app
COPY apps/admin/package.json apps/admin/pnpm-lock.yaml ./
RUN npm install -g pnpm && pnpm install --frozen-lockfile
COPY apps/admin/ ./
RUN pnpm run build

# =========================
# BACKEND BUILDER (Monolith Binary)
# =========================
FROM golang:1.25-alpine AS backend-builder
WORKDIR /app

# Build arguments for Telemetry
ARG VERSION="0.1.0"
ARG CLOUD_METRICS_ENDPOINT=""
ARG CLOUD_METRICS_AUTH_TOKEN=""

COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -ldflags \
    "-X github.com/smallbiznis/railzway/internal/config.DefaultAppVersion=${VERSION} \
     -X github.com/smallbiznis/railzway/internal/config.DefaultCloudMetricsEndpoint=${CLOUD_METRICS_ENDPOINT} \
     -X github.com/smallbiznis/railzway/internal/config.DefaultCloudMetricsAuthToken=${CLOUD_METRICS_AUTH_TOKEN}" \
    -o /app/railzway ./apps/railzway

# =========================
# RUNTIME
# =========================
FROM alpine:latest
WORKDIR /app

# Copy binary
COPY --from=backend-builder /app/railzway .

# Copy UI assets (Admin Dashboard)
COPY --from=frontend-builder /app/dist ./public

# Config
ENV PORT=8080
ENV STATIC_DIR=./public
ENV GIN_MODE=release

EXPOSE 8080
CMD ["./railzway"]
