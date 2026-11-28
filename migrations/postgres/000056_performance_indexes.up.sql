-- Performance indexes found by the query plans of the hottest read paths on a 20,000 issue dataset.
-- Every index is partial where the queries it serves always filter soft deleted rows, which keeps it small.

-- Calendar window: issues whose start date falls inside the visible range. Without it the planner reads the
-- whole issues table to find the few rows of a month. The due date window is served by idx_issues_due_date.
CREATE INDEX IF NOT EXISTS idx_issues_start_date_live ON issues (start_date)
    WHERE deleted_at IS NULL AND start_date IS NOT NULL;

-- Global search matches the typed text as a substring of the description (ILIKE '%text%'). A trigram index lets
-- the planner combine it with the full text and summary indexes instead of reading every description. Like the
-- summary trigram index it is only created when pg_trgm is available.
DO $$
DECLARE
    ext_schema text;
BEGIN
    SELECT n.nspname INTO ext_schema
    FROM pg_extension e JOIN pg_namespace n ON n.oid = e.extnamespace
    WHERE e.extname = 'pg_trgm';
    IF ext_schema IS NOT NULL THEN
        EXECUTE format('CREATE INDEX IF NOT EXISTS idx_issues_description_trgm ON issues USING GIN (description %I.gin_trgm_ops) WHERE deleted_at IS NULL', ext_schema);
    END IF;
END
$$;
