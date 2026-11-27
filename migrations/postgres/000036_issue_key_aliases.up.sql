-- Keys of issues that moved to another project keep resolving: the old project and number point to the issue.
CREATE TABLE IF NOT EXISTS issue_key_aliases (
    source_project_id UUID        NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    key_number        INT         NOT NULL CHECK (key_number > 0),
    issue_id          UUID        NOT NULL REFERENCES issues (id) ON DELETE CASCADE,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (source_project_id, key_number)
);

CREATE INDEX IF NOT EXISTS idx_issue_key_aliases_issue ON issue_key_aliases (issue_id);
