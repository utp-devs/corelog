-- Custom fields: organization-level definitions, their contexts (which projects and issue types show them and
-- whether they are required, with an optional default) and the per-project field layout of each issue type.
-- Values live in issues.custom, keyed by the field id. The application validates values per field type.

CREATE TABLE IF NOT EXISTS custom_fields (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    name            VARCHAR(60) NOT NULL CHECK (char_length(btrim(name)) > 0),
    description     TEXT NOT NULL DEFAULT '' CHECK (char_length(description) <= 500),
    field_type      TEXT NOT NULL CHECK (field_type IN (
                        'text', 'paragraph', 'number', 'date', 'datetime', 'select', 'multiselect',
                        'checkbox', 'user', 'url', 'labels')),
    options         JSONB NOT NULL DEFAULT '[]' CHECK (jsonb_typeof(options) = 'array'),
    settings        JSONB NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(settings) = 'object'),
    position        INT NOT NULL DEFAULT 0 CHECK (position >= 0),
    is_system       BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_custom_fields_org_name ON custom_fields (organization_id, lower(name));
CREATE INDEX IF NOT EXISTS idx_custom_fields_org_position ON custom_fields (organization_id, position);

-- A context scopes a field to one project (NULL = every project of the organization) and one issue type
-- (NULL = every issue type). Each (field, project, type) combination appears once.
CREATE TABLE IF NOT EXISTS custom_field_contexts (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    field_id      UUID NOT NULL REFERENCES custom_fields (id) ON DELETE CASCADE,
    project_id    UUID NULL REFERENCES projects (id) ON DELETE CASCADE,
    issue_type_id UUID NULL REFERENCES issue_types (id) ON DELETE CASCADE,
    required      BOOLEAN NOT NULL DEFAULT FALSE,
    default_value JSONB NULL,
    position      INT NOT NULL DEFAULT 0 CHECK (position >= 0),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_custom_field_contexts_scope ON custom_field_contexts (
    field_id,
    (COALESCE(project_id, '00000000-0000-0000-0000-000000000000'::uuid)),
    (COALESCE(issue_type_id, '00000000-0000-0000-0000-000000000000'::uuid)));
CREATE INDEX IF NOT EXISTS idx_custom_field_contexts_project_id ON custom_field_contexts (project_id);
CREATE INDEX IF NOT EXISTS idx_custom_field_contexts_issue_type_id ON custom_field_contexts (issue_type_id);

-- Ordered layout of one issue type in one project. items is a JSON array of
-- {"kind": "system", "key": "<system field>", "required": bool} or {"kind": "custom", "field_id": "<uuid>", "required": bool}.
CREATE TABLE IF NOT EXISTS project_field_layouts (
    project_id    UUID NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    issue_type_id UUID NOT NULL REFERENCES issue_types (id) ON DELETE CASCADE,
    items         JSONB NOT NULL DEFAULT '[]' CHECK (jsonb_typeof(items) = 'array'),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (project_id, issue_type_id)
);
CREATE INDEX IF NOT EXISTS idx_project_field_layouts_issue_type_id ON project_field_layouts (issue_type_id);

-- Values of custom fields, keyed by field id. Issues created before this migration have no values.
ALTER TABLE issues ADD COLUMN IF NOT EXISTS custom JSONB NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(custom) = 'object');
