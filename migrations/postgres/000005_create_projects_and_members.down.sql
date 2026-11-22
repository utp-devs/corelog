DROP INDEX IF EXISTS idx_team_members_user_id;
DROP INDEX IF EXISTS idx_project_members_user_id;
DROP INDEX IF EXISTS idx_issues_project_id;

ALTER TABLE issues DROP CONSTRAINT IF EXISTS fk_issues_project;
ALTER TABLE issues DROP COLUMN IF EXISTS project_id;

ALTER INDEX idx_issues_status RENAME TO idx_tickets_status;
ALTER INDEX idx_issues_priority RENAME TO idx_tickets_priority;
ALTER INDEX idx_issues_requester_id RENAME TO idx_tickets_requester_id;
ALTER INDEX idx_issues_assignee_id RENAME TO idx_tickets_assignee_id;

ALTER TABLE issues RENAME TO tickets;
DROP TABLE IF EXISTS team_members;
DROP TABLE IF EXISTS project_members;
DROP TABLE IF EXISTS projects;
