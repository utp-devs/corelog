CREATE TABLE IF NOT EXISTS watchers (
    issue_id UUID NOT NULL REFERENCES issues (id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (issue_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_watchers_user_id ON watchers (user_id, issue_id);
