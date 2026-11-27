-- Sprints v2: lifecycle timestamps, committed/completed points, optional board and the scope history
-- (sprint_issue_events) that the sprint report and the burndown are computed from.

ALTER TABLE sprints
    ADD COLUMN IF NOT EXISTS goal             VARCHAR(500) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS started_at       TIMESTAMPTZ  NULL,
    ADD COLUMN IF NOT EXISTS completed_at     TIMESTAMPTZ  NULL,
    ADD COLUMN IF NOT EXISTS committed_points NUMERIC(7,1) NULL CHECK (committed_points IS NULL OR committed_points >= 0),
    ADD COLUMN IF NOT EXISTS completed_points NUMERIC(7,1) NULL CHECK (completed_points IS NULL OR completed_points >= 0),
    ADD COLUMN IF NOT EXISTS board_id         UUID         NULL REFERENCES boards (id) ON DELETE SET NULL;

-- A board also has at most one active sprint (the project-wide rule already exists since the first sprints migration).
CREATE UNIQUE INDEX IF NOT EXISTS idx_sprints_one_active_per_board
    ON sprints (board_id) WHERE status = 'active' AND board_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_sprints_board_id ON sprints (board_id) WHERE board_id IS NOT NULL;

-- Scope history. 'added' rows written when the sprint starts form the committed snapshot (after_start = false).
-- 'points_changed' records a story point edit made while the sprint is active.
CREATE TABLE IF NOT EXISTS sprint_issue_events (
    id                      BIGSERIAL PRIMARY KEY,
    sprint_id               UUID         NOT NULL REFERENCES sprints (id) ON DELETE CASCADE,
    issue_id                UUID         NOT NULL REFERENCES issues (id) ON DELETE CASCADE,
    event                   TEXT         NOT NULL CHECK (event IN ('added', 'removed', 'completed', 'incomplete', 'moved_out', 'points_changed')),
    points_at_time          NUMERIC(5,1) NULL,
    status_category_at_time TEXT         NULL CHECK (status_category_at_time IS NULL OR status_category_at_time IN ('todo', 'in_progress', 'done')),
    happened_at             TIMESTAMPTZ  NOT NULL DEFAULT now(),
    actor_id                UUID         NULL REFERENCES users (id) ON DELETE SET NULL,
    after_start             BOOLEAN      NOT NULL DEFAULT FALSE
);
CREATE INDEX IF NOT EXISTS idx_sprint_issue_events_sprint ON sprint_issue_events (sprint_id, happened_at, id);
CREATE INDEX IF NOT EXISTS idx_sprint_issue_events_issue ON sprint_issue_events (issue_id);
CREATE INDEX IF NOT EXISTS idx_sprint_issue_events_actor ON sprint_issue_events (actor_id) WHERE actor_id IS NOT NULL;

-- corelog_issue_category returns the status category of an issue row: the workflow status when it has one,
-- otherwise the category implied by the legacy status column.
CREATE OR REPLACE FUNCTION corelog_issue_category(p_status_id uuid, p_legacy text) RETURNS text
LANGUAGE sql STABLE AS $$
    SELECT COALESCE(
        (SELECT s.category FROM statuses s WHERE s.id = p_status_id),
        CASE p_legacy WHEN 'closed' THEN 'done' WHEN 'resolved' THEN 'done' WHEN 'in_progress' THEN 'in_progress' ELSE 'todo' END)
$$;

-- Backfill (best effort): sprints that already started get lifecycle timestamps, the committed snapshot of the
-- issues they hold today and, when closed, completed/incomplete events for those issues.
UPDATE sprints
SET started_at   = COALESCE(start_date::timestamptz, created_at),
    completed_at = closed_at
WHERE status IN ('active', 'closed') AND started_at IS NULL;

INSERT INTO sprint_issue_events (sprint_id, issue_id, event, points_at_time, status_category_at_time, happened_at, after_start)
SELECT s.id, i.id, 'added', i.story_points, 'todo', s.started_at, FALSE
FROM sprints s
JOIN issues i ON i.sprint_id = s.id AND i.deleted_at IS NULL
WHERE s.status IN ('active', 'closed')
  AND NOT EXISTS (SELECT 1 FROM sprint_issue_events e WHERE e.sprint_id = s.id);

INSERT INTO sprint_issue_events (sprint_id, issue_id, event, points_at_time, status_category_at_time, happened_at, after_start)
SELECT s.id, i.id,
       CASE WHEN corelog_issue_category(i.status_id, i.status) = 'done' THEN 'completed' ELSE 'incomplete' END,
       i.story_points, corelog_issue_category(i.status_id, i.status), COALESCE(s.closed_at, s.updated_at), TRUE
FROM sprints s
JOIN issues i ON i.sprint_id = s.id AND i.deleted_at IS NULL
WHERE s.status = 'closed';

UPDATE sprints s
SET committed_points = COALESCE((SELECT SUM(e.points_at_time) FROM sprint_issue_events e
                                 WHERE e.sprint_id = s.id AND e.event = 'added' AND NOT e.after_start), 0)
WHERE s.status IN ('active', 'closed') AND s.committed_points IS NULL;

UPDATE sprints s
SET completed_points = COALESCE((SELECT SUM(e.points_at_time) FROM sprint_issue_events e
                                 WHERE e.sprint_id = s.id AND e.event = 'completed'), 0)
WHERE s.status = 'closed' AND s.completed_points IS NULL;

-- Story point edits made while the issue sits in an active sprint are part of the scope history.
CREATE OR REPLACE FUNCTION corelog_sprint_points_event() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.sprint_id IS NOT NULL
       AND EXISTS (SELECT 1 FROM sprints s WHERE s.id = NEW.sprint_id AND s.status = 'active') THEN
        INSERT INTO sprint_issue_events (sprint_id, issue_id, event, points_at_time, status_category_at_time, after_start)
        VALUES (NEW.sprint_id, NEW.id, 'points_changed', NEW.story_points,
                corelog_issue_category(NEW.status_id, NEW.status), TRUE);
    END IF;
    RETURN NEW;
END
$$;

DROP TRIGGER IF EXISTS trg_issues_sprint_points_event ON issues;
CREATE TRIGGER trg_issues_sprint_points_event
    AFTER UPDATE OF story_points ON issues
    FOR EACH ROW
    WHEN (OLD.story_points IS DISTINCT FROM NEW.story_points)
    EXECUTE FUNCTION corelog_sprint_points_event();
