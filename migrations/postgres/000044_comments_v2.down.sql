-- Reverts comments v2. Internal comments are removed first: once the visibility column is gone they would
-- become public, and their replies (always internal too) would be published with them.
DELETE FROM comments WHERE visibility = 'internal';

DROP TABLE IF EXISTS comment_notices;
DROP TABLE IF EXISTS comment_reactions;
DROP TABLE IF EXISTS comment_revisions;

ALTER TABLE comment_mentions DROP COLUMN IF EXISTS created_at;

DROP INDEX IF EXISTS idx_comments_parent_id;
DROP INDEX IF EXISTS idx_comments_issue_parent;

ALTER TABLE comments
    DROP CONSTRAINT IF EXISTS comments_no_self_parent_check,
    DROP CONSTRAINT IF EXISTS comments_internal_role_check,
    DROP CONSTRAINT IF EXISTS comments_visibility_check;

ALTER TABLE comments
    DROP COLUMN IF EXISTS edited_at,
    DROP COLUMN IF EXISTS internal_role,
    DROP COLUMN IF EXISTS visibility,
    DROP COLUMN IF EXISTS parent_id;
