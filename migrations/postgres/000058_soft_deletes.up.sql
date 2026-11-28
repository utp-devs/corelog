-- Logical deletion. Rows that carry history or feed statistics are no longer removed: deleting them stamps
-- deleted_at and every read ignores the stamped rows. Unique indexes become partial on deleted_at IS NULL so a
-- deleted row never blocks creating again the same name.

ALTER TABLE api_tokens        ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ NULL;
ALTER TABLE webhooks          ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ NULL;
ALTER TABLE automation_rules  ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ NULL;
ALTER TABLE attachments       ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ NULL;
ALTER TABLE issue_links       ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ NULL;
ALTER TABLE saved_filters     ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ NULL;
ALTER TABLE custom_fields     ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ NULL;
ALTER TABLE filters           ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ NULL;
ALTER TABLE dashboards        ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ NULL;
ALTER TABLE labels            ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ NULL;
ALTER TABLE components        ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ NULL;
ALTER TABLE versions          ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ NULL;
ALTER TABLE boards            ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ NULL;
ALTER TABLE sprints           ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ NULL;
ALTER TABLE teams             ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ NULL;
ALTER TABLE issue_types       ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ NULL;
ALTER TABLE priorities        ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ NULL;
ALTER TABLE resolutions       ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ NULL;

-- Revoked tokens are logically deleted: backfill them so the list hides them.
UPDATE api_tokens SET deleted_at = revoked_at WHERE revoked_at IS NOT NULL AND deleted_at IS NULL;

-- Unique constraints that were declared inline become partial unique indexes.
DO $$
DECLARE
    t text;
    c record;
BEGIN
    FOREACH t IN ARRAY ARRAY['labels', 'components', 'issue_links', 'saved_filters', 'teams'] LOOP
        FOR c IN SELECT conname FROM pg_constraint WHERE conrelid = t::regclass AND contype = 'u' LOOP
            EXECUTE format('ALTER TABLE %I DROP CONSTRAINT %I', t, c.conname);
        END LOOP;
    END LOOP;
END
$$;

CREATE UNIQUE INDEX IF NOT EXISTS uq_labels_project_name ON labels (project_id, name) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_components_project_name ON components (project_id, name) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_issue_links_pair ON issue_links (source_issue_id, target_issue_id, kind) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS saved_filters_owner_name_unique ON saved_filters (owner_id, name) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_teams_name ON teams (name) WHERE deleted_at IS NULL;

-- Unique indexes that already existed are rebuilt as partial ones under the same name.
DROP INDEX IF EXISTS uq_versions_project_name;
CREATE UNIQUE INDEX uq_versions_project_name ON versions (project_id, lower(name)) WHERE deleted_at IS NULL;

DROP INDEX IF EXISTS uq_custom_fields_org_name;
CREATE UNIQUE INDEX uq_custom_fields_org_name ON custom_fields (organization_id, lower(name)) WHERE deleted_at IS NULL;

DROP INDEX IF EXISTS uq_filters_owner_name;
CREATE UNIQUE INDEX uq_filters_owner_name ON filters (owner_id, lower(name)) WHERE deleted_at IS NULL;

DROP INDEX IF EXISTS uq_dashboards_owner_name;
CREATE UNIQUE INDEX uq_dashboards_owner_name ON dashboards (owner_id, lower(name)) WHERE deleted_at IS NULL;
DROP INDEX IF EXISTS uq_dashboards_owner_default;
CREATE UNIQUE INDEX uq_dashboards_owner_default ON dashboards (owner_id) WHERE is_default AND deleted_at IS NULL;

DROP INDEX IF EXISTS uq_boards_project_name;
CREATE UNIQUE INDEX uq_boards_project_name ON boards (project_id, lower(name)) WHERE deleted_at IS NULL;
DROP INDEX IF EXISTS idx_boards_one_default_per_project;
CREATE UNIQUE INDEX idx_boards_one_default_per_project ON boards (project_id) WHERE is_default AND deleted_at IS NULL;

DROP INDEX IF EXISTS idx_sprints_one_active_per_project;
CREATE UNIQUE INDEX idx_sprints_one_active_per_project ON sprints (project_id) WHERE status = 'active' AND deleted_at IS NULL;
DROP INDEX IF EXISTS idx_sprints_one_active_per_board;
CREATE UNIQUE INDEX idx_sprints_one_active_per_board ON sprints (board_id) WHERE status = 'active' AND board_id IS NOT NULL AND deleted_at IS NULL;

DROP INDEX IF EXISTS uq_issue_types_org_name;
CREATE UNIQUE INDEX uq_issue_types_org_name ON issue_types (organization_id, lower(name)) WHERE deleted_at IS NULL;
DROP INDEX IF EXISTS uq_issue_types_org_default;
CREATE UNIQUE INDEX uq_issue_types_org_default ON issue_types (organization_id) WHERE is_default AND deleted_at IS NULL;
DROP INDEX IF EXISTS uq_issue_types_org_hierarchy;
CREATE UNIQUE INDEX uq_issue_types_org_hierarchy ON issue_types (organization_id, hierarchy_level) WHERE hierarchy_level <> 0 AND deleted_at IS NULL;

DROP INDEX IF EXISTS uq_priorities_org_name;
CREATE UNIQUE INDEX uq_priorities_org_name ON priorities (organization_id, lower(name)) WHERE deleted_at IS NULL;
DROP INDEX IF EXISTS uq_priorities_org_default;
CREATE UNIQUE INDEX uq_priorities_org_default ON priorities (organization_id) WHERE is_default AND deleted_at IS NULL;

DROP INDEX IF EXISTS uq_resolutions_org_name;
CREATE UNIQUE INDEX uq_resolutions_org_name ON resolutions (organization_id, lower(name)) WHERE deleted_at IS NULL;
DROP INDEX IF EXISTS uq_resolutions_org_default;
CREATE UNIQUE INDEX uq_resolutions_org_default ON resolutions (organization_id) WHERE is_default AND deleted_at IS NULL;

-- Lookups of live rows.
CREATE INDEX IF NOT EXISTS idx_api_tokens_live ON api_tokens (user_id, created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_webhooks_live ON webhooks (organization_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_automation_rules_live ON automation_rules (project_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_sprints_live ON sprints (project_id) WHERE deleted_at IS NULL;
