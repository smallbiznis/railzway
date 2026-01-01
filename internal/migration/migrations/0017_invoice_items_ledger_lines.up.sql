ALTER TABLE invoice_items
    ADD COLUMN IF NOT EXISTS ledger_entry_line_id BIGINT;

CREATE INDEX IF NOT EXISTS idx_invoice_items_ledger_entry_line_id
    ON invoice_items(ledger_entry_line_id);
