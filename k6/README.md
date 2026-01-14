# Railzway k6 Test Suite

This suite validates Railzway architecture invariants: hot-path ingest, idempotency, async boundaries, and admin read scaling.

## Prerequisites
- k6 installed locally
- Railzway API reachable
- Valid API key and seeded billing data (customer, meter, subscription)

## Environment
Global config (required by all scripts):
- `BASE_URL` (default: `http://localhost:8080`)
- `API_KEY` (Bearer token for `/api` routes)
- `ORG_ID` (org header for `/admin` routes; optional if session has an active org)

Usage ingest tests (required):
- `CUSTOMER_ID`
- `METER_CODE`

Admin tests (required):
- `ADMIN_SESSION` (value of the `_sid` cookie)

Payment webhook test (required):
- `PAYMENT_PROVIDER` (e.g. `stripe`)
- `WEBHOOK_PAYLOAD` (raw JSON string; must include a stable provider event id)
- `WEBHOOK_HEADERS` (JSON map for provider signature headers)

Auth perf test (optional):
- `AUTH_COMPARE=true` to run the unauthorized baseline request

Tuning (optional):
- `INGEST_VUS`, `INGEST_DURATION`
- `IDEMPOTENCY_ITERATIONS`
- `BURST_VUS`
- `DASHBOARD_VUS`, `DASHBOARD_DURATION`
- `PAGE_SIZE`, `MAX_PAGES`
- `AUTH_VUS`, `AUTH_DURATION`
- `WEBHOOK_VUS`

## Run commands
Usage ingest load (hot path):
```
BASE_URL=http://localhost:8080 \
API_KEY=railzway_test_key \
CUSTOMER_ID=1234567890123456 \
METER_CODE=api_calls \
k6 run k6/usage/ingest_load.js
```

Usage ingest idempotency:
```
BASE_URL=http://localhost:8080 \
API_KEY=railzway_test_key \
CUSTOMER_ID=1234567890123456 \
METER_CODE=api_calls \
k6 run k6/usage/ingest_idempotency.js
```

Usage ingest burst:
```
BASE_URL=http://localhost:8080 \
API_KEY=railzway_test_key \
CUSTOMER_ID=1234567890123456 \
METER_CODE=api_calls \
BURST_VUS=40 \
k6 run k6/usage/ingest_burst.js
```

API key auth performance:
```
BASE_URL=http://localhost:8080 \
API_KEY=railzway_test_key \
AUTH_COMPARE=true \
k6 run k6/usage/api_key_auth_perf.js
```

Payment webhook idempotency:
```
BASE_URL=http://localhost:8080 \
PAYMENT_PROVIDER=stripe \
WEBHOOK_PAYLOAD='{"id":"evt_123","type":"charge.succeeded",...}' \
WEBHOOK_HEADERS='{"Stripe-Signature":"t=...,v1=..."}' \
k6 run k6/payments/webhook_idempotency.js
```

Admin billing dashboard read:
```
BASE_URL=http://localhost:8080 \
ADMIN_SESSION=your_sid_cookie_value \
ORG_ID=1234567890123456 \
k6 run k6/admin/billing_dashboard.js
```

Admin pagination stability:
```
BASE_URL=http://localhost:8080 \
ADMIN_SESSION=your_sid_cookie_value \
ORG_ID=1234567890123456 \
PAGE_SIZE=50 \
MAX_PAGES=10 \
k6 run k6/admin/pagination.js
```

## Notes
- `/api` routes must not include `X-Org-Id` or `org_id`; org is derived from API keys.
- Admin routes require the `_sid` session cookie; reuse a valid browser session.
- Webhook payloads must match the configured provider adapter and include a stable provider event id to validate idempotency.
