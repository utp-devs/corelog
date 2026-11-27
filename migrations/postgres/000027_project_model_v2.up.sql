-- Project model v2: templates, appearance, default assignee policy, issue number counter,
-- soft delete, per-organization key uniqueness over live rows, team grants, favorites and
-- recently viewed items.

-- Projects: new columns. The template defaults to kanban and is then backfilled from the legacy type.
ALTER TABLE projects
    ADD COLUMN IF NOT EXISTS template                TEXT NOT NULL DEFAULT 'kanban',
    ADD COLUMN IF NOT EXISTS color                   TEXT NOT NULL DEFAULT 'blue',
    ADD COLUMN IF NOT EXISTS icon                    TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS next_issue_number       INT  NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS default_assignee_policy TEXT NOT NULL DEFAULT 'unassigned',
    ADD COLUMN IF NOT EXISTS deleted_at              TIMESTAMPTZ NULL;

UPDATE projects SET template = 'scrum' WHERE type = 'scrum' AND template <> 'scrum';

ALTER TABLE projects DROP CONSTRAINT IF EXISTS projects_template_check;
ALTER TABLE projects
    ADD CONSTRAINT projects_template_check
    CHECK (template IN ('scrum', 'kanban', 'task_tracking', 'bug_tracking'));

ALTER TABLE projects DROP CONSTRAINT IF EXISTS projects_default_assignee_policy_check;
ALTER TABLE projects
    ADD CONSTRAINT projects_default_assignee_policy_check
    CHECK (default_assignee_policy IN ('unassigned', 'lead'));

-- Issue numbers: the counter must start after the issues that already exist. Issues have no
-- per-project number yet, so the legacy issues of each project count as numbers 1..n.
UPDATE projects p
SET next_issue_number = 1 + (SELECT count(*) FROM issues i WHERE i.project_id = p.id)
WHERE next_issue_number = 1;

-- Keys are unique per organization among live projects. A deleted project frees its key.
DROP INDEX IF EXISTS uq_projects_organization_key;
CREATE UNIQUE INDEX IF NOT EXISTS uq_projects_organization_key
    ON projects (organization_id, key) WHERE deleted_at IS NULL;

-- Listing by organization (sorted by name) over live rows.
CREATE INDEX IF NOT EXISTS idx_projects_organization_name
    ON projects (organization_id, name) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_projects_lead_id ON projects (lead_id);

-- Team grants on a project. The team role applies to every member of the team.
CREATE TABLE IF NOT EXISTS project_team_roles (
    project_id UUID NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    team_id    UUID NOT NULL REFERENCES teams (id) ON DELETE CASCADE,
    role       TEXT NOT NULL CHECK (role IN ('lead', 'contributor', 'viewer')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (project_id, team_id)
);
CREATE INDEX IF NOT EXISTS idx_project_team_roles_team_id ON project_team_roles (team_id);

-- Starred entities of a user. The entity id is not a foreign key: the target type decides the table.
CREATE TABLE IF NOT EXISTS favorites (
    user_id     UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    entity_type TEXT NOT NULL CHECK (entity_type IN ('project', 'filter', 'board', 'dashboard', 'issue', 'team')),
    entity_id   UUID NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, entity_type, entity_id)
);
CREATE INDEX IF NOT EXISTS idx_favorites_entity ON favorites (entity_type, entity_id);

-- Entities a user viewed most recently.
CREATE TABLE IF NOT EXISTS recent_items (
    user_id     UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    entity_type TEXT NOT NULL CHECK (entity_type IN ('project', 'filter', 'board', 'dashboard', 'issue', 'team')),
    entity_id   UUID NOT NULL,
    viewed_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, entity_type, entity_id)
);
CREATE INDEX IF NOT EXISTS idx_recent_items_user_type_viewed
    ON recent_items (user_id, entity_type, viewed_at DESC);
