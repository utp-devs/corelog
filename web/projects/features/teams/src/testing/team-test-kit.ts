import { signal } from '@angular/core';
import {
  ApiError,
  Organization,
  OrgRole,
  Team,
  TeamMemberView,
  TeamRole,
  TeamStoreStatus,
} from '@corelog/core';
import { Observable, of } from 'rxjs';
import { vi } from 'vitest';

export const ORG_ID = 'org-1';

export function teamOf(id: string, name: string, description = ''): Team {
  return {
    id,
    organizationId: ORG_ID,
    name,
    description,
    avatarColor: null,
    createdBy: null,
    createdAt: '2026-11-01T10:00:00Z',
    updatedAt: '2026-11-01T10:00:00Z',
  };
}

export function memberOf(userId: string, name: string, role: TeamRole, email = ''): TeamMemberView {
  return { teamId: 'team-1', userId, role, createdAt: '', name, email };
}

export function apiErrorOf(code: string, message: string, status = 400): ApiError {
  return { status, code, message, details: [] };
}

/** Writable signals plus spies for every TeamStore member the team pages use. */
export function fakeTeamStore() {
  return {
    teams: signal<Team[]>([]),
    status: signal<TeamStoreStatus>('idle'),
    error: signal<ApiError | null>(null),
    currentTeam: signal<Team | null>(null),
    currentTeamStatus: signal<TeamStoreStatus>('idle'),
    members: signal<TeamMemberView[]>([]),
    membersStatus: signal<TeamStoreStatus>('idle'),
    membersError: signal<ApiError | null>(null),
    fetchTeams: vi.fn(),
    fetchTeam: vi.fn(),
    fetchMembers: vi.fn(),
    createTeam: vi.fn((): Observable<Team> => of(teamOf('new', 'Nuevo'))),
    updateTeam: vi.fn((): Observable<Team> => of(teamOf('team-1', 'Soporte'))),
    removeTeam: vi.fn((): Observable<void> => of(undefined)),
    addMember: vi.fn((): Observable<void> => of(undefined)),
    updateMemberRole: vi.fn((): Observable<void> => of(undefined)),
    removeMember: vi.fn((): Observable<void> => of(undefined)),
  };
}

export type FakeTeamStore = ReturnType<typeof fakeTeamStore>;

/** Organization store stand-in: the active organization, its team policy and the caller role. */
export function fakeOrganizationStore(
  role: OrgRole | null,
  policy: 'members' | 'admins' = 'admins',
) {
  const org = { id: ORG_ID, settings: { whoCanCreateTeams: policy } } as Organization;
  return {
    currentRole: signal<OrgRole | null>(role),
    currentOrg: signal<Organization | null>(org),
    currentOrgId: signal<string | null>(ORG_ID),
  };
}

export function fakeAuthStore(userId: string | null) {
  return { user: signal(userId === null ? null : { id: userId }) };
}

export function fakeDirectory() {
  return { searchTeammates: vi.fn((_organizationId: string, _query: string) => of([])) };
}

/** jsdom has no ResizeObserver; PrimeNG tabs observe their size, so specs with tabs stub it. */
class ResizeObserverStub {
  observe(): void {}
  unobserve(): void {}
  disconnect(): void {}
}

export function stubResizeObserver(): void {
  vi.stubGlobal('ResizeObserver', ResizeObserverStub);
}
