CREATE TABLE IF NOT EXISTS products (
    id BIGINT PRIMARY KEY,
    org_id BIGINT NOT NULL,
    code TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_products_org_code ON products(org_id, code);
CREATE INDEX IF NOT EXISTS idx_products_org_id ON products(org_id);

CREATE TABLE IF NOT EXISTS prices (
    id BIGINT PRIMARY KEY,
    org_id BIGINT NOT NULL,
    product_id BIGINT NOT NULL,
    code TEXT NOT NULL,
    name TEXT,
    description TEXT,
    pricing_model TEXT NOT NULL DEFAULT '0',
    billing_mode TEXT NOT NULL DEFAULT '0',
    billing_interval TEXT NOT NULL DEFAULT '0',
    billing_interval_count INTEGER NOT NULL DEFAULT 1,
    aggregate_usage TEXT,
    billing_unit TEXT,
    billing_threshold NUMERIC,
    tax_behavior TEXT NOT NULL DEFAULT '0',
    tax_code TEXT,
    version INTEGER NOT NULL DEFAULT 1,
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    retired_at TIMESTAMPTZ,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_prices_org_id ON prices(org_id);
CREATE INDEX IF NOT EXISTS idx_prices_product_id ON prices(product_id);

CREATE TABLE IF NOT EXISTS price_amounts (
    id BIGINT PRIMARY KEY,
    org_id BIGINT NOT NULL,
    price_id BIGINT NOT NULL,
    meter_id BIGINT,
    currency TEXT NOT NULL,
    unit_amount_cents BIGINT NOT NULL,
    minimum_amount_cents BIGINT,
    maximum_amount_cents BIGINT,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_price_amounts_org_id ON price_amounts(org_id);
CREATE INDEX IF NOT EXISTS idx_price_amounts_price_id ON price_amounts(price_id);
CREATE INDEX IF NOT EXISTS idx_price_amounts_meter_id ON price_amounts(meter_id);

CREATE TABLE IF NOT EXISTS price_tiers (
    id BIGINT PRIMARY KEY,
    org_id BIGINT NOT NULL,
    price_id BIGINT NOT NULL,
    tier_mode SMALLINT NOT NULL DEFAULT 0,
    start_quantity NUMERIC NOT NULL,
    end_quantity NUMERIC,
    unit_amount_cents BIGINT,
    flat_amount_cents BIGINT,
    unit TEXT NOT NULL,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_price_tiers_org_id ON price_tiers(org_id);
CREATE INDEX IF NOT EXISTS idx_price_tiers_price_id ON price_tiers(price_id);

CREATE TABLE IF NOT EXISTS customers (
    id BIGINT PRIMARY KEY,
    org_id BIGINT NOT NULL,
    name TEXT NOT NULL,
    email TEXT NOT NULL,
    currency TEXT,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_customers_org_id ON customers(org_id);
