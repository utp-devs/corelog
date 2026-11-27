-- Issue core v2: estimates, start date, flag, votes counter, soft delete and activity batches.
-- due_date becomes a calendar date (the time of day never carried meaning) and story points allow 999.5.

ALTER TABLE issues
    ADD COLUMN IF NOT EXISTS original_estimate_seconds  BIGINT      NULL,
    ADD COLUMN IF NOT EXISTS remaining_estimate_seconds BIGINT      NULL,
    ADD COLUMN IF NOT EXISTS start_date                 DATE        NULL,
    ADD COLUMN IF NOT EXISTS flagged                    BOOLEAN     NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS vote_count                 INT         NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS deleted_at                 TIMESTAMPTZ NULL;

ALTER TABLE issues
    ALTER COLUMN due_date TYPE DATE USING (due_date AT TIME ZONE 'UTC')::date,
    ALTER COLUMN story_points TYPE NUMERIC(5,1);

ALTER TABLE issues
    DROP CONSTRAINT IF EXISTS issues_original_estimate_check,
    DROP CONSTRAINT IF EXISTS issues_remaining_estimate_check,
    DROP CONSTRAINT IF EXISTS issues_vote_count_check,
    DROP CONSTRAINT IF EXISTS issues_story_points_check,
    DROP CONSTRAINT IF EXISTS issues_dates_order_check;

ALTER TABLE issues
    ADD CONSTRAINT issues_original_estimate_check CHECK (original_estimate_seconds IS NULL OR original_estimate_seconds >= 0),
    ADD CONSTRAINT issues_remaining_estimate_check CHECK (remaining_estimate_seconds IS NULL OR remaining_estimate_seconds >= 0),
    ADD CONSTRAINT issues_vote_count_check CHECK (vote_count >= 0),
    ADD CONSTRAINT issues_story_points_check CHECK (story_points IS NULL OR (story_points >= 0 AND story_points <= 999.5)),
    ADD CONSTRAINT issues_dates_order_check CHECK (start_date IS NULL OR due_date IS NULL OR start_date <= due_date);

-- Every live read filters on deleted_at IS NULL; the partial indexes keep those reads cheap.
CREATE INDEX IF NOT EXISTS idx_issues_project_live ON issues (project_id, rank) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_issues_parent_live ON issues (parent_id) WHERE deleted_at IS NULL AND parent_id IS NOT NULL;

-- One PATCH writes one batch of activities that share a batch id.
ALTER TABLE activities ADD COLUMN IF NOT EXISTS batch_id UUID NULL;
CREATE INDEX IF NOT EXISTS idx_activities_batch_id ON activities (batch_id) WHERE batch_id IS NOT NULL;
