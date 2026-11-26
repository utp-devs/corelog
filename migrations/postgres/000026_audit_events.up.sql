-- Registro de auditoria inmutable. Las filas solo se insertan. UPDATE y TRUNCATE
-- siempre se rechazan; DELETE solo se permite dentro de la transaccion del job de
-- retencion (set_config('corelog.audit_retention', 'on', true)).
CREATE TABLE IF NOT EXISTS audit_events (
    id BIGSERIAL PRIMARY KEY,
    organization_id UUID NULL,
    project_id UUID NULL,
    actor_id UUID NULL,
    action TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id UUID NULL,
    summary TEXT NOT NULL,
    before JSONB NULL,
    after JSONB NULL,
    ip INET NULL,
    user_agent TEXT NULL,
    source_event_id BIGINT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_audit_events_org_id ON audit_events (organization_id, id DESC);
CREATE INDEX IF NOT EXISTS idx_audit_events_org_action ON audit_events (organization_id, action);
CREATE INDEX IF NOT EXISTS idx_audit_events_actor ON audit_events (actor_id);
CREATE INDEX IF NOT EXISTS idx_audit_events_created_at ON audit_events (created_at);

CREATE OR REPLACE FUNCTION audit_events_block_mutation() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' AND coalesce(current_setting('corelog.audit_retention', true), '') = 'on' THEN
        RETURN OLD;
    END IF;
    RAISE EXCEPTION 'audit_events is append-only';
END;
$$;

CREATE OR REPLACE TRIGGER trg_audit_events_block_update_delete
    BEFORE UPDATE OR DELETE ON audit_events
    FOR EACH ROW EXECUTE FUNCTION audit_events_block_mutation();

CREATE OR REPLACE TRIGGER trg_audit_events_block_truncate
    BEFORE TRUNCATE ON audit_events
    FOR EACH STATEMENT EXECUTE FUNCTION audit_events_block_mutation();
