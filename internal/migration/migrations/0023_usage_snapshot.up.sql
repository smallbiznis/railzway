ALTER TABLE usage_events
    ADD COLUMN IF NOT EXISTS snapshot_at TIMESTAMPTZ;

ALTER TABLE usage_events
    ALTER COLUMN subscription_item_id DROP NOT NULL;

CREATE INDEX IF NOT EXISTS idx_usage_events_status_recorded_at
    ON usage_events (status, recorded_at);
