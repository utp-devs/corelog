CREATE TABLE IF NOT EXISTS boards (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    name VARCHAR(80) NOT NULL,
    type VARCHAR(10) NOT NULL CHECK (type IN ('kanban', 'scrum')),
    config JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(config) = 'object'),
    is_default BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_boards_one_default_per_project
    ON boards (project_id) WHERE is_default;
CREATE INDEX IF NOT EXISTS idx_boards_project_id ON boards (project_id, created_at, id);

CREATE TABLE IF NOT EXISTS board_columns (
    id UUID PRIMARY KEY,
    board_id UUID NOT NULL REFERENCES boards (id) ON DELETE CASCADE,
    name VARCHAR(80) NOT NULL,
    status VARCHAR(20) NOT NULL CHECK (status IN ('open', 'in_progress', 'resolved', 'closed')),
    position INTEGER NOT NULL CHECK (position >= 0),
    wip_limit INTEGER CHECK (wip_limit IS NULL OR wip_limit > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (board_id, status),
    UNIQUE (board_id, position)
);

WITH new_boards AS (
    INSERT INTO boards (id, project_id, name, type, config, is_default)
    SELECT gen_random_uuid(), p.id, 'Default', p.type, '{}'::jsonb, true
    FROM projects p
    WHERE NOT EXISTS (
        SELECT 1 FROM boards b WHERE b.project_id = p.id AND b.is_default
    )
    RETURNING id
)
INSERT INTO board_columns (id, board_id, name, status, position)
SELECT gen_random_uuid(), b.id, c.name, c.status, c.position
FROM new_boards b
CROSS JOIN (VALUES
    ('Open', 'open', 0),
    ('In progress', 'in_progress', 1),
    ('Resolved', 'resolved', 2),
    ('Closed', 'closed', 3)
) AS c(name, status, position)
ON CONFLICT (board_id, status) DO NOTHING;
