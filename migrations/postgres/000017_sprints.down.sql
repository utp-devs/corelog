DROP INDEX IF EXISTS idx_issues_sprint_id;
ALTER TABLE issues DROP COLUMN IF EXISTS sprint_id;
DROP INDEX IF EXISTS idx_sprints_project_id;
DROP INDEX IF EXISTS idx_sprints_one_active_per_project;
DROP TABLE IF EXISTS sprints;
