import { Provider, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import {
  AuthStore,
  Invitation,
  InvitationCreated,
  Organization,
  OrganizationApiService,
  OrganizationMember,
  OrganizationStore,
  OrgRole,
} from '@corelog/core';
import { provideCorelogTesting } from '@corelog/core/testing';
import { Observable, of } from 'rxjs';
import { Mock, vi } from 'vitest';

export const ORG_ID = 'org-1';
export const ME_ID = 'me';

export function orgOf(role: OrgRole, overrides: Partial<Organization> = {}): Organization {
  return {
    id: ORG_ID,
    slug: 'acme',
    name: 'Acme',
    description: 'Equipo de pruebas',
    settings: {
      whoCanCreateProjects: 'members',
      whoCanCreateTeams: 'admins',
      defaultProjectVisibility: 'open',
      require2fa: false,
      maxUploadBytes: 2 * 1024 * 1024,
      hoursPerDay: 8,
      daysPerWeek: 5,
      timezone: 'Europe/Madrid',
    },
    role,
    hasLogo: false,
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-02T00:00:00Z',
    ...overrides,
  };
}

export function memberOf(userId: string, name: string, role: OrgRole): OrganizationMember {
  return {
    userId,
    name,
    displayName: '',
    email: `${userId}@acme.test`,
    jobTitle: '',
    department: '',
    hasAvatar: false,
    role,
    joinedAt: '2026-02-01T00:00:00Z',
  };
}

export function invitationOf(
  id: string,
  email: string,
  status: Invitation['status'] = 'pending',
): Invitation {
  return {
    id,
    email,
    role: 'member',
    status,
    invitedBy: null,
    expiresAt: '2026-12-01T00:00:00Z',
    acceptedAt: null,
    revokedAt: null,
    createdAt: '2026-11-01T00:00:00Z',
  };
}

export function createdOf(id: string, email: string): InvitationCreated {
  const token = `token-${id}-abcdefghijklmnop`;
  return {
    ...invitationOf(id, email),
    token,
    acceptUrl: `https://corelog.test/invite/${token}`,
  };
}

/** Fake of the organization HTTP service. Every method answers with an empty success unless overridden. */
export type FakeOrgApi = Record<keyof OrganizationApiService, Mock>;

export function fakeOrgApi(
  overrides: Partial<
    Record<keyof OrganizationApiService, (...args: never[]) => Observable<unknown>>
  > = {},
  orgs: Organization[] = [orgOf('owner')],
): FakeOrgApi {
  const defaults: Record<keyof OrganizationApiService, (...args: never[]) => Observable<unknown>> =
    {
      list: () => of(orgs),
      create: () => of(orgOf('owner')),
      leave: () => of(undefined),
      createInvitations: () => of([]),
      previewInvitation: () => of(undefined),
      acceptInvitation: () => of(undefined),
      updateOrganization: () => of(orgOf('owner')),
      uploadLogo: () => of('/organizations/org-1/logo'),
      getLogo: () => of(null),
      fetchLogoBlob: () => of(null),
      deleteLogo: () => of(undefined),
      listMembers: () => of([]),
      changeMemberRole: () => of(undefined),
      removeMember: () => of(undefined),
      transferOwnership: () => of(undefined),
      deleteOrganization: () => of(undefined),
      listInvitations: () => of([]),
      revokeInvitation: () => of(undefined),
      resendInvitation: () => of(createdOf('i-1', 'ana@acme.test')),
    } as unknown as Record<keyof OrganizationApiService, (...args: never[]) => Observable<unknown>>;

  const fake = {} as FakeOrgApi;
  for (const key of Object.keys(defaults) as (keyof OrganizationApiService)[]) {
    fake[key] = vi.fn(overrides[key] ?? defaults[key]);
  }
  return fake;
}

export interface OrgTestContext {
  readonly api: FakeOrgApi;
  readonly store: OrganizationStore;
}

/**
 * Configures TestBed with the fake API, a signed-in user and an active organization whose caller
 * holds `role`. The organization list is loaded before the test body runs.
 */
export function setupOrgContext(
  role: OrgRole,
  overrides: Partial<
    Record<keyof OrganizationApiService, (...args: never[]) => Observable<unknown>>
  > = {},
  extraProviders: Provider[] = [],
): OrgTestContext {
  localStorage.clear();
  const api = fakeOrgApi(overrides, [orgOf(role)]);
  TestBed.configureTestingModule({
    providers: [
      provideCorelogTesting(),
      { provide: OrganizationApiService, useValue: api },
      { provide: AuthStore, useValue: { user: signal({ id: ME_ID, name: 'Yo' }).asReadonly() } },
      ...extraProviders,
    ],
  });
  const store = TestBed.inject(OrganizationStore);
  TestBed.flushEffects();
  store.fetchOrgs();
  store.setCurrentOrg(ORG_ID);
  return { api, store };
}
