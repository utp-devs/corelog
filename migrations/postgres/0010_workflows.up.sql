CREATE TABLE IF NOT EXISTS workflows (
    id         UUID PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    name       VARCHAR(80) NOT NULL,
    is_default BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS workflow_transitions (
    workflow_id UUID NOT NULL REFERENCES workflows (id) ON DELETE CASCADE,
    from_status VARCHAR(20) NOT NULL,
    to_status   VARCHAR(20) NOT NULL,
    allowed_roles TEXT NOT NULL DEFAULT 'admin,agent,requester',
    PRIMARY KEY (workflow_id, from_status, to_status)
);

CREATE INDEX IF NOT EXISTS idx_workflows_project_id ON workflows (project_id);
CREATE INDEX IF NOT EXISTS idx_workflow_transitions_from ON workflow_transitions (from_status);

INSERT INTO workflows (id, project_id, name, is_default)
SELECT
    '00000000-0000-4000-8000-000000000002',
    p.id,
    'Workflow por defecto',
    true
FROM projects p
WHERE p.id = '00000000-0000-4000-8000-000000000001'
ON CONFLICT (id) DO NOTHING;

INSERT INTO workflow_transitions (workflow_id, from_status, to_status, allowed_roles)
SELECT w.id, t.from_status, t.to_status, 'admin,agent,requester'
FROM workflows w
CROSS JOIN (VALUES
    ('open', 'in_progress'),
    ('open', 'closed'),
    ('in_progress', 'resolved'),
    ('in_progress', 'closed'),
    ('resolved', 'closed'),
    ('resolved', 'in_progress')
) AS t(from_status, to_status)
WHERE w.id = '00000000-0000-4000-8000-000000000002'
ON CONFLICT (workflow_id, from_status, to_status) DO NOTHING;
