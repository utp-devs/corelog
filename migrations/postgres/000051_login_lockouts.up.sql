-- Estado persistente del bloqueo de inicio de sesion por cuenta. La clave es el hash SHA-256 del correo
-- normalizado, de modo que tambien protege cuentas inexistentes sin revelar cual existe.
CREATE TABLE IF NOT EXISTS account_lockouts (
    key_hash TEXT PRIMARY KEY,
    failures INTEGER NOT NULL DEFAULT 0 CHECK (failures >= 0),
    lock_count INTEGER NOT NULL DEFAULT 0 CHECK (lock_count >= 0),
    first_failure_at TIMESTAMPTZ NULL,
    locked_until TIMESTAMPTZ NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_account_lockouts_updated ON account_lockouts (updated_at);
