
CREATE TABLE IF NOT EXISTS invoice_public_tokens (
  id BIGINT PRIMARY KEY,
  org_id BIGINT NOT NULL,
  invoice_id BIGINT NOT NULL,

  token TEXT NOT NULL,

  expires_at TIMESTAMPTZ NOT NULL,
  revoked_at TIMESTAMPTZ,

  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_invoice_public_tokens_token
ON invoice_public_tokens(token);

CREATE UNIQUE INDEX IF NOT EXISTS ux_invoice_public_tokens_invoice
ON invoice_public_tokens(invoice_id)
WHERE revoked_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_invoice_public_tokens_org_id
ON invoice_public_tokens(org_id);

CREATE INDEX IF NOT EXISTS idx_invoice_public_tokens_valid
ON invoice_public_tokens(token, expires_at)
WHERE revoked_at IS NULL;
