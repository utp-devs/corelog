-- Issue links gain the full kind set and a no-self-link rule; votes get their own table with a counter that the
-- application keeps in step inside the same statement.

-- Self links cannot be valid under the new rule; they were never produced by the application.
DELETE FROM issue_links WHERE source_issue_id = target_issue_id;

ALTER TABLE issue_links DROP CONSTRAINT IF EXISTS issue_links_kind_check;
ALTER TABLE issue_links
    ADD CONSTRAINT issue_links_kind_check CHECK (kind IN ('blocks', 'relates', 'duplicates', 'clones', 'causes', 'implements')),
    ADD CONSTRAINT issue_links_no_self_check CHECK (source_issue_id <> target_issue_id);

CREATE TABLE IF NOT EXISTS issue_votes (
    issue_id   UUID        NOT NULL REFERENCES issues (id) ON DELETE CASCADE,
    user_id    UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (issue_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_issue_votes_user ON issue_votes (user_id);

-- No votes existed before this migration, so every counter starts from the rows that exist now (zero).
UPDATE issues SET vote_count = (SELECT COUNT(*) FROM issue_votes v WHERE v.issue_id = issues.id)::int;
