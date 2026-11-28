-- Calendar events with recurrence, attendees and a private ICS feed token.
-- recurrence_rule holds an RRULE subset (FREQ, INTERVAL, BYDAY, COUNT, UNTIL). recurrence_exceptions is a JSON
-- array of {"original_start": RFC 3339 UTC, "cancelled": bool, "override": {...}}. All-day events store UTC
-- midnights and an exclusive end.

CREATE TABLE IF NOT EXISTS calendar_events (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id              UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    organization_id       UUID NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    project_id            UUID NULL REFERENCES projects (id) ON DELETE CASCADE,
    title                 VARCHAR(200) NOT NULL CHECK (char_length(btrim(title)) > 0),
    description           TEXT NOT NULL DEFAULT '' CHECK (char_length(description) <= 8000),
    location              VARCHAR(300) NOT NULL DEFAULT '',
    starts_at             TIMESTAMPTZ NOT NULL,
    ends_at               TIMESTAMPTZ NOT NULL,
    all_day               BOOLEAN NOT NULL DEFAULT FALSE,
    timezone              VARCHAR(64) NOT NULL DEFAULT 'UTC',
    recurrence_rule       TEXT NULL CHECK (recurrence_rule IS NULL OR char_length(recurrence_rule) <= 400),
    recurrence_exceptions JSONB NOT NULL DEFAULT '[]' CHECK (jsonb_typeof(recurrence_exceptions) = 'array'),
    visibility            TEXT NOT NULL DEFAULT 'private' CHECK (visibility IN ('private', 'project', 'organization')),
    color                 TEXT NOT NULL DEFAULT 'blue' CHECK (color IN (
                              'gray', 'red', 'orange', 'amber', 'green', 'teal', 'blue', 'indigo', 'purple', 'pink')),
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at            TIMESTAMPTZ NULL,
    CONSTRAINT calendar_events_order_check CHECK (ends_at >= starts_at),
    CONSTRAINT calendar_events_project_visibility_check CHECK (visibility <> 'project' OR project_id IS NOT NULL)
);

CREATE INDEX IF NOT EXISTS idx_calendar_events_org_start ON calendar_events (organization_id, starts_at) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_calendar_events_owner ON calendar_events (owner_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_calendar_events_project ON calendar_events (project_id) WHERE deleted_at IS NULL AND project_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS calendar_event_attendees (
    event_id   UUID NOT NULL REFERENCES calendar_events (id) ON DELETE CASCADE,
    user_id    UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    response   TEXT NOT NULL DEFAULT 'needs_action' CHECK (response IN ('needs_action', 'accepted', 'declined', 'tentative')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (event_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_calendar_event_attendees_user ON calendar_event_attendees (user_id);

-- One private feed token per user. Only the SHA-256 hash is stored; revoked_at disables the feed.
CREATE TABLE IF NOT EXISTS calendar_feed_tokens (
    user_id    UUID PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at TIMESTAMPTZ NULL
);
