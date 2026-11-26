-- Project visibility goes back to the legacy spelling.
ALTER TABLE projects DROP CONSTRAINT IF EXISTS projects_visibility_check;
UPDATE projects SET visibility = 'team' WHERE visibility = 'open';
ALTER TABLE projects ALTER COLUMN visibility SET DEFAULT 'team';
ALTER TABLE projects ADD CONSTRAINT projects_visibility_check CHECK (visibility IN ('private', 'team'));

-- Restores the global unique on projects.key. Fails if keys are duplicated across
-- organizations, which is expected: resolve duplicates before rolling back.
DROP INDEX IF EXISTS uq_projects_organization_key;
ALTER TABLE projects DROP CONSTRAINT IF EXISTS fk_projects_organization;
ALTER TABLE projects DROP COLUMN IF EXISTS organization_id;
ALTER TABLE projects ADD CONSTRAINT projects_key_key UNIQUE (key);

-- Team roles: maintainer goes back to owner.
ALTER TABLE team_members DROP CONSTRAINT IF EXISTS team_members_role_check;
UPDATE team_members SET role = 'owner' WHERE role = 'maintainer';
ALTER TABLE team_members ADD CONSTRAINT team_members_role_check CHECK (role IN ('owner', 'member'));

DROP INDEX IF EXISTS idx_teams_created_by;
DROP INDEX IF EXISTS idx_teams_organization_id;
ALTER TABLE teams DROP CONSTRAINT IF EXISTS fk_teams_organization;
ALTER TABLE teams
    DROP COLUMN IF EXISTS created_by,
    DROP COLUMN IF EXISTS avatar_color,
    DROP COLUMN IF EXISTS description,
    DROP COLUMN IF EXISTS organization_id;
