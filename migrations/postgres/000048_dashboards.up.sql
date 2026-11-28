-- Dashboards with gadgets and sharing, plus the manual order of epics on the roadmap.
-- layout is {"template": "1" | "2" | "3" | "2-1" | "1-2"}. Gadget types and their configuration are validated by
-- the application, so the catalog can grow without a migration. dashboard_shares.target_id is polymorphic (user,
-- team, project or organization) and carries no foreign key, like the filter shares.

CREATE TABLE IF NOT EXISTS dashboards (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id        UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    organization_id UUID NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    name            VARCHAR(120) NOT NULL CHECK (char_length(btrim(name)) > 0),
    description     TEXT NOT NULL DEFAULT '' CHECK (char_length(description) <= 500),
    layout          JSONB NOT NULL DEFAULT '{"template": "2"}' CHECK (jsonb_typeof(layout) = 'object'),
    is_default      BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_dashboards_owner_name ON dashboards (owner_id, lower(name));
CREATE UNIQUE INDEX IF NOT EXISTS uq_dashboards_owner_default ON dashboards (owner_id) WHERE is_default;
CREATE INDEX IF NOT EXISTS idx_dashboards_organization ON dashboards (organization_id);

CREATE TABLE IF NOT EXISTS dashboard_gadgets (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    dashboard_id UUID NOT NULL REFERENCES dashboards (id) ON DELETE CASCADE,
    type         VARCHAR(40) NOT NULL CHECK (char_length(type) > 0),
    title        VARCHAR(120) NOT NULL DEFAULT '',
    config       JSONB NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(config) = 'object'),
    position     INT NOT NULL DEFAULT 0 CHECK (position >= 0),
    column_index SMALLINT NOT NULL DEFAULT 0 CHECK (column_index BETWEEN 0 AND 2),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_dashboard_gadgets_dashboard ON dashboard_gadgets (dashboard_id, column_index, position);

CREATE TABLE IF NOT EXISTS dashboard_shares (
    dashboard_id UUID NOT NULL REFERENCES dashboards (id) ON DELETE CASCADE,
    share_type   TEXT NOT NULL CHECK (share_type IN ('user', 'team', 'project', 'organization')),
    target_id    UUID NOT NULL,
    permission   TEXT NOT NULL DEFAULT 'view' CHECK (permission IN ('view', 'edit')),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (dashboard_id, share_type, target_id)
);

CREATE INDEX IF NOT EXISTS idx_dashboard_shares_target ON dashboard_shares (share_type, target_id);

-- Manual order of the epics of a project on the roadmap. It is independent of the backlog rank of the issue.
CREATE TABLE IF NOT EXISTS roadmap_epic_ranks (
    project_id UUID NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    issue_id   UUID NOT NULL REFERENCES issues (id) ON DELETE CASCADE,
    position   INT NOT NULL CHECK (position >= 0),
    PRIMARY KEY (project_id, issue_id)
);

CREATE INDEX IF NOT EXISTS idx_roadmap_epic_ranks_issue ON roadmap_epic_ranks (issue_id);

-- The activity stream gadget reads the newest activity across projects.
CREATE INDEX IF NOT EXISTS idx_activities_created_at ON activities (created_at DESC, id);
