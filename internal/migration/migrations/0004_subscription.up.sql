CREATE TABLE IF NOT EXISTS subscriptions (
    id BIGINT PRIMARY KEY,
    org_id BIGINT NOT NULL,
    customer_id BIGINT NOT NULL,
    status TEXT NOT NULL,
    collection_mode TEXT NOT NULL,
    start_at TIMESTAMPTZ NOT NULL,
    end_at TIMESTAMPTZ,
    trial_starts_at TIMESTAMPTZ,
    trial_ends_at TIMESTAMPTZ,
    cancel_at TIMESTAMPTZ,
    cancel_at_period_end BOOLEAN NOT NULL DEFAULT FALSE,
    canceled_at TIMESTAMPTZ,
    billing_anchor_day SMALLINT,
    billing_cycle_type TEXT NOT NULL,
    default_payment_term_days INTEGER,
    default_currency TEXT,
    default_tax_behavior TEXT,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_subscriptions_org_id ON subscriptions(org_id);
CREATE INDEX IF NOT EXISTS idx_subscriptions_customer_id ON subscriptions(customer_id);

CREATE TABLE IF NOT EXISTS subscription_items (
    id BIGINT PRIMARY KEY,
    org_id BIGINT NOT NULL,
    subscription_id BIGINT NOT NULL,
    price_id BIGINT NOT NULL,
    price_code TEXT,
    meter_id BIGINT,
    meter_code TEXT,
    quantity SMALLINT,
    billing_mode TEXT NOT NULL,
    usage_behavior TEXT,
    billing_threshold NUMERIC,
    proration_behavior TEXT,
    next_period_start TIMESTAMPTZ,
    next_period_end TIMESTAMPTZ,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_subscription_items_org_id ON subscription_items(org_id);
CREATE INDEX IF NOT EXISTS idx_subscription_items_subscription_id ON subscription_items(subscription_id);
CREATE INDEX IF NOT EXISTS idx_subscription_items_price_id ON subscription_items(price_id);
CREATE INDEX IF NOT EXISTS idx_subscription_items_meter_id ON subscription_items(meter_id);
CREATE INDEX IF NOT EXISTS idx_subscription_items_sub_meter ON subscription_items(subscription_id, meter_id)
