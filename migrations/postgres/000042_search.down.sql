DROP TABLE IF EXISTS filter_subscriptions;
DROP TABLE IF EXISTS filter_shares;
DROP TABLE IF EXISTS filters;
DROP INDEX IF EXISTS idx_projects_key_trgm;
DROP INDEX IF EXISTS idx_projects_name_trgm;
DROP INDEX IF EXISTS idx_issues_summary_trgm;
DROP INDEX IF EXISTS idx_comments_fts;
DROP INDEX IF EXISTS idx_issues_search_vector;
ALTER TABLE issues DROP COLUMN IF EXISTS search_vector;
