CREATE TABLE IF NOT EXISTS payment_disputes (
  id BIGINT PRIMARY KEY,
  org_id BIGINT NOT NULL,
  provider TEXT NOT NULL,
  provider_dispute_id TEXT NOT NULL,
  provider_event_id TEXT NOT NULL,
  customer_id BIGINT NOT NULL,
  amount BIGINT NOT NULL,
  currency TEXT NOT NULL,
  status TEXT NOT NULL,
  reason TEXT,
  received_at TIMESTAMPTZ NOT NULL,
  processed_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_payment_disputes_provider_dispute_id
  ON payment_disputes(provider, provider_dispute_id);
CREATE INDEX IF NOT EXISTS idx_payment_disputes_org_id
  ON payment_disputes(org_id);
CREATE INDEX IF NOT EXISTS idx_payment_disputes_customer_id
  ON payment_disputes(customer_id);
