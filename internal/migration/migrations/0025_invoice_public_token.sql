
CREATE TABLE IF NOT EXISTS invoice_public_tokens (
  id BIGINT PRIMARY KEY,
  org_id BIGINT NOT NULL,
  invoice_id BIGINT NOT NULL,

  token_hash TEXT NOT NULL,

  expires_at TIMESTAMPTZ,
  revoked_at TIMESTAMPTZ,

  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Token scoped per org
CREATE UNIQUE INDEX IF NOT EXISTS ux_invoice_public_tokens_org_token
ON invoice_public_tokens(org_id, token_hash);

-- Only one active public token per invoice
CREATE UNIQUE INDEX IF NOT EXISTS ux_invoice_public_tokens_invoice_active
ON invoice_public_tokens(invoice_id)
WHERE revoked_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_invoice_public_tokens_lookup
ON invoice_public_tokens(org_id, token_hash)
WHERE revoked_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_invoice_public_tokens_org_id
ON invoice_public_tokens(org_id);
