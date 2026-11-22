-- Seguridad de cuenta: sesiones revocables y desactivacion de cuentas (borrado logico).
ALTER TABLE users ADD COLUMN IF NOT EXISTS deactivated_at TIMESTAMPTZ;

CREATE TABLE IF NOT EXISTS user_sessions (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    user_agent VARCHAR(300) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_user_sessions_user ON user_sessions (user_id) WHERE revoked_at IS NULL;
