DROP INDEX IF EXISTS uq_issues_project_key_number;
ALTER TABLE issues DROP COLUMN IF EXISTS key_number;

ALTER INDEX IF EXISTS idx_issues_reporter_id RENAME TO idx_issues_requester_id;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns
               WHERE table_schema = current_schema() AND table_name = 'issues' AND column_name = 'summary') THEN
        ALTER TABLE issues RENAME COLUMN summary TO title;
    END IF;
    IF EXISTS (SELECT 1 FROM information_schema.columns
               WHERE table_schema = current_schema() AND table_name = 'issues' AND column_name = 'reporter_id') THEN
        ALTER TABLE issues RENAME COLUMN reporter_id TO requester_id;
    END IF;
END $$;
