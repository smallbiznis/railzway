CREATE TABLE IF NOT EXISTS invoices (
    id BIGINT PRIMARY KEY,
    org_id BIGINT NOT NULL,
    billing_cycle_id BIGINT NOT NULL,
    subscription_id BIGINT NOT NULL,
    customer_id BIGINT NOT NULL,
    invoice_seq INT NOT NULL,
    invoice_number TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'DRAFT',
    total_amount BIGINT NOT NULL DEFAULT 0,
    currency TEXT NOT NULL,
    issued_at TIMESTAMPTZ,
    due_at TIMESTAMPTZ,
    voided_at TIMESTAMPTZ,
    paid_at TIMESTAMPTZ,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_invoice_billing_cycle ON invoices(billing_cycle_id);
CREATE UNIQUE INDEX IF NOT EXISTS ux_invoice_invoice_seq ON invoices(org_id, invoice_seq);
CREATE UNIQUE INDEX IF NOT EXISTS ux_invoice_invoice_number ON invoices(org_id, invoice_number);
CREATE INDEX IF NOT EXISTS idx_invoices_org_id ON invoices(org_id);
CREATE INDEX IF NOT EXISTS idx_invoices_subscription_id ON invoices(subscription_id);
CREATE INDEX IF NOT EXISTS idx_invoices_customer_id ON invoices(customer_id);

CREATE TABLE IF NOT EXISTS invoice_items (
    id BIGINT PRIMARY KEY,
    org_id BIGINT NOT NULL,
    invoice_id BIGINT NOT NULL,
    rating_result_item_id BIGINT,
    subscription_item_id BIGINT,
    line_type TEXT,
    description TEXT,
    quantity BIGINT NOT NULL,
    unit_amount BIGINT NOT NULL,
    amount BIGINT NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_invoice_items_org_id ON invoice_items(org_id);
CREATE INDEX IF NOT EXISTS idx_invoice_items_invoice_id ON invoice_items(invoice_id);
CREATE INDEX IF NOT EXISTS idx_invoice_items_rating_result_item_id ON invoice_items(rating_result_item_id);
CREATE INDEX IF NOT EXISTS idx_invoice_items_subscription_item_id ON invoice_items(subscription_item_id);

CREATE TABLE invoice_sequences (
  org_id          BIGINT PRIMARY KEY,
  next_number     BIGINT NOT NULL DEFAULT 1,
  updated_at      TIMESTAMPTZ NOT NULL
);