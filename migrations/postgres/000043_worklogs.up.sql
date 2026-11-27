-- Time tracking: worklogs record the time spent on an issue; issues keep an aggregate of the live entries.
CREATE TABLE IF NOT EXISTS worklogs (
    id                       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    issue_id                 UUID NOT NULL REFERENCES issues (id) ON DELETE CASCADE,
    author_id                UUID NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    started_at               TIMESTAMPTZ NOT NULL,
    time_spent_seconds       INT NOT NULL,
    description              TEXT NOT NULL DEFAULT '',
    remaining_mode           VARCHAR(10) NOT NULL DEFAULT 'auto',
    remaining_adjust_seconds BIGINT NOT NULL DEFAULT 0,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at               TIMESTAMPTZ NULL,
    CONSTRAINT worklogs_time_spent_check CHECK (time_spent_seconds > 0 AND time_spent_seconds <= 259200),
    CONSTRAINT worklogs_description_check CHECK (char_length(description) <= 5000),
    CONSTRAINT worklogs_remaining_mode_check CHECK (remaining_mode IN ('auto', 'set', 'reduce', 'leave')),
    CONSTRAINT worklogs_remaining_adjust_check CHECK (remaining_adjust_seconds >= 0)
);

CREATE INDEX IF NOT EXISTS idx_worklogs_issue_id ON worklogs (issue_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_worklogs_author_started ON worklogs (author_id, started_at) WHERE deleted_at IS NULL;

-- Aggregate of the live worklogs of the issue. It is recomputed from worklogs in the same transaction that
-- changes them, so it never drifts; the issue view can read it without a join.
ALTER TABLE issues
    ADD COLUMN IF NOT EXISTS time_spent_seconds BIGINT NOT NULL DEFAULT 0 CHECK (time_spent_seconds >= 0);
