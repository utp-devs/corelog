DROP TABLE IF EXISTS scheduler_runs;
DROP TABLE IF EXISTS scheduler_jobs;
DROP TABLE IF EXISTS notification_deliveries;
DROP TABLE IF EXISTS notification_digest_items;
DROP TABLE IF EXISTS email_outbox;
ALTER TABLE notification_preferences DROP CONSTRAINT IF EXISTS notification_preferences_email_mode_check;
ALTER TABLE notification_preferences DROP COLUMN IF EXISTS notify_own_actions, DROP COLUMN IF EXISTS email_mode;
DROP TABLE IF EXISTS notification_schemes;
