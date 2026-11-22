-- ADR-002: la membresia de equipos vive en team_members. Antes de quitar el espejo, se copian las
-- membresias que pudieran faltar (usuarios con team_id y sin fila en team_members).
INSERT INTO team_members (team_id, user_id, role, created_at)
SELECT u.team_id, u.id, 'member', u.created_at
FROM users u
WHERE u.team_id IS NOT NULL
ON CONFLICT (team_id, user_id) DO NOTHING;

ALTER TABLE users DROP COLUMN IF EXISTS team_id;
