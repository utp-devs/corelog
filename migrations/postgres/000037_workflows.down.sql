-- Reverts the workflow model. The legacy issues.status column was kept in sync, so issues need no
-- rewrite. Every live project gets back the legacy default workflow and its fixed transitions.
DROP TRIGGER IF EXISTS trg_issues_sync_status ON issues;
DROP FUNCTION IF EXISTS corelog_sync_issue_status();

DROP TABLE IF EXISTS board_column_statuses;
DROP TABLE IF EXISTS issue_status_history;

DROP INDEX IF EXISTS idx_issues_status_id;
ALTER TABLE issues DROP COLUMN IF EXISTS status_id;

DROP TABLE IF EXISTS workflows;
DROP TABLE IF EXISTS statuses;

DROP FUNCTION IF EXISTS corelog_status_for_legacy(uuid, text);
DROP FUNCTION IF EXISTS corelog_ensure_workflow(uuid);
DROP FUNCTION IF EXISTS corelog_workflow_template(text);
DROP FUNCTION IF EXISTS corelog_legacy_status(text);

CREATE TABLE IF NOT EXISTS workflows (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    name       VARCHAR(80) NOT NULL,
    is_default BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS workflow_transitions (
    workflow_id   UUID NOT NULL REFERENCES workflows (id) ON DELETE CASCADE,
    from_status   VARCHAR(20) NOT NULL,
    to_status     VARCHAR(20) NOT NULL,
    allowed_roles TEXT NOT NULL DEFAULT 'admin,lead,contributor,viewer',
    PRIMARY KEY (workflow_id, from_status, to_status)
);

CREATE INDEX IF NOT EXISTS idx_workflows_project_id ON workflows (project_id);
CREATE INDEX IF NOT EXISTS idx_workflow_transitions_from ON workflow_transitions (from_status);

INSERT INTO workflows (project_id, name, is_default)
SELECT p.id, 'Default', true FROM projects p;

INSERT INTO workflow_transitions (workflow_id, from_status, to_status, allowed_roles)
SELECT w.id, t.from_status, t.to_status, 'admin,lead,contributor,viewer'
FROM workflows w
CROSS JOIN (VALUES
    ('open', 'in_progress'),
    ('open', 'closed'),
    ('in_progress', 'resolved'),
    ('in_progress', 'closed'),
    ('resolved', 'closed'),
    ('resolved', 'in_progress')
) AS t (from_status, to_status);
