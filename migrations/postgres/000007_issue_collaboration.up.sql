CREATE TABLE IF NOT EXISTS comments (
    id         UUID PRIMARY KEY,
    issue_id   UUID NOT NULL REFERENCES issues (id) ON DELETE CASCADE,
    author_id  UUID NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    body       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS comment_mentions (
    comment_id UUID NOT NULL REFERENCES comments (id) ON DELETE CASCADE,
    user_id    UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    PRIMARY KEY (comment_id, user_id)
);

CREATE TABLE IF NOT EXISTS activities (
    id         UUID PRIMARY KEY,
    issue_id   UUID NOT NULL REFERENCES issues (id) ON DELETE CASCADE,
    actor_id   UUID NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    action     VARCHAR(40) NOT NULL,
    field_name VARCHAR(40),
    old_value  TEXT,
    new_value  TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_comments_issue_id ON comments (issue_id);
CREATE INDEX IF NOT EXISTS idx_comments_author_id ON comments (author_id);
CREATE INDEX IF NOT EXISTS idx_comment_mentions_user_id ON comment_mentions (user_id);
CREATE INDEX IF NOT EXISTS idx_activities_issue_id ON activities (issue_id);
CREATE INDEX IF NOT EXISTS idx_activities_actor_id ON activities (actor_id);
