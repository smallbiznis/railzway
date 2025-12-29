CREATE TABLE IF NOT EXISTS api_keys (
    id BIGINT PRIMARY KEY,
    org_id BIGINT NOT NULL,
    key_id TEXT NOT NULL,
    name TEXT NOT NULL,
    scope TEXT NOT NULL,
    key_hash TEXT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_used_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    rotated_from_key_id TEXT
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_api_keys_org_key_id ON api_keys(org_id, key_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_org_id ON api_keys(org_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_key_id ON api_keys(key_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_active ON api_keys(org_id, is_active);
