import {
  effectiveProjectRole,
  hasProjectPermission,
  projectPermissions,
  ProjectPermission,
} from './project-permissions';

describe('project permission matrix', () => {
  it('grants each member role the permissions the backend defines', () => {
    const expected: Record<string, ProjectPermission[]> = {
      lead: [
        'view_project',
        'watch_issue',
        'comment',
        'create_issue',
        'edit_issue',
        'move_cards',
        'manage_sprint',
        'manage_board',
        'manage_workflow',
        'edit_project',
        'manage_members',
      ],
      contributor: [
        'view_project',
        'watch_issue',
        'comment',
        'create_issue',
        'edit_issue',
        'move_cards',
      ],
      viewer: ['view_project', 'watch_issue'],
    };
    for (const [role, permissions] of Object.entries(expected)) {
      const granted = projectPermissions
        .filter((item) => hasProjectPermission(role as 'lead', item.key))
        .map((item) => item.key);
      expect(granted.sort()).toEqual([...permissions].sort());
    }
  });

  it('lets only the global administrator delete the project', () => {
    expect(hasProjectPermission('admin', 'delete_project')).toBe(true);
    for (const role of ['lead', 'contributor', 'viewer', null] as const) {
      expect(hasProjectPermission(role, 'delete_project')).toBe(false);
    }
  });

  it('grants nothing without membership, except to the global administrator', () => {
    expect(hasProjectPermission(null, 'view_project')).toBe(false);
    expect(hasProjectPermission('admin', 'manage_members')).toBe(true);
  });

  it('computes the effective role from the global role and the membership', () => {
    const members = [{ userId: 'u1', role: 'contributor' as const }];
    expect(effectiveProjectRole('user', 'u1', members)).toBe('contributor');
    expect(effectiveProjectRole('user', 'u2', members)).toBeNull();
    expect(effectiveProjectRole('admin', 'u2', members)).toBe('admin');
    expect(effectiveProjectRole('user', undefined, members)).toBeNull();
  });
});
