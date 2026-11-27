-- Boards v2: several boards per project, settings (swimlanes, estimation, card layout), column colors and
-- minimum limits, quick filters and query swimlanes. Existing boards keep working: their columns stay bound
-- to workflow statuses through board_column_statuses and the legacy status column becomes optional.
--
-- board_columns.wip_limit is the maximum number of cards of a column (max_issues in the API).

ALTER TABLE boards
    ADD COLUMN swimlane_mode       TEXT    NOT NULL DEFAULT 'none' CHECK (swimlane_mode IN ('none', 'assignee', 'epic', 'query', 'story')),
    ADD COLUMN estimation          TEXT    NOT NULL DEFAULT 'story_points' CHECK (estimation IN ('story_points', 'issue_count', 'time')),
    ADD COLUMN card_fields         JSONB   NOT NULL DEFAULT '["key","type","priority","assignee","labels","points"]'::jsonb
                                   CHECK (jsonb_typeof(card_fields) = 'array'),
    ADD COLUMN show_unmapped_lane  BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN done_retention_days INTEGER NULL CHECK (done_retention_days IS NULL OR done_retention_days >= 0);

-- Existing boards take the name of their project so several boards can be told apart.
UPDATE boards b
SET name = left(p.name, 80)
FROM projects p
WHERE p.id = b.project_id AND b.name = 'Default';

-- Board names are unique per project ignoring case. Legacy duplicates get a numeric suffix.
UPDATE boards b
SET name = left(b.name, 75) || ' ' || d.rn
FROM (
    SELECT id, row_number() OVER (PARTITION BY project_id, lower(name) ORDER BY created_at, id) AS rn
    FROM boards
) d
WHERE d.id = b.id AND d.rn > 1;
CREATE UNIQUE INDEX uq_boards_project_name ON boards (project_id, lower(name));

ALTER TABLE board_columns
    ADD COLUMN min_issues INTEGER NULL CHECK (min_issues IS NULL OR min_issues >= 0),
    ADD COLUMN color      TEXT    NOT NULL DEFAULT '' CHECK (color IN ('', 'gray', 'red', 'orange', 'amber', 'green', 'teal', 'blue', 'indigo', 'purple', 'pink')),
    ADD CONSTRAINT board_columns_wip_range CHECK (min_issues IS NULL OR wip_limit IS NULL OR min_issues <= wip_limit);

-- The legacy status is optional (columns of v2 boards are defined by their workflow statuses) and the
-- position unique constraint is deferred so a reorder can run in one statement.
ALTER TABLE board_columns ALTER COLUMN status DROP NOT NULL;
ALTER TABLE board_columns DROP CONSTRAINT IF EXISTS board_columns_board_id_position_key;
ALTER TABLE board_columns ADD CONSTRAINT board_columns_board_id_position_key
    UNIQUE (board_id, position) DEFERRABLE INITIALLY DEFERRED;

CREATE TABLE board_quick_filters (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    board_id   UUID NOT NULL REFERENCES boards (id) ON DELETE CASCADE,
    name       VARCHAR(60) NOT NULL CHECK (char_length(btrim(name)) > 0),
    query_text VARCHAR(500) NOT NULL CHECK (char_length(btrim(query_text)) > 0),
    position   INTEGER NOT NULL DEFAULT 0 CHECK (position >= 0),
    CONSTRAINT uq_board_quick_filters_position UNIQUE (board_id, position) DEFERRABLE INITIALLY DEFERRED
);

CREATE TABLE board_swimlanes (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    board_id   UUID NOT NULL REFERENCES boards (id) ON DELETE CASCADE,
    name       VARCHAR(60) NOT NULL CHECK (char_length(btrim(name)) > 0),
    query_text VARCHAR(500) NOT NULL CHECK (char_length(btrim(query_text)) > 0),
    position   INTEGER NOT NULL DEFAULT 0 CHECK (position >= 0),
    CONSTRAINT uq_board_swimlanes_position UNIQUE (board_id, position) DEFERRABLE INITIALLY DEFERRED
);
