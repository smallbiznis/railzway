CREATE TABLE IF NOT EXISTS casbin_rule (
    id BIGSERIAL PRIMARY KEY,
    ptype VARCHAR(100) NOT NULL,
    v0 VARCHAR(100),
    v1 VARCHAR(100),
    v2 VARCHAR(100),
    v3 VARCHAR(100),
    v4 VARCHAR(100),
    v5 VARCHAR(100)
);

CREATE INDEX IF NOT EXISTS idx_casbin_rule_ptype ON casbin_rule (ptype);
CREATE INDEX IF NOT EXISTS idx_casbin_rule_v0 ON casbin_rule (v0);
CREATE INDEX IF NOT EXISTS idx_casbin_rule_v1 ON casbin_rule (v1);
CREATE INDEX IF NOT EXISTS idx_casbin_rule_v2 ON casbin_rule (v2);
