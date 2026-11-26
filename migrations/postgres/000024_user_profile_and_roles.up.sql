-- User profile columns and platform role migration.
-- Also creates the default organization and its memberships. Memberships must be
-- derived from the ORIGINAL users.role (requester -> guest), so they are inserted
-- here, before the role values are rewritten. Teams and projects are scoped to the
-- default organization in 000025.

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS display_name      VARCHAR(120),
    ADD COLUMN IF NOT EXISTS job_title         VARCHAR(120),
    ADD COLUMN IF NOT EXISTS department        VARCHAR(120),
    ADD COLUMN IF NOT EXISTS avatar_key        VARCHAR(255),
    ADD COLUMN IF NOT EXISTS timezone          VARCHAR(64) NOT NULL DEFAULT 'UTC',
    ADD COLUMN IF NOT EXISTS last_login_at     TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS email_verified_at TIMESTAMPTZ;

UPDATE users SET display_name = name WHERE display_name IS NULL;
ALTER TABLE users ALTER COLUMN display_name SET NOT NULL;

-- Default organization. The fixed id lets the down migration and the scoping
-- migration target it without a lookup by slug.
INSERT INTO organizations (id, slug, name, description, created_by)
VALUES (
    '00000000-0000-4000-8000-000000000002',
    'corelog',
    'CoreLog',
    'Organizacion por defecto creada durante la migracion',
    (SELECT id FROM users WHERE role = 'admin' ORDER BY created_at ASC LIMIT 1)
)
ON CONFLICT (id) DO NOTHING;

-- Memberships from the original role: admin -> owner, requester -> guest, others -> member.
INSERT INTO organization_members (organization_id, user_id, role, joined_at)
SELECT
    '00000000-0000-4000-8000-000000000002',
    u.id,
    CASE u.role
        WHEN 'admin' THEN 'owner'
        WHEN 'requester' THEN 'guest'
        ELSE 'member'
    END,
    u.created_at
FROM users u
ON CONFLICT (organization_id, user_id) DO NOTHING;

-- Role migration: the platform role is now admin|user. Legacy agent and requester
-- become user; admin stays admin. The old CHECK must go before rewriting values.
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
UPDATE users SET role = 'user' WHERE role IN ('agent', 'requester');
ALTER TABLE users ADD CONSTRAINT users_role_check CHECK (role IN ('admin', 'user'));
