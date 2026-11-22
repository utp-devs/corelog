DROP INDEX IF EXISTS idx_issues_rank;
DROP INDEX IF EXISTS idx_issues_due_date;
DROP INDEX IF EXISTS idx_issues_parent_id;
DROP INDEX IF EXISTS idx_issues_issue_type;

ALTER TABLE issues DROP CONSTRAINT IF EXISTS fk_issues_parent;
ALTER TABLE issues DROP CONSTRAINT IF EXISTS issues_issue_type_check;

ALTER TABLE issues
    DROP COLUMN IF EXISTS closed_at,
    DROP COLUMN IF EXISTS rank,
    DROP COLUMN IF EXISTS story_points,
    DROP COLUMN IF EXISTS due_date,
    DROP COLUMN IF EXISTS parent_id,
    DROP COLUMN IF EXISTS issue_type;
