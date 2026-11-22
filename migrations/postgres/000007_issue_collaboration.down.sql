DROP INDEX IF EXISTS idx_activities_actor_id;
DROP INDEX IF EXISTS idx_activities_issue_id;
DROP INDEX IF EXISTS idx_comment_mentions_user_id;
DROP INDEX IF EXISTS idx_comments_author_id;
DROP INDEX IF EXISTS idx_comments_issue_id;

DROP TABLE IF EXISTS activities;
DROP TABLE IF EXISTS comment_mentions;
DROP TABLE IF EXISTS comments;
