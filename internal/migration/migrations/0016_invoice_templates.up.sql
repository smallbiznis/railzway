CREATE TABLE IF NOT EXISTS invoice_templates (
    id BIGINT PRIMARY KEY,
    org_id BIGINT NOT NULL,
    name TEXT NOT NULL,
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    locale TEXT NOT NULL DEFAULT 'en',
    currency TEXT NOT NULL,
    header JSONB,
    footer JSONB,
    style JSONB,
    show_ship_to BOOLEAN NOT NULL DEFAULT true,
    show_payment_details BOOLEAN NOT NULL DEFAULT true,
    show_tax_details BOOLEAN NOT NULL DEFAULT false,
    version INT NOT NULL DEFAULT 1,
    is_locked BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_invoice_templates_org_id ON invoice_templates(org_id);

CREATE UNIQUE INDEX IF NOT EXISTS ux_invoice_templates_default
    ON invoice_templates(org_id)
    WHERE is_default = TRUE;

ALTER TABLE invoices
    ADD COLUMN IF NOT EXISTS invoice_template_id BIGINT,
    ADD COLUMN IF NOT EXISTS rendered_html TEXT,
    ADD COLUMN IF NOT EXISTS rendered_pdf_url TEXT;

CREATE INDEX IF NOT EXISTS idx_invoices_invoice_template_id ON invoices(invoice_template_id);
