import { describe, expect, it } from 'vitest';

import { memberOf } from '../settings-test-kit';
import { accessRowsOf, teamRowsOf } from './access-rows';

const personOf = (userId: string) => ({
  userId,
  name: userId === 'a' ? 'Ana' : 'Beto',
  email: `${userId}@x.test`,
  hasAvatar: false,
});

describe('accessRowsOf', () => {
  it('orders leads first, then by name, and tells direct from team access', () => {
    const rows = accessRowsOf(
      [
        memberOf('b', 'contributor'),
        memberOf('a', 'viewer', { viaTeam: true, effectiveRole: 'viewer', teamName: 'Soporte' }),
        memberOf('c', 'lead'),
      ],
      personOf,
    );
    expect(rows.map((row) => row.userId)).toEqual(['c', 'b', 'a']);
    expect(rows[2]).toMatchObject({
      directRole: null,
      teamName: 'Soporte',
      effectiveRole: 'viewer',
    });
    expect(rows[1].directRole).toBe('contributor');
  });
});

describe('teamRowsOf', () => {
  it('sorts teams by name', () => {
    const rows = teamRowsOf([
      { projectId: 'p', teamId: '2', teamName: 'Zeta', role: 'viewer', createdAt: '' },
      { projectId: 'p', teamId: '1', teamName: 'Alfa', role: 'lead', createdAt: '' },
    ]);
    expect(rows.map((row) => row.name)).toEqual(['Alfa', 'Zeta']);
  });
});
