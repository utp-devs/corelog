DROP INDEX IF EXISTS idx_activities_batch_id;
ALTER TABLE activities DROP COLUMN IF EXISTS batch_id;

DROP INDEX IF EXISTS idx_issues_parent_live;
DROP INDEX IF EXISTS idx_issues_project_live;

ALTER TABLE issues
    DROP CONSTRAINT IF EXISTS issues_dates_order_check,
    DROP CONSTRAINT IF EXISTS issues_story_points_check,
    DROP CONSTRAINT IF EXISTS issues_vote_count_check,
    DROP CONSTRAINT IF EXISTS issues_remaining_estimate_check,
    DROP CONSTRAINT IF EXISTS issues_original_estimate_check;

ALTER TABLE issues
    ALTER COLUMN story_points TYPE NUMERIC(4,1),
    ALTER COLUMN due_date TYPE TIMESTAMPTZ USING (due_date::timestamp AT TIME ZONE 'UTC');

ALTER TABLE issues
    DROP COLUMN IF EXISTS deleted_at,
    DROP COLUMN IF EXISTS vote_count,
    DROP COLUMN IF EXISTS flagged,
    DROP COLUMN IF EXISTS start_date,
    DROP COLUMN IF EXISTS remaining_estimate_seconds,
    DROP COLUMN IF EXISTS original_estimate_seconds;
