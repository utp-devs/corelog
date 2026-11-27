import { PeopleMember } from '../models/people.model';
import { filterMembers, ORG_ROLE_LABELS, ORG_ROLE_TONES } from './people-roles';

function member(name: string, email: string): PeopleMember {
  return {
    userId: name,
    name,
    displayName: '',
    email,
    jobTitle: '',
    department: '',
    hasAvatar: false,
    role: 'member',
    joinedAt: '2026-01-01T00:00:00Z',
  };
}

describe('people roles', () => {
  it('labels every organization role in Spanish with a tone', () => {
    expect(ORG_ROLE_LABELS.owner).toBe('Propietario');
    expect(ORG_ROLE_LABELS.guest).toBe('Invitado');
    expect(ORG_ROLE_TONES.guest).toBe('warning');
    expect(ORG_ROLE_TONES.owner).toBe('info');
  });
});

describe('filterMembers', () => {
  const members = [
    member('Ana Torres', 'ana@corelog.test'),
    member('Luis Peña', 'luis@example.org'),
  ];

  it('keeps every member for an empty query', () => {
    expect(filterMembers(members, '   ')).toHaveLength(2);
  });

  it('matches name and email ignoring case', () => {
    expect(filterMembers(members, 'ANA')).toEqual([members[0]]);
    expect(filterMembers(members, '@example.org')).toEqual([members[1]]);
  });
});
