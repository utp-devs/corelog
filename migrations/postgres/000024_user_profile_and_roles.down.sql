-- LOSSY: 000024 cannot tell agent from requester after the role rewrite, so every
-- 'user' comes back as 'agent'. The original requester role is not restored here;
-- the organization memberships of the default organization are removed instead.
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
UPDATE users SET role = 'agent' WHERE role = 'user';
ALTER TABLE users ADD CONSTRAINT users_role_check CHECK (role IN ('admin', 'agent', 'requester'));

DELETE FROM organization_members WHERE organization_id = '00000000-0000-4000-8000-000000000002';
DELETE FROM organizations WHERE id = '00000000-0000-4000-8000-000000000002';

ALTER TABLE users
    DROP COLUMN IF EXISTS email_verified_at,
    DROP COLUMN IF EXISTS last_login_at,
    DROP COLUMN IF EXISTS timezone,
    DROP COLUMN IF EXISTS avatar_key,
    DROP COLUMN IF EXISTS department,
    DROP COLUMN IF EXISTS job_title,
    DROP COLUMN IF EXISTS display_name;
