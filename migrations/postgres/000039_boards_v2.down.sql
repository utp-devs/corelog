-- Reverts boards v2. Boards that are not the default of their project and the columns that have no legacy
-- status are removed because the legacy shape cannot represent them.

DROP TABLE IF EXISTS board_swimlanes;
DROP TABLE IF EXISTS board_quick_filters;

DELETE FROM boards WHERE NOT is_default;
DELETE FROM board_columns WHERE status IS NULL;

ALTER TABLE board_columns DROP CONSTRAINT IF EXISTS board_columns_board_id_position_key;
ALTER TABLE board_columns ADD CONSTRAINT board_columns_board_id_position_key UNIQUE (board_id, position);
ALTER TABLE board_columns ALTER COLUMN status SET NOT NULL;
ALTER TABLE board_columns
    DROP CONSTRAINT IF EXISTS board_columns_wip_range,
    DROP COLUMN IF EXISTS color,
    DROP COLUMN IF EXISTS min_issues;

DROP INDEX IF EXISTS uq_boards_project_name;
ALTER TABLE boards
    DROP COLUMN IF EXISTS done_retention_days,
    DROP COLUMN IF EXISTS show_unmapped_lane,
    DROP COLUMN IF EXISTS card_fields,
    DROP COLUMN IF EXISTS estimation,
    DROP COLUMN IF EXISTS swimlane_mode;
