-- Identidad de usuario v2: nombre y apellido separados, telefono con pais (para SMS) e idioma.
-- users.name se conserva como nombre completo y lo mantiene un trigger, de modo que los modulos que
-- leen users.name siguen funcionando. users.timezone ya existe desde 000024.

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS first_name        VARCHAR(80)  NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS last_name         VARCHAR(120) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS phone_country     CHAR(2)      NULL,
    ADD COLUMN IF NOT EXISTS phone_number      VARCHAR(20)  NULL,
    ADD COLUMN IF NOT EXISTS phone_e164        VARCHAR(16)  NULL,
    ADD COLUMN IF NOT EXISTS phone_verified_at TIMESTAMPTZ  NULL,
    ADD COLUMN IF NOT EXISTS locale            VARCHAR(10)  NOT NULL DEFAULT 'es';

ALTER TABLE users
    ADD CONSTRAINT users_phone_country_check CHECK (phone_country IS NULL OR phone_country ~ '^[A-Z]{2}$'),
    ADD CONSTRAINT users_phone_e164_check CHECK (phone_e164 IS NULL OR phone_e164 ~ '^\+[1-9][0-9]{6,14}$'),
    ADD CONSTRAINT users_phone_all_or_none_check CHECK (
        (phone_country IS NULL AND phone_number IS NULL AND phone_e164 IS NULL AND phone_verified_at IS NULL)
        OR (phone_country IS NOT NULL AND phone_number IS NOT NULL AND phone_e164 IS NOT NULL)
    ),
    ADD CONSTRAINT users_locale_check CHECK (locale ~ '^[a-z]{2}(-[A-Z]{2})?$');

-- A number is unique only among VERIFIED numbers: an unverified number never blocks another user, and the
-- owner of a number claims it by verifying it.
CREATE UNIQUE INDEX IF NOT EXISTS uq_users_phone_e164_verified
    ON users (phone_e164) WHERE phone_verified_at IS NOT NULL;

-- Splits a full name into first and last name. One word is a first name; two words are first and last;
-- three words keep the first word as first name (the rest are surnames); four or more keep two words as
-- first name. The Go helper domain.SplitName implements the same rule.
CREATE OR REPLACE FUNCTION users_split_name(full_name TEXT, OUT first_part TEXT, OUT last_part TEXT)
LANGUAGE plpgsql IMMUTABLE AS $$
DECLARE
    parts TEXT[];
    n     INT;
    k     INT;
BEGIN
    parts := regexp_split_to_array(btrim(full_name), '\s+');
    IF parts IS NULL OR btrim(full_name) = '' THEN
        first_part := '';
        last_part := '';
        RETURN;
    END IF;
    n := array_length(parts, 1);
    IF n = 1 THEN
        k := 1;
    ELSIF n <= 3 THEN
        k := 1;
    ELSE
        k := 2;
    END IF;
    first_part := left(array_to_string(parts[1:k], ' '), 80);
    last_part := left(array_to_string(parts[k + 1:n], ' '), 120);
END;
$$;

-- Keeps name, first_name and last_name consistent for every writer:
--   * insert without first/last names (legacy writers): they are derived from name;
--   * insert or update with first/last names: name becomes their concatenation;
--   * update of name alone: first/last names are derived from the new name.
CREATE OR REPLACE FUNCTION users_sync_name()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE
    split RECORD;
BEGIN
    IF TG_OP = 'INSERT' THEN
        IF NEW.first_name = '' AND NEW.last_name = '' THEN
            IF btrim(NEW.name) <> '' THEN
                SELECT * INTO split FROM users_split_name(NEW.name);
                NEW.first_name := split.first_part;
                NEW.last_name := split.last_part;
            END IF;
        ELSE
            NEW.name := left(btrim(NEW.first_name || ' ' || NEW.last_name), 120);
        END IF;
    ELSIF (NEW.first_name, NEW.last_name) IS DISTINCT FROM (OLD.first_name, OLD.last_name)
          AND (NEW.first_name <> '' OR NEW.last_name <> '') THEN
        NEW.name := left(btrim(NEW.first_name || ' ' || NEW.last_name), 120);
    ELSIF NEW.name IS DISTINCT FROM OLD.name AND btrim(NEW.name) <> '' THEN
        SELECT * INTO split FROM users_split_name(NEW.name);
        NEW.first_name := split.first_part;
        NEW.last_name := split.last_part;
    END IF;
    RETURN NEW;
END;
$$;

-- Backfill from the existing name; name itself is not touched.
UPDATE users u
SET first_name = s.first_part,
    last_name = s.last_part
FROM (SELECT id, (users_split_name(name)).* FROM users) s
WHERE s.id = u.id AND u.first_name = '' AND u.last_name = '';

DROP TRIGGER IF EXISTS trg_users_sync_name ON users;
CREATE TRIGGER trg_users_sync_name
    BEFORE INSERT OR UPDATE OF name, first_name, last_name ON users
    FOR EACH ROW EXECUTE FUNCTION users_sync_name();

-- Phone verification codes (SMS). Only the SHA-256 of the code is stored.
CREATE TABLE IF NOT EXISTS user_phone_verifications (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    phone_e164  VARCHAR(16) NOT NULL CHECK (phone_e164 ~ '^\+[1-9][0-9]{6,14}$'),
    code_hash   BYTEA       NOT NULL CHECK (octet_length(code_hash) = 32),
    attempts    INT         NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    expires_at  TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_user_phone_verifications_user
    ON user_phone_verifications (user_id, created_at DESC);
