-- Organizations (top-level tenant), their members and invitations.
-- Slugs are unique case-insensitively among live (not soft-deleted) organizations.
CREATE TABLE IF NOT EXISTS organizations (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug        VARCHAR(40) NOT NULL,
    name        VARCHAR(120) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    logo_key    TEXT,
    settings    JSONB NOT NULL DEFAULT '{
        "who_can_create_projects": "members",
        "who_can_create_teams": "members",
        "default_project_visibility": "open",
        "require_2fa": false,
        "max_upload_bytes": 10485760,
        "hours_per_day": 8,
        "days_per_week": 5,
        "timezone": "UTC"
    }'::jsonb,
    created_by  UUID REFERENCES users (id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_organizations_slug
    ON organizations (lower(slug));
CREATE INDEX IF NOT EXISTS idx_organizations_created_by ON organizations (created_by);

CREATE TABLE IF NOT EXISTS organization_members (
    organization_id UUID NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    user_id         UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role            VARCHAR(20) NOT NULL CHECK (role IN ('owner', 'admin', 'member', 'guest')),
    joined_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (organization_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_organization_members_user_id ON organization_members (user_id);

CREATE TABLE IF NOT EXISTS organization_invitations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    email           VARCHAR(255) NOT NULL,
    role            VARCHAR(20) NOT NULL CHECK (role IN ('owner', 'admin', 'member', 'guest')),
    token_hash      TEXT NOT NULL UNIQUE,
    invited_by      UUID REFERENCES users (id) ON DELETE SET NULL,
    expires_at      TIMESTAMPTZ NOT NULL,
    accepted_at     TIMESTAMPTZ,
    revoked_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- At most one pending invitation per organization and email (case-insensitive).
CREATE UNIQUE INDEX IF NOT EXISTS uq_organization_invitations_pending
    ON organization_invitations (organization_id, lower(email))
    WHERE accepted_at IS NULL AND revoked_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_organization_invitations_invited_by ON organization_invitations (invited_by);
