CREATE TABLE IF NOT EXISTS projects (
    id UUID PRIMARY KEY,
    key VARCHAR(20) NOT NULL UNIQUE,
    name VARCHAR(120) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    type VARCHAR(10) NOT NULL CHECK (type IN ('kanban', 'scrum')) DEFAULT 'kanban',
    lead_id UUID NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    visibility VARCHAR(10) NOT NULL CHECK (visibility IN ('private', 'team')) DEFAULT 'team',
    archived_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS project_members (
    project_id UUID NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role VARCHAR(20) NOT NULL CHECK (role IN ('lead', 'contributor', 'viewer')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (project_id, user_id)
);

CREATE TABLE IF NOT EXISTS team_members (
    team_id UUID NOT NULL REFERENCES teams (id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role VARCHAR(20) NOT NULL CHECK (role IN ('owner', 'member')) DEFAULT 'member',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (team_id, user_id)
);

INSERT INTO team_members (team_id, user_id, role, created_at)
SELECT team_id, id, 'member', updated_at
FROM users
WHERE team_id IS NOT NULL
ON CONFLICT (team_id, user_id) DO NOTHING;

INSERT INTO projects (
    id, key, name, description, type, lead_id, visibility, archived_at, created_at, updated_at
)
SELECT
    '00000000-0000-4000-8000-000000000001',
    'CORE',
    'CoreLog',
    'Proyecto por defecto creado durante la migracion F2',
    'kanban',
    id,
    'team',
    NULL,
    updated_at,
    updated_at
FROM users
ORDER BY created_at ASC
LIMIT 1
ON CONFLICT (id) DO NOTHING;

INSERT INTO project_members (project_id, user_id, role, created_at)
SELECT
    '00000000-0000-4000-8000-000000000001',
    id,
    CASE WHEN role = 'admin' THEN 'lead' ELSE 'contributor' END,
    updated_at
FROM users
ON CONFLICT (project_id, user_id) DO NOTHING;

ALTER TABLE tickets RENAME TO issues;
ALTER TABLE issues ADD COLUMN project_id UUID;
UPDATE issues
SET project_id = '00000000-0000-4000-8000-000000000001';
ALTER TABLE issues ALTER COLUMN project_id SET NOT NULL;
ALTER TABLE issues
    ADD CONSTRAINT fk_issues_project
    FOREIGN KEY (project_id) REFERENCES projects (id) ON DELETE RESTRICT;

ALTER INDEX idx_tickets_status RENAME TO idx_issues_status;
ALTER INDEX idx_tickets_priority RENAME TO idx_issues_priority;
ALTER INDEX idx_tickets_requester_id RENAME TO idx_issues_requester_id;
ALTER INDEX idx_tickets_assignee_id RENAME TO idx_issues_assignee_id;
CREATE INDEX IF NOT EXISTS idx_issues_project_id ON issues (project_id);
CREATE INDEX IF NOT EXISTS idx_project_members_user_id ON project_members (user_id);
CREATE INDEX IF NOT EXISTS idx_team_members_user_id ON team_members (user_id);
