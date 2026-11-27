DROP TRIGGER IF EXISTS trg_issues_sprint_points_event ON issues;
DROP FUNCTION IF EXISTS corelog_sprint_points_event();
DROP TABLE IF EXISTS sprint_issue_events;
DROP FUNCTION IF EXISTS corelog_issue_category(uuid, text);
DROP INDEX IF EXISTS idx_sprints_one_active_per_board;
DROP INDEX IF EXISTS idx_sprints_board_id;
ALTER TABLE sprints
    DROP COLUMN IF EXISTS board_id,
    DROP COLUMN IF EXISTS completed_points,
    DROP COLUMN IF EXISTS committed_points,
    DROP COLUMN IF EXISTS completed_at,
    DROP COLUMN IF EXISTS started_at;
