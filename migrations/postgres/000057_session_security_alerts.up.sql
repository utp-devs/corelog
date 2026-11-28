-- Structured detail of a notification (actions and facts of a security alert) and the devices already seen
-- in a login, so a known browser does not raise an alert every time.
ALTER TABLE notifications ADD COLUMN IF NOT EXISTS data JSONB NOT NULL DEFAULT '{}'::jsonb;

CREATE TABLE IF NOT EXISTS login_fingerprints (
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    fingerprint CHAR(64) NOT NULL,
    first_seen_at TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (user_id, fingerprint)
);

CREATE INDEX IF NOT EXISTS idx_login_fingerprints_last_seen ON login_fingerprints (last_seen_at);
