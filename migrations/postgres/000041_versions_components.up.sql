-- Versions (releases) of a project and their links to issues, plus the lead and the default assignee policy of
-- components. A version is unreleased until it is released; released and archived versions do not take new fix
-- issues unless an administrator overrides the rule in the application.

CREATE TABLE IF NOT EXISTS versions (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id   UUID NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    name         VARCHAR(80) NOT NULL CHECK (char_length(btrim(name)) > 0),
    description  TEXT NOT NULL DEFAULT '' CHECK (char_length(description) <= 4000),
    start_date   DATE NULL,
    release_date DATE NULL,
    status       TEXT NOT NULL DEFAULT 'unreleased' CHECK (status IN ('unreleased', 'released', 'archived')),
    released_at  TIMESTAMPTZ NULL,
    archived_at  TIMESTAMPTZ NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Names are unique per project ignoring case.
CREATE UNIQUE INDEX IF NOT EXISTS uq_versions_project_name ON versions (project_id, lower(name));
CREATE INDEX IF NOT EXISTS idx_versions_project_status ON versions (project_id, status);

CREATE TABLE IF NOT EXISTS issue_versions (
    issue_id   UUID NOT NULL REFERENCES issues (id) ON DELETE CASCADE,
    version_id UUID NOT NULL REFERENCES versions (id) ON DELETE CASCADE,
    kind       TEXT NOT NULL CHECK (kind IN ('fix', 'affects')),
    PRIMARY KEY (issue_id, version_id, kind)
);

CREATE INDEX IF NOT EXISTS idx_issue_versions_version ON issue_versions (version_id, kind);

ALTER TABLE components
    ADD COLUMN IF NOT EXISTS lead_id                UUID NULL REFERENCES users (id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS default_assignee_policy TEXT NOT NULL DEFAULT 'none'
        CHECK (default_assignee_policy IN ('none', 'component_lead', 'project_lead'));

CREATE INDEX IF NOT EXISTS idx_components_lead_id ON components (lead_id);
