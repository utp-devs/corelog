-- Issue catalogs: issue types, priorities and resolutions are configured per organization. Issues reference
-- them by id. The legacy enum columns issues.issue_type and issues.priority stay in place until the workflow
-- phase drops them; the application keeps them in sync through the legacy_key of each catalog item.
-- Palette keys (icon, color) are validated by the application and mirrored here by CHECK constraints.

CREATE TABLE IF NOT EXISTS issue_types (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    name            VARCHAR(60) NOT NULL CHECK (char_length(btrim(name)) > 0),
    description     TEXT NOT NULL DEFAULT '' CHECK (char_length(description) <= 500),
    icon            TEXT NOT NULL DEFAULT 'task' CHECK (icon IN (
                        'task', 'bug', 'story', 'epic', 'subtask', 'star', 'flag', 'bolt', 'alert', 'check',
                        'layers', 'tag', 'book', 'rocket', 'shield', 'clock', 'circle', 'arrow-up',
                        'arrow-down', 'minus', 'chevrons-up', 'chevrons-down')),
    color           TEXT NOT NULL DEFAULT 'blue' CHECK (color IN (
                        'gray', 'red', 'orange', 'amber', 'green', 'teal', 'blue', 'indigo', 'purple', 'pink')),
    hierarchy_level SMALLINT NOT NULL DEFAULT 0 CHECK (hierarchy_level IN (-1, 0, 1)),
    position        INT NOT NULL DEFAULT 0 CHECK (position >= 0),
    is_default      BOOLEAN NOT NULL DEFAULT FALSE,
    legacy_key      TEXT NULL CHECK (legacy_key IN ('task', 'bug', 'story', 'epic', 'subtask')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Names are unique per organization ignoring case; the unique index also serves lookups by organization.
CREATE UNIQUE INDEX IF NOT EXISTS uq_issue_types_org_name ON issue_types (organization_id, lower(name));
-- One default per organization, at most one epic-level type and one subtask-level type.
CREATE UNIQUE INDEX IF NOT EXISTS uq_issue_types_org_default ON issue_types (organization_id) WHERE is_default;
CREATE UNIQUE INDEX IF NOT EXISTS uq_issue_types_org_hierarchy
    ON issue_types (organization_id, hierarchy_level) WHERE hierarchy_level <> 0;
CREATE INDEX IF NOT EXISTS idx_issue_types_org_position ON issue_types (organization_id, position);

CREATE TABLE IF NOT EXISTS priorities (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    name            VARCHAR(40) NOT NULL CHECK (char_length(btrim(name)) > 0),
    icon            TEXT NOT NULL DEFAULT 'minus' CHECK (icon IN (
                        'task', 'bug', 'story', 'epic', 'subtask', 'star', 'flag', 'bolt', 'alert', 'check',
                        'layers', 'tag', 'book', 'rocket', 'shield', 'clock', 'circle', 'arrow-up',
                        'arrow-down', 'minus', 'chevrons-up', 'chevrons-down')),
    color           TEXT NOT NULL DEFAULT 'gray' CHECK (color IN (
                        'gray', 'red', 'orange', 'amber', 'green', 'teal', 'blue', 'indigo', 'purple', 'pink')),
    position        INT NOT NULL DEFAULT 0 CHECK (position >= 0),
    is_default      BOOLEAN NOT NULL DEFAULT FALSE,
    legacy_key      TEXT NULL CHECK (legacy_key IN ('low', 'medium', 'high', 'critical')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_priorities_org_name ON priorities (organization_id, lower(name));
CREATE UNIQUE INDEX IF NOT EXISTS uq_priorities_org_default ON priorities (organization_id) WHERE is_default;
CREATE INDEX IF NOT EXISTS idx_priorities_org_position ON priorities (organization_id, position);

CREATE TABLE IF NOT EXISTS resolutions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    name            VARCHAR(60) NOT NULL CHECK (char_length(btrim(name)) > 0),
    description     TEXT NOT NULL DEFAULT '' CHECK (char_length(description) <= 500),
    position        INT NOT NULL DEFAULT 0 CHECK (position >= 0),
    is_default      BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_resolutions_org_name ON resolutions (organization_id, lower(name));
CREATE UNIQUE INDEX IF NOT EXISTS uq_resolutions_org_default ON resolutions (organization_id) WHERE is_default;
CREATE INDEX IF NOT EXISTS idx_resolutions_org_position ON resolutions (organization_id, position);

-- Issue types enabled for a project. A project without rows uses every issue type of its organization.
CREATE TABLE IF NOT EXISTS project_issue_types (
    project_id    UUID NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    issue_type_id UUID NOT NULL REFERENCES issue_types (id) ON DELETE CASCADE,
    position      INT NOT NULL DEFAULT 0 CHECK (position >= 0),
    PRIMARY KEY (project_id, issue_type_id)
);
CREATE INDEX IF NOT EXISTS idx_project_issue_types_issue_type_id ON project_issue_types (issue_type_id);

-- Default catalogs for every existing organization (the same values organization creation seeds).
INSERT INTO issue_types (organization_id, name, description, icon, color, hierarchy_level, position, is_default, legacy_key)
SELECT o.id, v.name, '', v.icon, v.color, v.level, v.position, v.is_default, v.legacy_key
FROM organizations o
CROSS JOIN (VALUES
    ('Épica',    'epic',    'purple', 1,   0, FALSE, 'epic'),
    ('Historia', 'story',   'green',  0,   1, FALSE, 'story'),
    ('Tarea',    'task',    'blue',   0,   2, TRUE,  'task'),
    ('Error',    'bug',     'red',    0,   3, FALSE, 'bug'),
    ('Subtarea', 'subtask', 'teal',  -1,   4, FALSE, 'subtask')
) AS v (name, icon, color, level, position, is_default, legacy_key)
ON CONFLICT DO NOTHING;

INSERT INTO priorities (organization_id, name, icon, color, position, is_default, legacy_key)
SELECT o.id, v.name, v.icon, v.color, v.position, v.is_default, v.legacy_key
FROM organizations o
CROSS JOIN (VALUES
    ('Máxima', 'chevrons-up',   'red',    0, FALSE, 'critical'),
    ('Alta',   'arrow-up',      'orange', 1, FALSE, 'high'),
    ('Media',  'minus',         'amber',  2, TRUE,  'medium'),
    ('Baja',   'arrow-down',    'blue',   3, FALSE, 'low'),
    ('Mínima', 'chevrons-down', 'gray',   4, FALSE, 'low')
) AS v (name, icon, color, position, is_default, legacy_key)
ON CONFLICT DO NOTHING;

INSERT INTO resolutions (organization_id, name, description, position, is_default)
SELECT o.id, v.name, '', v.position, v.is_default
FROM organizations o
CROSS JOIN (VALUES
    ('Hecho',           0, TRUE),
    ('Descartado',      1, FALSE),
    ('Duplicado',       2, FALSE),
    ('No se reproduce', 3, FALSE),
    ('No se hará',      4, FALSE)
) AS v (name, position, is_default)
ON CONFLICT DO NOTHING;

ALTER TABLE issues
    ADD COLUMN IF NOT EXISTS issue_type_id UUID NULL REFERENCES issue_types (id) ON DELETE RESTRICT,
    ADD COLUMN IF NOT EXISTS priority_id   UUID NULL REFERENCES priorities (id) ON DELETE RESTRICT,
    ADD COLUMN IF NOT EXISTS resolution_id UUID NULL REFERENCES resolutions (id) ON DELETE RESTRICT,
    ADD COLUMN IF NOT EXISTS resolved_at   TIMESTAMPTZ NULL;

-- Backfill: every legacy issue gets the catalog item of its organization that carries the same legacy key.
UPDATE issues i
SET issue_type_id = (
    SELECT it.id
    FROM issue_types it
    JOIN projects p ON p.organization_id = it.organization_id
    WHERE p.id = i.project_id AND it.legacy_key = i.issue_type
    LIMIT 1
);

UPDATE issues i
SET priority_id = (
    SELECT pr.id
    FROM priorities pr
    JOIN projects p ON p.organization_id = pr.organization_id
    WHERE p.id = i.project_id AND pr.legacy_key = i.priority
    ORDER BY pr.position ASC
    LIMIT 1
);

-- Legacy resolved and closed issues are done: they get the default resolution and the resolution time.
UPDATE issues i
SET resolution_id = (
        SELECT rs.id
        FROM resolutions rs
        JOIN projects p ON p.organization_id = rs.organization_id
        WHERE p.id = i.project_id AND rs.is_default
        LIMIT 1
    ),
    resolved_at = COALESCE(i.closed_at, i.updated_at)
WHERE i.status IN ('resolved', 'closed');

-- Fails loudly if any legacy issue could not be mapped to a catalog item.
ALTER TABLE issues
    ALTER COLUMN issue_type_id SET NOT NULL,
    ALTER COLUMN priority_id SET NOT NULL;

CREATE INDEX IF NOT EXISTS idx_issues_issue_type_id ON issues (issue_type_id);
CREATE INDEX IF NOT EXISTS idx_issues_priority_id ON issues (priority_id);
CREATE INDEX IF NOT EXISTS idx_issues_resolution_id ON issues (resolution_id);
