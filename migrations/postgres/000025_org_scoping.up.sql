-- Scope teams and projects to an organization. Every existing row is attached to
-- the default organization created in 000024 (fixed id 00000000-0000-4000-8000-000000000002).

-- Teams: organization scope and profile columns.
ALTER TABLE teams
    ADD COLUMN IF NOT EXISTS organization_id UUID,
    ADD COLUMN IF NOT EXISTS description     TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS avatar_color    VARCHAR(7),
    ADD COLUMN IF NOT EXISTS created_by      UUID REFERENCES users (id) ON DELETE SET NULL;

UPDATE teams
SET organization_id = '00000000-0000-4000-8000-000000000002'
WHERE organization_id IS NULL;

ALTER TABLE teams ALTER COLUMN organization_id SET NOT NULL;
ALTER TABLE teams DROP CONSTRAINT IF EXISTS fk_teams_organization;
ALTER TABLE teams
    ADD CONSTRAINT fk_teams_organization
    FOREIGN KEY (organization_id) REFERENCES organizations (id) ON DELETE CASCADE;
CREATE INDEX IF NOT EXISTS idx_teams_organization_id ON teams (organization_id);
CREATE INDEX IF NOT EXISTS idx_teams_created_by ON teams (created_by);

-- Team roles: owner becomes maintainer. The old CHECK must go before rewriting values.
ALTER TABLE team_members DROP CONSTRAINT IF EXISTS team_members_role_check;
UPDATE team_members SET role = 'maintainer' WHERE role = 'owner';
ALTER TABLE team_members ADD CONSTRAINT team_members_role_check CHECK (role IN ('maintainer', 'member'));

-- Projects: organization scope. Project keys become unique per organization instead
-- of globally. The composite unique index also serves lookups by organization_id.
ALTER TABLE projects ADD COLUMN IF NOT EXISTS organization_id UUID;

UPDATE projects
SET organization_id = '00000000-0000-4000-8000-000000000002'
WHERE organization_id IS NULL;

ALTER TABLE projects ALTER COLUMN organization_id SET NOT NULL;
ALTER TABLE projects DROP CONSTRAINT IF EXISTS fk_projects_organization;
ALTER TABLE projects
    ADD CONSTRAINT fk_projects_organization
    FOREIGN KEY (organization_id) REFERENCES organizations (id) ON DELETE CASCADE;

ALTER TABLE projects DROP CONSTRAINT IF EXISTS projects_key_key;
CREATE UNIQUE INDEX IF NOT EXISTS uq_projects_organization_key ON projects (organization_id, key);

-- Project visibility: the legacy value 'team' becomes 'open' (visible to every org member).
ALTER TABLE projects DROP CONSTRAINT IF EXISTS projects_visibility_check;
UPDATE projects SET visibility = 'open' WHERE visibility = 'team';
ALTER TABLE projects ALTER COLUMN visibility SET DEFAULT 'open';
ALTER TABLE projects ADD CONSTRAINT projects_visibility_check CHECK (visibility IN ('private', 'open'));
