-- Personal API tokens. Only the SHA-256 hash is stored; prefix is the first characters for display.
CREATE TABLE IF NOT EXISTS api_tokens (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name         VARCHAR(80) NOT NULL,
    token_hash   CHAR(64) NOT NULL UNIQUE,
    prefix       VARCHAR(16) NOT NULL,
    scopes       TEXT[] NOT NULL CHECK (cardinality(scopes) > 0 AND scopes <@ ARRAY['read', 'write', 'admin']::TEXT[]),
    expires_at   TIMESTAMPTZ NULL,
    last_used_at TIMESTAMPTZ NULL,
    revoked_at   TIMESTAMPTZ NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_api_tokens_user ON api_tokens (user_id, created_at DESC);

-- Outgoing webhooks. The signing secret is stored encrypted (AES-GCM).
CREATE TABLE IF NOT EXISTS webhooks (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id  UUID NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    project_id       UUID NULL REFERENCES projects (id) ON DELETE CASCADE,
    name             VARCHAR(120) NOT NULL,
    url              TEXT NOT NULL,
    secret_encrypted BYTEA NOT NULL,
    events           TEXT[] NOT NULL DEFAULT '{}',
    filter_query     TEXT NULL,
    enabled          BOOLEAN NOT NULL DEFAULT TRUE,
    created_by       UUID NULL REFERENCES users (id) ON DELETE SET NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_webhooks_org ON webhooks (organization_id, enabled);
CREATE INDEX IF NOT EXISTS idx_webhooks_project ON webhooks (project_id) WHERE project_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_webhooks_created_by ON webhooks (created_by);

CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    webhook_id       UUID NOT NULL REFERENCES webhooks (id) ON DELETE CASCADE,
    event_id         BIGINT NULL,
    event_type       VARCHAR(60) NOT NULL,
    payload          JSONB NOT NULL,
    status           VARCHAR(10) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'success', 'failed', 'dead')),
    attempts         INT NOT NULL DEFAULT 0,
    next_attempt_at  TIMESTAMPTZ NULL,
    response_status  INT NULL,
    response_excerpt TEXT NOT NULL DEFAULT '',
    duration_ms      INT NOT NULL DEFAULT 0,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_webhook ON webhook_deliveries (webhook_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_due ON webhook_deliveries (next_attempt_at) WHERE status IN ('pending', 'failed');
CREATE UNIQUE INDEX IF NOT EXISTS uq_webhook_deliveries_event ON webhook_deliveries (webhook_id, event_id) WHERE event_id IS NOT NULL;
