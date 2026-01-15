CREATE TABLE IF NOT EXISTS users (
    id BIGINT PRIMARY KEY,
    external_id TEXT UNIQUE,
    provider TEXT,
    display_name TEXT,
    email TEXT UNIQUE,
    password_hash TEXT,
    is_default BOOLEAN DEFAULT FALSE,
    last_password_changed TIMESTAMPTZ,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS sessions (
    id BIGINT PRIMARY KEY,
    user_id BIGINT NOT NULL,
    session_token_hash TEXT NOT NULL UNIQUE,
    user_agent TEXT,
    ip_address TEXT,
    active_org_id BIGINT,
    org_ids JSONB,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_active_org_id ON sessions(active_org_id);
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);

CREATE TABLE IF NOT EXISTS organizations (
    id BIGINT PRIMARY KEY,
    name TEXT NOT NULL,
    slug TEXT NOT NULL,
    is_default BOOLEAN DEFAULT FALSE,
    country_code TEXT,
    timezone_name TEXT,
    support_email TEXT,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_organizations_slug ON organizations(slug);

CREATE TABLE IF NOT EXISTS organization_members (
    id BIGINT PRIMARY KEY,
    org_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    role TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_org_user ON organization_members(org_id, user_id);
CREATE INDEX IF NOT EXISTS idx_organization_members_org_id ON organization_members(org_id);
CREATE INDEX IF NOT EXISTS idx_organization_members_user_id ON organization_members(user_id);

CREATE TABLE IF NOT EXISTS organization_invites (
    id BIGINT PRIMARY KEY,
    org_id BIGINT NOT NULL,
    email TEXT NOT NULL,
    role TEXT NOT NULL,
    status TEXT NOT NULL,
    invited_by BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_organization_invites_org_id ON organization_invites(org_id);
CREATE INDEX IF NOT EXISTS idx_organization_invites_invited_by ON organization_invites(invited_by);

CREATE TABLE IF NOT EXISTS organization_billing_preferences (
    org_id BIGINT PRIMARY KEY,
    currency TEXT NOT NULL,
    timezone TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
