CREATE TABLE IF NOT EXISTS labels (
    id         UUID PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    name       VARCHAR(40) NOT NULL,
    color      VARCHAR(20) NOT NULL DEFAULT 'blue',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (project_id, name)
);

CREATE TABLE IF NOT EXISTS issue_labels (
    issue_id UUID NOT NULL REFERENCES issues (id) ON DELETE CASCADE,
    label_id UUID NOT NULL REFERENCES labels (id) ON DELETE CASCADE,
    PRIMARY KEY (issue_id, label_id)
);

CREATE TABLE IF NOT EXISTS components (
    id          UUID PRIMARY KEY,
    project_id  UUID NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    name        VARCHAR(80) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (project_id, name)
);

CREATE TABLE IF NOT EXISTS issue_components (
    issue_id     UUID NOT NULL REFERENCES issues (id) ON DELETE CASCADE,
    component_id UUID NOT NULL REFERENCES components (id) ON DELETE CASCADE,
    PRIMARY KEY (issue_id, component_id)
);

CREATE INDEX IF NOT EXISTS idx_labels_project_id ON labels (project_id);
CREATE INDEX IF NOT EXISTS idx_issue_labels_label_id ON issue_labels (label_id);
CREATE INDEX IF NOT EXISTS idx_components_project_id ON components (project_id);
CREATE INDEX IF NOT EXISTS idx_issue_components_component_id ON issue_components (component_id);
