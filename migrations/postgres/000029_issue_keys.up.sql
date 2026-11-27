-- Issues: requester_id becomes reporter_id, title becomes summary, and every issue gets a
-- per-project number (key_number) that forms its human key (PROJECT-n).

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns
               WHERE table_schema = current_schema() AND table_name = 'issues' AND column_name = 'requester_id') THEN
        ALTER TABLE issues RENAME COLUMN requester_id TO reporter_id;
    END IF;
    IF EXISTS (SELECT 1 FROM information_schema.columns
               WHERE table_schema = current_schema() AND table_name = 'issues' AND column_name = 'title') THEN
        ALTER TABLE issues RENAME COLUMN title TO summary;
    END IF;
END $$;

ALTER INDEX IF EXISTS idx_issues_requester_id RENAME TO idx_issues_reporter_id;

ALTER TABLE issues ADD COLUMN IF NOT EXISTS key_number INT;

-- Backfill: number the issues without one per project by creation time (id breaks ties), continuing
-- after the highest number already used by the project.
UPDATE issues i
SET key_number = n.base + n.rn
FROM (
    SELECT id,
           ROW_NUMBER() OVER (PARTITION BY project_id ORDER BY created_at, id) AS rn,
           COALESCE((SELECT MAX(x.key_number) FROM issues x WHERE x.project_id = issues.project_id), 0) AS base
    FROM issues
    WHERE key_number IS NULL
) n
WHERE i.id = n.id;

ALTER TABLE issues ALTER COLUMN key_number SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_issues_project_key_number ON issues (project_id, key_number);

-- The project counter continues after the highest number in use.
UPDATE projects p
SET next_issue_number = COALESCE((SELECT MAX(i.key_number) FROM issues i WHERE i.project_id = p.id), 0) + 1;
