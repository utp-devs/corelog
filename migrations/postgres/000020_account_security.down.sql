DROP INDEX IF EXISTS idx_user_sessions_user;
DROP TABLE IF EXISTS user_sessions;
ALTER TABLE users DROP COLUMN IF EXISTS deactivated_at;
