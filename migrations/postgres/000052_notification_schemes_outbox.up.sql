-- Esquemas de notificacion por proyecto y por organizacion, preferencias ampliadas, cola de correo (outbox),
-- digest diario, deduplicacion de entregas y trabajos programados.

-- Un esquema por proyecto (project_id) o el esquema por defecto de una organizacion (project_id NULL).
CREATE TABLE IF NOT EXISTS notification_schemes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    project_id UUID NULL REFERENCES projects(id) ON DELETE CASCADE,
    rules JSONB NOT NULL DEFAULT '[]'::jsonb,
    updated_by UUID NULL REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_notification_schemes_project ON notification_schemes (project_id) WHERE project_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_notification_schemes_org_default ON notification_schemes (organization_id) WHERE project_id IS NULL;

ALTER TABLE notification_preferences
    ADD COLUMN IF NOT EXISTS email_mode TEXT NOT NULL DEFAULT 'immediate',
    ADD COLUMN IF NOT EXISTS notify_own_actions BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE notification_preferences DROP CONSTRAINT IF EXISTS notification_preferences_email_mode_check;
ALTER TABLE notification_preferences
    ADD CONSTRAINT notification_preferences_email_mode_check CHECK (email_mode IN ('immediate', 'daily', 'off'));

-- Cola de correo. next_attempt_at sirve tambien de arrendamiento mientras un trabajador envia la fila.
CREATE TABLE IF NOT EXISTS email_outbox (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NULL REFERENCES users(id) ON DELETE SET NULL,
    to_email VARCHAR(255) NOT NULL,
    kind VARCHAR(40) NOT NULL,
    subject VARCHAR(300) NOT NULL,
    body_text TEXT NOT NULL,
    body_html TEXT NOT NULL DEFAULT '',
    status VARCHAR(10) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'sent', 'dead')),
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_error TEXT NOT NULL DEFAULT '',
    idempotency_key VARCHAR(200) NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    sent_at TIMESTAMPTZ NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_email_outbox_idempotency ON email_outbox (idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_email_outbox_due ON email_outbox (next_attempt_at) WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_email_outbox_status ON email_outbox (status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_email_outbox_user ON email_outbox (user_id);

-- Avisos de usuarios con correo en modo diario, pendientes de agrupar en un resumen.
CREATE TABLE IF NOT EXISTS notification_digest_items (
    id BIGSERIAL PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    event VARCHAR(40) NOT NULL,
    issue_key VARCHAR(40) NOT NULL DEFAULT '',
    summary VARCHAR(300) NOT NULL DEFAULT '',
    actor_name VARCHAR(120) NOT NULL DEFAULT '',
    link VARCHAR(300) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    digested_at TIMESTAMPTZ NULL
);
CREATE INDEX IF NOT EXISTS idx_digest_items_pending ON notification_digest_items (user_id, id) WHERE digested_at IS NULL;

-- Una entrega por (clave, usuario): un evento reentregado no duplica avisos.
CREATE TABLE IF NOT EXISTS notification_deliveries (
    dedup_key VARCHAR(120) NOT NULL,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (dedup_key, user_id)
);
CREATE INDEX IF NOT EXISTS idx_notification_deliveries_created ON notification_deliveries (created_at);

-- Trabajos programados registrados y su historial de ejecuciones.
CREATE TABLE IF NOT EXISTS scheduler_jobs (
    name VARCHAR(80) PRIMARY KEY,
    schedule VARCHAR(120) NOT NULL,
    registered_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS scheduler_runs (
    id BIGSERIAL PRIMARY KEY,
    job VARCHAR(80) NOT NULL,
    started_at TIMESTAMPTZ NOT NULL,
    finished_at TIMESTAMPTZ NULL,
    status VARCHAR(12) NOT NULL DEFAULT 'running' CHECK (status IN ('running', 'ok', 'error', 'abandoned')),
    error TEXT NOT NULL DEFAULT '',
    duration_ms BIGINT NULL
);
CREATE INDEX IF NOT EXISTS idx_scheduler_runs_job ON scheduler_runs (job, id DESC);
CREATE INDEX IF NOT EXISTS idx_scheduler_runs_started ON scheduler_runs (started_at);
