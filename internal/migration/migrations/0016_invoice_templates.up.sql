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
