-- Editable permission scheme per project. Absence of rows means the default scheme. Each row grants
-- one permission to one holder (a project role, org_admin, or a special holder of an issue).
CREATE TABLE IF NOT EXISTS project_permissions (
    project_id UUID NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    permission TEXT NOT NULL,
    holder     TEXT NOT NULL,
    PRIMARY KEY (project_id, permission, holder)
);
