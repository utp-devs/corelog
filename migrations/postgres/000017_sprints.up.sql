CREATE TABLE IF NOT EXISTS sprints (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    name VARCHAR(80) NOT NULL,
    goal VARCHAR(500) NOT NULL DEFAULT '',
    status VARCHAR(10) NOT NULL DEFAULT 'planned' CHECK (status IN ('planned', 'active', 'closed')),
    start_date DATE,
    end_date DATE,
    closed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (start_date IS NULL OR end_date IS NULL OR end_date >= start_date)
);

-- Como mucho un sprint activo por proyecto.
CREATE UNIQUE INDEX IF NOT EXISTS idx_sprints_one_active_per_project
    ON sprints (project_id) WHERE status = 'active';
CREATE INDEX IF NOT EXISTS idx_sprints_project_id ON sprints (project_id, created_at, id);

ALTER TABLE issues ADD COLUMN IF NOT EXISTS sprint_id UUID REFERENCES sprints (id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_issues_sprint_id ON issues (sprint_id);
