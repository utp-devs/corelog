-- CSV import jobs of a project. The uploaded file lives in file storage under file_ref.
CREATE TABLE IF NOT EXISTS import_jobs (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id     UUID NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    user_id        UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    source         VARCHAR(10) NOT NULL DEFAULT 'csv' CHECK (source IN ('csv')),
    status         VARCHAR(12) NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'validating', 'running', 'done', 'failed')),
    total_rows     INT NOT NULL DEFAULT 0,
    processed_rows INT NOT NULL DEFAULT 0,
    created_rows   INT NOT NULL DEFAULT 0,
    error_rows     INT NOT NULL DEFAULT 0,
    dry_run        BOOLEAN NOT NULL DEFAULT FALSE,
    file_name      VARCHAR(255) NOT NULL DEFAULT '',
    mapping        JSONB NOT NULL DEFAULT '{}',
    file_ref       TEXT NOT NULL DEFAULT '',
    started        BOOLEAN NOT NULL DEFAULT FALSE,
    error_report   JSONB NOT NULL DEFAULT '[]',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at    TIMESTAMPTZ NULL
);

CREATE INDEX IF NOT EXISTS idx_import_jobs_project ON import_jobs (project_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_import_jobs_user ON import_jobs (user_id);
CREATE INDEX IF NOT EXISTS idx_import_jobs_pending ON import_jobs (created_at) WHERE status = 'queued' AND started;
