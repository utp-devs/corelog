-- Personal notes. A note is private to its owner: no other user, administrators included, can read it.
-- Links to an issue or a project are optional and are cleared when the linked row is hard deleted.
CREATE TABLE IF NOT EXISTS notes (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id    UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    title       TEXT NOT NULL DEFAULT '' CHECK (char_length(title) <= 120),
    body        TEXT NOT NULL DEFAULT '' CHECK (char_length(body) <= 20000),
    color       TEXT NOT NULL DEFAULT 'yellow'
                CHECK (color IN ('yellow', 'blue', 'green', 'pink', 'purple', 'orange', 'red', 'gray')),
    pinned      BOOLEAN NOT NULL DEFAULT FALSE,
    tags        TEXT[] NOT NULL DEFAULT '{}' CHECK (cardinality(tags) <= 10),
    issue_id    UUID NULL REFERENCES issues (id) ON DELETE SET NULL,
    project_id  UUID NULL REFERENCES projects (id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ NULL
);

CREATE INDEX IF NOT EXISTS idx_notes_owner_order
    ON notes (owner_id, pinned DESC, updated_at DESC, id DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_notes_tags ON notes USING GIN (tags);
CREATE INDEX IF NOT EXISTS idx_notes_issue_id ON notes (issue_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_notes_project_id ON notes (project_id) WHERE deleted_at IS NULL;
