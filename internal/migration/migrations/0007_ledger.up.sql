CREATE TABLE IF NOT EXISTS ledger_accounts (
    id BIGINT PRIMARY KEY,
    org_id BIGINT NOT NULL,
    name TEXT NOT NULL,
    currency TEXT NOT NULL,
    account_type TEXT NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_ledger_accounts_org_id ON ledger_accounts(org_id);

CREATE TABLE IF NOT EXISTS ledger_entries (
    id BIGINT PRIMARY KEY,
    org_id BIGINT NOT NULL,
    account_id BIGINT NOT NULL,
    entry_type TEXT NOT NULL,
    amount BIGINT NOT NULL,
    currency TEXT NOT NULL,
    description TEXT,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_ledger_entries_org_id ON ledger_entries(org_id);
CREATE INDEX IF NOT EXISTS idx_ledger_entries_account_id ON ledger_entries(account_id);
