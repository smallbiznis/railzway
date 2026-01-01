CREATE TABLE IF NOT EXISTS payment_provider_catalog (
  provider TEXT PRIMARY KEY,
  display_name TEXT NOT NULL,
  description TEXT,
  supports_webhook BOOLEAN NOT NULL DEFAULT TRUE,
  supports_refund BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS payment_provider_configs (
  id BIGINT PRIMARY KEY,
  org_id BIGINT NOT NULL,
  provider TEXT NOT NULL,
  config JSONB NOT NULL,
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_payment_provider_configs_org_provider ON payment_provider_configs(org_id, provider);
CREATE INDEX IF NOT EXISTS idx_payment_provider_configs_org_id ON payment_provider_configs(org_id);

INSERT INTO payment_provider_catalog (provider, display_name, description, supports_webhook, supports_refund)
VALUES
  ('stripe', 'Stripe', 'Card and wallet payments with global reach.', TRUE, TRUE),
  ('midtrans', 'Midtrans', 'SEA payments with bank transfer and wallets.', TRUE, FALSE),
  ('xendit', 'Xendit', 'Indonesia payments and disbursements.', TRUE, FALSE),
  ('manual', 'Manual / Offline', 'Offline settlement via bank transfer or cash.', FALSE, FALSE)
ON CONFLICT (provider) DO NOTHING;
