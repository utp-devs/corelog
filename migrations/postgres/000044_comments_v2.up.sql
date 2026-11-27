-- Comments v2: threads (single level), internal visibility, edit history, reactions and notification marks.

ALTER TABLE comments
    ADD COLUMN IF NOT EXISTS parent_id     UUID REFERENCES comments (id) ON DELETE CASCADE,
    ADD COLUMN IF NOT EXISTS visibility    VARCHAR(16) NOT NULL DEFAULT 'public',
    ADD COLUMN IF NOT EXISTS internal_role VARCHAR(16),
    ADD COLUMN IF NOT EXISTS edited_at     TIMESTAMPTZ;

ALTER TABLE comments
    ADD CONSTRAINT comments_visibility_check CHECK (visibility IN ('public', 'internal')),
    ADD CONSTRAINT comments_internal_role_check CHECK (
        (visibility = 'public' AND internal_role IS NULL)
        OR (visibility = 'internal' AND internal_role IS NOT NULL AND internal_role IN ('contributor', 'lead'))
    ),
    ADD CONSTRAINT comments_no_self_parent_check CHECK (parent_id IS NULL OR parent_id <> id);

-- Comments edited before this migration: the update time is the best known edit time.
UPDATE comments SET edited_at = updated_at WHERE updated_at > created_at AND edited_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_comments_issue_parent ON comments (issue_id, parent_id, created_at);
CREATE INDEX IF NOT EXISTS idx_comments_parent_id ON comments (parent_id);

ALTER TABLE comment_mentions
    ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now();

CREATE TABLE IF NOT EXISTS comment_revisions (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    comment_id UUID NOT NULL REFERENCES comments (id) ON DELETE CASCADE,
    body       TEXT NOT NULL,
    edited_by  UUID NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    edited_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_comment_revisions_comment ON comment_revisions (comment_id, edited_at);
CREATE INDEX IF NOT EXISTS idx_comment_revisions_editor ON comment_revisions (edited_by);

CREATE TABLE IF NOT EXISTS comment_reactions (
    comment_id UUID NOT NULL REFERENCES comments (id) ON DELETE CASCADE,
    user_id    UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    emoji      TEXT NOT NULL CHECK (emoji IN ('thumbs_up', 'heart', 'tada', 'eyes', 'rocket', 'check')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (comment_id, user_id, emoji)
);

CREATE INDEX IF NOT EXISTS idx_comment_reactions_user ON comment_reactions (user_id);

-- One in-app notice per comment, user and kind: the notification consumer uses it to deliver at most once.
CREATE TABLE IF NOT EXISTS comment_notices (
    comment_id UUID NOT NULL REFERENCES comments (id) ON DELETE CASCADE,
    user_id    UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    kind       VARCHAR(16) NOT NULL CHECK (kind IN ('mention', 'reply')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (comment_id, user_id, kind)
);

CREATE INDEX IF NOT EXISTS idx_comment_notices_user ON comment_notices (user_id);
