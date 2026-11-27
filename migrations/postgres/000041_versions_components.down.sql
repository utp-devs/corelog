DROP INDEX IF EXISTS idx_components_lead_id;

ALTER TABLE components
    DROP COLUMN IF EXISTS default_assignee_policy,
    DROP COLUMN IF EXISTS lead_id;

DROP INDEX IF EXISTS idx_issue_versions_version;
DROP TABLE IF EXISTS issue_versions;

DROP INDEX IF EXISTS idx_versions_project_status;
DROP INDEX IF EXISTS uq_versions_project_name;
DROP TABLE IF EXISTS versions;
