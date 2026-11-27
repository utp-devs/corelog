import {
  assignableRoles,
  canEditOrgSettings,
  canManageOrg,
  canRemoveMember,
  canUseOwnerDangerZone,
  canViewMembers,
  invitableRoles,
  validateLogoFile,
} from './org-permissions';

describe('validateLogoFile', () => {
  it('accepts PNG, JPEG and WebP up to 2 MB', () => {
    expect(validateLogoFile({ type: 'image/png', size: 1000 })).toBeNull();
    expect(validateLogoFile({ type: 'image/jpeg', size: 2 * 1024 * 1024 })).toBeNull();
    expect(validateLogoFile({ type: 'image/webp', size: 10 })).toBeNull();
  });

  it('rejects other formats and oversized files', () => {
    expect(validateLogoFile({ type: 'image/gif', size: 10 })).toMatch(/Formato/);
    expect(validateLogoFile({ type: 'image/png', size: 2 * 1024 * 1024 + 1 })).toMatch(/2 MB/);
  });
});

describe('organization permissions', () => {
  it('lets only owners and admins manage the organization', () => {
    expect(canManageOrg('owner')).toBe(true);
    expect(canManageOrg('admin')).toBe(true);
    expect(canManageOrg('member')).toBe(false);
    expect(canManageOrg('guest')).toBe(false);
    expect(canManageOrg(undefined)).toBe(false);
    expect(canManageOrg(null)).toBe(false);
  });

  it('gives settings rights to the same roles as management', () => {
    expect(canEditOrgSettings('admin')).toBe(true);
    expect(canEditOrgSettings('member')).toBe(false);
  });

  it('lets owners assign any role to others and nobody to self', () => {
    expect(assignableRoles('owner', 'member', false)).toEqual([
      'owner',
      'admin',
      'member',
      'guest',
    ]);
    expect(assignableRoles('owner', 'owner', true)).toEqual([]);
  });

  it('lets admins switch only members and guests', () => {
    expect(assignableRoles('admin', 'member', false)).toEqual(['member', 'guest']);
    expect(assignableRoles('admin', 'guest', false)).toEqual(['member', 'guest']);
    expect(assignableRoles('admin', 'admin', false)).toEqual([]);
    expect(assignableRoles('member', 'guest', false)).toEqual([]);
  });

  it('restricts removal the same way', () => {
    expect(canRemoveMember('owner', 'owner', false)).toBe(true);
    expect(canRemoveMember('admin', 'admin', false)).toBe(false);
    expect(canRemoveMember('admin', 'guest', false)).toBe(true);
    expect(canRemoveMember('admin', 'member', true)).toBe(false);
  });

  it('gives invitation and danger-zone rights by role', () => {
    expect(invitableRoles('owner')).toEqual(['admin', 'member', 'guest']);
    expect(invitableRoles('admin')).toEqual(['member', 'guest']);
    expect(invitableRoles('member')).toEqual([]);
    expect(canUseOwnerDangerZone('admin')).toBe(false);
    expect(canUseOwnerDangerZone('owner')).toBe(true);
    expect(canViewMembers('guest')).toBe(false);
    expect(canViewMembers('member')).toBe(true);
  });
});
