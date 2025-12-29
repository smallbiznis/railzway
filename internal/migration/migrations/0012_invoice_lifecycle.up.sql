ALTER TABLE invoices
    RENAME COLUMN total_amount TO subtotal_amount;

ALTER TABLE invoices
    ADD COLUMN IF NOT EXISTS invoice_number BIGINT,
    ADD COLUMN IF NOT EXISTS period_start TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS period_end TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS finalized_at TIMESTAMPTZ;

CREATE UNIQUE INDEX IF NOT EXISTS ux_invoice_number_org
    ON invoices(org_id, invoice_number);

ALTER TABLE invoice_items
    RENAME COLUMN rating_result_item_id TO rating_result_id;

ALTER TABLE invoice_items
    RENAME COLUMN unit_amount TO unit_price;

ALTER TABLE invoice_items
    ALTER COLUMN quantity TYPE DOUBLE PRECISION USING quantity::double precision;
