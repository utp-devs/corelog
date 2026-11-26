DROP INDEX IF EXISTS idx_organization_invitations_invited_by;
DROP INDEX IF EXISTS uq_organization_invitations_pending;
DROP TABLE IF EXISTS organization_invitations;

DROP INDEX IF EXISTS idx_organization_members_user_id;
DROP TABLE IF EXISTS organization_members;

DROP INDEX IF EXISTS idx_organizations_created_by;
DROP INDEX IF EXISTS uq_organizations_slug;
DROP TABLE IF EXISTS organizations;
