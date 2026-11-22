-- Restaura el espejo users.team_id desde la primera membresia de cada usuario.
ALTER TABLE users ADD COLUMN IF NOT EXISTS team_id UUID REFERENCES teams (id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_users_team_id ON users (team_id);

UPDATE users u
SET team_id = first_membership.team_id
FROM (
    SELECT DISTINCT ON (user_id) user_id, team_id
    FROM team_members
    ORDER BY user_id, created_at ASC, team_id ASC
) AS first_membership
WHERE first_membership.user_id = u.id;
