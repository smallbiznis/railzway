CREATE TABLE IF NOT EXISTS ledger_accounts (
  id BIGINT PRIMARY KEY,
  org_id BIGINT NOT NULL,
  code TEXT NOT NULL,
  name TEXT NOT NULL,
  type TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_ledger_accounts_org_code ON ledger_accounts(org_id, code);
CREATE INDEX IF NOT EXISTS idx_ledger_accounts_org_id ON ledger_accounts(org_id);

CREATE TABLE IF NOT EXISTS ledger_entries (
  id BIGINT PRIMARY KEY,
  org_id BIGINT NOT NULL,
  source_type TEXT NOT NULL,
  source_id BIGINT NOT NULL,
  currency TEXT NOT NULL,
  occurred_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_ledger_entries_source ON ledger_entries(org_id, source_type, source_id);
CREATE INDEX IF NOT EXISTS idx_ledger_entries_org_id ON ledger_entries(org_id);

CREATE TABLE IF NOT EXISTS ledger_entry_lines (
  id BIGINT PRIMARY KEY,
  ledger_entry_id BIGINT NOT NULL REFERENCES ledger_entries(id),
  account_id BIGINT NOT NULL REFERENCES ledger_accounts(id),
  direction TEXT NOT NULL CHECK (direction IN ('debit', 'credit')),
  currency TEXT NOT NULL DEFAULT 'USD',
  amount BIGINT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_ledger_entry_lines_entry_id ON ledger_entry_lines(ledger_entry_id);
CREATE INDEX IF NOT EXISTS idx_ledger_entry_lines_account_id ON ledger_entry_lines(account_id);
