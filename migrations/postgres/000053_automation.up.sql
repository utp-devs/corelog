-- Automation rules and their run log. A rule belongs to an organization and optionally to one project.
CREATE TABLE IF NOT EXISTS automation_rules (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    project_id      UUID NULL REFERENCES projects (id) ON DELETE CASCADE,
    name            VARCHAR(120) NOT NULL,
    enabled         BOOLEAN NOT NULL DEFAULT TRUE,
    trigger         JSONB NOT NULL CHECK (jsonb_typeof(trigger) = 'object'),
    conditions      JSONB NOT NULL DEFAULT '[]' CHECK (jsonb_typeof(conditions) = 'array'),
    actions         JSONB NOT NULL DEFAULT '[]' CHECK (jsonb_typeof(actions) = 'array'),
    created_by      UUID NULL REFERENCES users (id) ON DELETE SET NULL,
    version         INT NOT NULL DEFAULT 1,
    run_count       BIGINT NOT NULL DEFAULT 0,
    last_run_at     TIMESTAMPTZ NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_automation_rules_project ON automation_rules (project_id) WHERE project_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_automation_rules_org ON automation_rules (organization_id, enabled);
CREATE INDEX IF NOT EXISTS idx_automation_rules_created_by ON automation_rules (created_by);

CREATE TABLE IF NOT EXISTS automation_runs (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rule_id    UUID NOT NULL REFERENCES automation_rules (id) ON DELETE CASCADE,
    event_id   BIGINT NULL,
    issue_id   UUID NULL,
    status     VARCHAR(10) NOT NULL CHECK (status IN ('success', 'skipped', 'failed')),
    detail     TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_automation_runs_rule ON automation_runs (rule_id, created_at DESC);
