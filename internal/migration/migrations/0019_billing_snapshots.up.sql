CREATE TABLE IF NOT EXISTS customer_balances (
    org_id BIGINT NOT NULL,
    customer_id BIGINT NOT NULL,
    currency TEXT NOT NULL,
    balance BIGINT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (org_id, customer_id, currency)
);

CREATE INDEX IF NOT EXISTS idx_customer_balances_org_id ON customer_balances(org_id);

CREATE TABLE IF NOT EXISTS billing_cycle_stats (
    billing_cycle_id BIGINT PRIMARY KEY,
    org_id BIGINT NOT NULL,
    period_start TIMESTAMPTZ NOT NULL,
    status TEXT NOT NULL,
    total_revenue BIGINT NOT NULL,
    invoice_count INT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_billing_cycle_stats_org_id ON billing_cycle_stats(org_id);

CREATE TABLE IF NOT EXISTS billing_snapshot_rebuild_requests (
    id BIGINT PRIMARY KEY,
    org_id BIGINT,
    billing_cycle_id BIGINT,
    status TEXT NOT NULL,
    error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_billing_snapshot_rebuild_status ON billing_snapshot_rebuild_requests(status, created_at);
