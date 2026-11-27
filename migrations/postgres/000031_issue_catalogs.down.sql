DROP INDEX IF EXISTS idx_issues_resolution_id;
DROP INDEX IF EXISTS idx_issues_priority_id;
DROP INDEX IF EXISTS idx_issues_issue_type_id;

-- The legacy enum columns were never dropped, so the issues keep their previous values.
ALTER TABLE issues
    DROP COLUMN IF EXISTS resolved_at,
    DROP COLUMN IF EXISTS resolution_id,
    DROP COLUMN IF EXISTS priority_id,
    DROP COLUMN IF EXISTS issue_type_id;

DROP TABLE IF EXISTS project_issue_types;
DROP TABLE IF EXISTS resolutions;
DROP TABLE IF EXISTS priorities;
DROP TABLE IF EXISTS issue_types;
