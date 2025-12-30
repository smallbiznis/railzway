CREATE TABLE IF NOT EXISTS meters (
    id BIGINT PRIMARY KEY,
    org_id BIGINT NOT NULL,
    code TEXT NOT NULL,
    name TEXT NOT NULL,
    aggregation TEXT NOT NULL,
    unit TEXT NOT NULL,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_meters_org_code ON meters(org_id, code);
CREATE INDEX IF NOT EXISTS idx_meters_org_id ON meters(org_id);

CREATE TABLE IF NOT EXISTS usage_events (
    id BIGINT PRIMARY KEY,
    org_id BIGINT NOT NULL,
    customer_id BIGINT NOT NULL,
    subscription_id BIGINT NOT NULL,
    subscription_item_id BIGINT NOT NULL,
    meter_id BIGINT NOT NULL,
    meter_code TEXT NOT NULL,
    value DOUBLE PRECISION NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL,
    idempotency_key TEXT,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS uidx_usage_idempotency_key ON usage_events (org_id, idempotency_key);
CREATE INDEX IF NOT EXISTS idx_usage_events_org_id ON usage_events(org_id);
CREATE INDEX IF NOT EXISTS idx_usage_events_customer_id ON usage_events(customer_id);
CREATE INDEX IF NOT EXISTS idx_usage_events_subscription_id ON usage_events(subscription_id);
CREATE INDEX IF NOT EXISTS idx_usage_events_meter_id ON usage_events(meter_id);

CREATE TABLE IF NOT EXISTS billing_events (
    id BIGINT PRIMARY KEY,
    org_id BIGINT NOT NULL,
    event_type TEXT NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}',
    dedupe_key TEXT,
    published BOOLEAN NOT NULL DEFAULT FALSE,
    published_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_billing_event_dedupe ON billing_events(org_id, dedupe_key);
CREATE INDEX IF NOT EXISTS idx_billing_events_org_id ON billing_events(org_id);
