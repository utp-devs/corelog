-- Two-factor authentication (TOTP). One row per user: the secret is stored encrypted (AES-GCM) and is
-- usable for login only after enabled_at is set. last_used_step rejects replay of a time step.
CREATE TABLE IF NOT EXISTS user_two_factor (
    user_id          UUID PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    secret_encrypted BYTEA NOT NULL CHECK (octet_length(secret_encrypted) > 0),
    enabled_at       TIMESTAMPTZ NULL,
    last_used_step   BIGINT NULL CHECK (last_used_step IS NULL OR last_used_step >= 0),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- One-time recovery codes. Only SHA-256 digests are stored.
CREATE TABLE IF NOT EXISTS user_recovery_codes (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    code_hash  BYTEA NOT NULL CHECK (octet_length(code_hash) = 32),
    used_at    TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_user_recovery_codes_user_id ON user_recovery_codes (user_id);

-- Login challenges issued after a correct password for a user with two-factor enabled. The failed attempt
-- counter lives here so that the limit holds across requests and processes.
CREATE TABLE IF NOT EXISTS user_two_factor_challenges (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    expires_at      TIMESTAMPTZ NOT NULL,
    failed_attempts INT NOT NULL DEFAULT 0 CHECK (failed_attempts >= 0),
    consumed_at     TIMESTAMPTZ NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_user_two_factor_challenges_user_id ON user_two_factor_challenges (user_id);
