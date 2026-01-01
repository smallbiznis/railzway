CREATE TABLE IF NOT EXISTS payment_events (
  id BIGINT PRIMARY KEY,
  org_id BIGINT NOT NULL,
  provider TEXT NOT NULL,
  provider_event_id TEXT NOT NULL,
  event_type TEXT NOT NULL,
  customer_id BIGINT NOT NULL,
  payload JSONB NOT NULL,
  received_at TIMESTAMPTZ NOT NULL,
  processed_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_payment_events_provider_event_id
  ON payment_events(provider, provider_event_id);
CREATE INDEX IF NOT EXISTS idx_payment_events_org_id
  ON payment_events(org_id);
CREATE INDEX IF NOT EXISTS idx_payment_events_customer_id
  ON payment_events(customer_id);
