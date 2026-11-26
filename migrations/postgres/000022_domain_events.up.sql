-- Bandeja de eventos de dominio (outbox) y cursores por consumidor.
CREATE TABLE IF NOT EXISTS domain_events (
    id BIGSERIAL PRIMARY KEY,
    org_id UUID NULL,
    project_id UUID NULL,
    type TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id UUID NULL,
    actor_id UUID NULL,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_domain_events_type_id ON domain_events (type, id);

CREATE TABLE IF NOT EXISTS event_cursors (
    consumer TEXT PRIMARY KEY,
    last_event_id BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
