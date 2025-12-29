ALTER TABLE billing_cycles
    ADD COLUMN IF NOT EXISTS closing_started_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS rating_completed_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS invoice_finalized_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS last_error TEXT,
    ADD COLUMN IF NOT EXISTS last_error_at TIMESTAMPTZ;

UPDATE billing_cycles
SET rating_completed_at = rated_at
WHERE rating_completed_at IS NULL
  AND rated_at IS NOT NULL;
