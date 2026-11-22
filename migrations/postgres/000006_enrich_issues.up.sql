ALTER TABLE issues
    ADD COLUMN IF NOT EXISTS project_id UUID,
    ADD COLUMN IF NOT EXISTS issue_type VARCHAR(20) NOT NULL DEFAULT 'task',
    ADD COLUMN IF NOT EXISTS parent_id UUID,
    ADD COLUMN IF NOT EXISTS due_date TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS story_points NUMERIC(4,1),
    ADD COLUMN IF NOT EXISTS rank VARCHAR(64) NOT NULL DEFAULT 'm',
    ADD COLUMN IF NOT EXISTS closed_at TIMESTAMPTZ;

UPDATE issues
SET project_id = '00000000-0000-4000-8000-000000000001'
WHERE project_id IS NULL;

ALTER TABLE issues
    ALTER COLUMN project_id SET NOT NULL,
    DROP CONSTRAINT IF EXISTS fk_issues_project,
    ADD CONSTRAINT fk_issues_project
        FOREIGN KEY (project_id) REFERENCES projects (id) ON DELETE RESTRICT;

ALTER TABLE issues
    DROP CONSTRAINT IF EXISTS issues_issue_type_check,
    ADD CONSTRAINT issues_issue_type_check
        CHECK (issue_type IN ('task', 'bug', 'story', 'epic', 'subtask'));

ALTER TABLE issues
    DROP CONSTRAINT IF EXISTS issues_parent_id_check,
    ADD CONSTRAINT fk_issues_parent
        FOREIGN KEY (parent_id) REFERENCES issues (id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_issues_issue_type ON issues (issue_type);
CREATE INDEX IF NOT EXISTS idx_issues_parent_id ON issues (parent_id);
CREATE INDEX IF NOT EXISTS idx_issues_due_date ON issues (due_date);
CREATE INDEX IF NOT EXISTS idx_issues_rank ON issues (rank);
