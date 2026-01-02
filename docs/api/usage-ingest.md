# Usage Ingest API (Current Behavior)

## Overview

The Usage Ingest API records usage events for billing and metering. It writes events into a shared, monolithic service that also serves other product APIs.

## Current architecture

The ingest API currently runs in a single, shared deployment:

- Single instance
- Shared CPU and memory with other APIs (auth, admin, subscription, pricing)
- Shared database (no dedicated ingest datastore)
- Synchronous write path for usage events

This means ingest throughput and latency are directly affected by concurrent activity in the rest of the system. Limits are intentionally conservative to protect shared resources.

## Rate limiting behavior

Rate limiting exists to keep system behavior predictable and to prevent a single workload from starving others. It is expected behavior, not an error condition.

Rate limiting is applied at three layers:

- Organization-level rate limits
- Endpoint-level rate limits
- Per customer + meter concurrency limits

## Idempotency

Idempotency keys are required to make retries safe and deterministic. A duplicate request with the same idempotency key may return the same record instead of creating a new one. This protects against double-charging when clients retry after network errors or 429 responses.

## Concurrency guarantees

Usage ingestion for the same customer and meter is serialized. Only one concurrent ingest is allowed per `(customer_id, meter_code)` to preserve correctness and ordering.

## Performance characteristics (best effort)

Performance is best effort and depends on shared system load. Throughput and latency are affected by:

- Concurrent activity from other APIs
- Database latency and write contention
- The mix of other workloads sharing the same instance

No absolute throughput or latency numbers are guaranteed in the current architecture.

## What this API does not guarantee

- Hard throughput guarantees
- Isolation from other APIs
- Real-time aggregation guarantees

## Intended usage

Recommended client behavior:

- Batch usage events when possible
- Retry on 429 with exponential backoff and jitter
- Avoid burst traffic when possible; spread ingestion over time

## Future notes

This API may evolve into a dedicated service in the future. No timeline or performance commitments are implied.
