CREATE TABLE IF NOT EXISTS billing_cycles (
    id BIGINT PRIMARY KEY,
    org_id BIGINT NOT NULL,
    subscription_id BIGINT NOT NULL,
    period_start TIMESTAMPTZ NOT NULL,
    period_end TIMESTAMPTZ NOT NULL,
    status TEXT NOT NULL DEFAULT 'OPEN',
    rated_at TIMESTAMPTZ,
    invoiced_at TIMESTAMPTZ,
    closed_at TIMESTAMPTZ,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_billing_cycle_period ON billing_cycles(subscription_id, period_start, period_end);
CREATE INDEX IF NOT EXISTS idx_billing_cycles_org_id ON billing_cycles(org_id);
CREATE INDEX IF NOT EXISTS idx_billing_cycles_subscription_id ON billing_cycles(subscription_id);
