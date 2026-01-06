CREATE TABLE IF NOT EXISTS rating_results (
    id BIGINT PRIMARY KEY,
    org_id BIGINT NOT NULL,
    subscription_id BIGINT NOT NULL,
    billing_cycle_id BIGINT NOT NULL,
    meter_id BIGINT,
    price_id BIGINT NOT NULL,
    quantity DOUBLE PRECISION NOT NULL,
    unit_price BIGINT NOT NULL,
    amount BIGINT NOT NULL,
    currency TEXT NOT NULL,
    period_start TIMESTAMPTZ NOT NULL,
    period_end TIMESTAMPTZ NOT NULL,
    source TEXT NOT NULL,
    checksum TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_rating_results_checksum ON rating_results(checksum);
CREATE INDEX IF NOT EXISTS idx_rating_results_subscription_id ON rating_results(subscription_id);
CREATE INDEX IF NOT EXISTS idx_rating_results_billing_cycle_id ON rating_results(billing_cycle_id);
CREATE INDEX IF NOT EXISTS idx_rating_results_meter_id ON rating_results(meter_id);
