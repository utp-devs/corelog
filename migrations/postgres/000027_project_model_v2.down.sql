-- Reverts 000027. Fails if two projects of different organizations share a key, or if a
-- soft deleted project shares its key with a live one: resolve those duplicates before rolling back.

DROP TABLE IF EXISTS recent_items;
DROP TABLE IF EXISTS favorites;
DROP TABLE IF EXISTS project_team_roles;

DROP INDEX IF EXISTS idx_projects_lead_id;
DROP INDEX IF EXISTS idx_projects_organization_name;
DROP INDEX IF EXISTS uq_projects_organization_key;
CREATE UNIQUE INDEX IF NOT EXISTS uq_projects_organization_key ON projects (organization_id, key);

ALTER TABLE projects DROP CONSTRAINT IF EXISTS projects_default_assignee_policy_check;
ALTER TABLE projects DROP CONSTRAINT IF EXISTS projects_template_check;

ALTER TABLE projects
    DROP COLUMN IF EXISTS deleted_at,
    DROP COLUMN IF EXISTS default_assignee_policy,
    DROP COLUMN IF EXISTS next_issue_number,
    DROP COLUMN IF EXISTS icon,
    DROP COLUMN IF EXISTS color,
    DROP COLUMN IF EXISTS template;
