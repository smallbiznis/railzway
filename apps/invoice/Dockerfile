# Build Stage 1: Frontend (Vite/React)
FROM node:20-alpine AS frontend-builder
WORKDIR /app
COPY apps/invoice/package.json apps/invoice/pnpm-lock.yaml ./
RUN npm install -g pnpm
RUN pnpm install --frozen-lockfile
COPY apps/invoice/ ./
RUN pnpm run build

# Build Stage 2: Backend (Go)
FROM golang:1.25-alpine AS backend-builder
WORKDIR /app

# Build arguments
ARG VERSION="1.0.0"
ARG CLOUD_METRICS_ENDPOINT=""
ARG CLOUD_METRICS_AUTH_TOKEN=""

COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -ldflags \
    "-X github.com/smallbiznis/railzway/internal/config.DefaultAppVersion=${VERSION} \
     -X github.com/smallbiznis/railzway/internal/config.DefaultCloudMetricsEndpoint=${CLOUD_METRICS_ENDPOINT} \
     -X github.com/smallbiznis/railzway/internal/config.DefaultCloudMetricsAuthToken=${CLOUD_METRICS_AUTH_TOKEN}" \
    -o invoice-service ./apps/invoice

# Final Stage: Run
FROM alpine:latest
WORKDIR /app

# Copy binary from backend builder
COPY --from=backend-builder /app/invoice-service .

# Copy static assets from frontend builder
COPY --from=frontend-builder /app/dist ./public

# Configuration
ENV PORT=8080
ENV STATIC_DIR=./public
ENV GIN_MODE=release

EXPOSE 8080

CMD ["./invoice-service"]
