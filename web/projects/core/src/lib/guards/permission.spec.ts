import { describe, expect, it } from 'vitest';

import { hasPermission, permissionsFor, type Permission } from './permission';

describe('permissionsFor', () => {
  it('grants org and project administration to platform admins', () => {
    const granted = permissionsFor({ role: 'admin' });
    expect(granted.has('org.admin')).toBe(true);
    expect(granted.has('project.permissions')).toBe(true);
    expect(permissionsFor({ role: 'user' }).size).toBe(0);
    expect(permissionsFor(null).size).toBe(0);
  });

  it('grants org administration to owners and admins of the active organization', () => {
    expect(permissionsFor({ role: 'user' }, 'owner').has('org.admin')).toBe(true);
    expect(permissionsFor({ role: 'user' }, 'admin').has('org.admin')).toBe(true);
  });

  it('withholds org administration from members and guests and never grants project permissions from the org role', () => {
    expect(permissionsFor({ role: 'user' }, 'member').size).toBe(0);
    expect(permissionsFor({ role: 'user' }, 'guest').size).toBe(0);
    expect(permissionsFor({ role: 'user' }, 'owner').has('project.permissions')).toBe(false);
  });
});

describe('hasPermission', () => {
  it('accepts items without a permission and checks the others against the granted set', () => {
    const granted: ReadonlySet<Permission> = new Set<Permission>(['org.admin']);
    expect(hasPermission(granted)).toBe(true);
    expect(hasPermission(granted, 'org.admin')).toBe(true);
    expect(hasPermission(granted, 'project.permissions')).toBe(false);
  });
});
