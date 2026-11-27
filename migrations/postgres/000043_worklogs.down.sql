ALTER TABLE issues DROP COLUMN IF EXISTS time_spent_seconds;
DROP INDEX IF EXISTS idx_worklogs_author_started;
DROP INDEX IF EXISTS idx_worklogs_issue_id;
DROP TABLE IF EXISTS worklogs;
