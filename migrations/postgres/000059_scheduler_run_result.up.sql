-- A job can leave a short summary of what it did (for example the rows a purge removed) next to its run.
ALTER TABLE scheduler_runs ADD COLUMN IF NOT EXISTS result TEXT NOT NULL DEFAULT '';
