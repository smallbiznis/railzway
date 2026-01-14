# Observability (Railzway )

Railzway ships structured logging, OpenTelemetry tracing, and low-cardinality metrics with trace/log correlation.

## Environment variables

- `LOG_LEVEL=debug|info|warn|error`
- `LOG_FORMAT=json|console`
- `OTEL_ENABLED=true|false`
- `OTEL_EXPORTER_OTLP_ENDPOINT=host:port`
- `OTEL_EXPORTER_OTLP_PROTOCOL=grpc|http`
- `OTEL_SAMPLING_RATIO=0.1`
- `SERVICE_VERSION=...`
- `DEPLOYMENT_ENV=dev|staging|prod`

## Run example

```bash
export LOG_LEVEL=info
export LOG_FORMAT=json
export OTEL_ENABLED=true
export OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317
export OTEL_EXPORTER_OTLP_PROTOCOL=grpc
export OTEL_SAMPLING_RATIO=0.1
export SERVICE_VERSION=dev
export DEPLOYMENT_ENV=dev

go run ./apps/railzway
```

## Correlation fields

Every request log line includes:

- `request_id`, `trace_id`, `span_id`
- `org_id` (when known), `actor_type`, `actor_id`

## Sensitive data safety

- Authorization and Cookie headers are masked.
- JSON fields matching `password`, `secret`, `token`, `api_key`, `webhook_secret`, `authorization` are masked.
- Raw request bodies are not logged in production.

## Metrics

Low-cardinality labels only:

- HTTP: `endpoint`, `status_code`
- Domain counters: `meter_code`, `provider`, `event_type`, `source_type`

## HTTP client tracing

Use the helper to instrument outbound HTTP clients:

```go
client := tracing.WrapHTTPClient(http.DefaultClient)
```

## Notes

- `/api/usage` logs are minimal; validation errors log at debug.
- No high-cardinality identifiers are emitted in metrics labels.
