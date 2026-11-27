import { Team } from '../models';
import {
  OrganizationCreationPolicy,
  OrganizationRoleName,
  canCreateTeam,
  canManageTeam,
  filterTeams,
} from './team-policy';

describe('canCreateTeam', () => {
  const roles: (OrganizationRoleName | null)[] = ['owner', 'admin', 'member', 'guest', null];
  const policies: OrganizationCreationPolicy[] = ['members', 'admins'];

  // Matrix: organization role x who-can-create-teams policy.
  const expected: Record<string, boolean> = {
    'owner/members': true,
    'owner/admins': true,
    'admin/members': true,
    'admin/admins': true,
    'member/members': true,
    'member/admins': false,
    'guest/members': false,
    'guest/admins': false,
    'null/members': false,
    'null/admins': false,
  };

  for (const role of roles) {
    for (const policy of policies) {
      const key = `${role}/${policy}`;
      it(`returns ${expected[key]} for ${key}`, () => {
        expect(canCreateTeam(role, policy)).toBe(expected[key]);
      });
    }
  }

  it('treats a missing policy as no member creation', () => {
    expect(canCreateTeam('member', undefined)).toBe(false);
    expect(canCreateTeam('admin', undefined)).toBe(true);
  });
});

describe('canManageTeam', () => {
  it('lets organization owners and admins manage any team', () => {
    expect(canManageTeam('owner', null)).toBe(true);
    expect(canManageTeam('admin', 'member')).toBe(true);
  });

  it('lets a team maintainer manage that team', () => {
    expect(canManageTeam('member', 'maintainer')).toBe(true);
    expect(canManageTeam('guest', 'maintainer')).toBe(true);
  });

  it('keeps plain team members and non members read-only', () => {
    expect(canManageTeam('member', 'member')).toBe(false);
    expect(canManageTeam('member', null)).toBe(false);
    expect(canManageTeam('guest', undefined)).toBe(false);
    expect(canManageTeam(null, null)).toBe(false);
  });
});

describe('filterTeams', () => {
  const team = (id: string, name: string, description = ''): Team => ({
    id,
    organizationId: 'org',
    name,
    description,
    avatarColor: null,
    createdBy: null,
    createdAt: '2026-11-01T10:00:00Z',
    updatedAt: '2026-11-01T10:00:00Z',
  });
  const teams = [
    team('1', 'Plataforma', 'Infraestructura'),
    team('2', 'Soporte Nivel 1', 'Atención al cliente'),
  ];

  it('returns every team for an empty query', () => {
    expect(filterTeams(teams, '  ')).toHaveLength(2);
  });

  it('matches names and descriptions ignoring case and accents', () => {
    expect(filterTeams(teams, 'PLATAFORMA').map((t) => t.id)).toEqual(['1']);
    expect(filterTeams(teams, 'atencion').map((t) => t.id)).toEqual(['2']);
    expect(filterTeams(teams, 'nada')).toEqual([]);
  });
});
