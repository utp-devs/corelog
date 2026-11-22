CREATE TABLE IF NOT EXISTS issue_links (
    id               UUID PRIMARY KEY,
    source_issue_id  UUID NOT NULL REFERENCES issues (id) ON DELETE CASCADE,
    target_issue_id  UUID NOT NULL REFERENCES issues (id) ON DELETE CASCADE,
    kind             VARCHAR(20) NOT NULL CHECK (kind IN ('blocks', 'relates', 'duplicates')),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by       UUID NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    UNIQUE (source_issue_id, target_issue_id, kind)
);

CREATE INDEX IF NOT EXISTS idx_issue_links_source ON issue_links (source_issue_id);
CREATE INDEX IF NOT EXISTS idx_issue_links_target ON issue_links (target_issue_id);
CREATE INDEX IF NOT EXISTS idx_issue_links_kind ON issue_links (kind);
