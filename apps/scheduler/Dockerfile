# Build Stage: Backend (Go)
FROM golang:1.25-alpine AS builder
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
    -o scheduler ./apps/scheduler

# Final Stage: Run
FROM alpine:latest
WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/scheduler .

# Configuration
ENV ENABLED_JOBS=""
ENV PORT=8080

EXPOSE 8080

CMD ["./scheduler"]
