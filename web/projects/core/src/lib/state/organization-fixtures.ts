import { of } from 'rxjs';
import { vi } from 'vitest';

import {
  Invitation,
  InvitationCreated,
  Organization,
  OrganizationMember,
  OrgRole,
} from '../models/organization.model';

/** Test-only helpers shared by the organization store and guard specs. Not part of the public API. */

export const ORG: Organization = {
  id: 'org-1',
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
  role: 'owner',
  hasLogo: false,
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-02T00:00:00Z',
};

export function orgWith(id: string, role: OrgRole = 'member'): Organization {
  return { ...ORG, id, slug: `org-${id}`, name: `Org ${id}`, role, hasLogo: false };
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

export function invitationOf(id: string, email: string): Invitation {
  return {
    id,
    email,
    role: 'member',
    status: 'pending',
    invitedBy: null,
    expiresAt: '2026-12-01T00:00:00Z',
    acceptedAt: null,
    revokedAt: null,
    createdAt: '2026-11-01T00:00:00Z',
  };
}

export function createdOf(id: string, email: string): InvitationCreated {
  return {
    ...invitationOf(id, email),
    token: `token-${id}-abcdefghijklmnop`,
    acceptUrl: `https://corelog.test/invite/token-${id}-abcdefghijklmnop`,
  };
}

/**
 * Fake API service: every method is a spy that answers with an empty success unless overridden.
 * Overrides return observables (of, throwError or a Subject for pending requests).
 */
export function createFakeOrganizationApi(
  overrides: Readonly<Record<string, (...args: never[]) => unknown>> = {},
) {
  const spies = Object.fromEntries(
    Object.entries(overrides).map(([name, implementation]) => [name, vi.fn(implementation)]),
  );
  return {
    list: vi.fn(() => of<Organization[]>([ORG])),
    create: vi.fn(() => of(ORG)),
    leave: vi.fn(() => of(undefined)),
    createInvitations: vi.fn(() => of<InvitationCreated[]>([])),
    previewInvitation: vi.fn(() => of(undefined)),
    acceptInvitation: vi.fn(() => of(undefined)),
    updateOrganization: vi.fn(() => of(ORG)),
    uploadLogo: vi.fn(() => of('/organizations/org-1/logo')),
    getLogo: vi.fn(() => of<Blob | null>(null)),
    fetchLogoBlob: vi.fn(() => of<Blob | null>(null)),
    deleteLogo: vi.fn(() => of(undefined)),
    listMembers: vi.fn(() => of<OrganizationMember[]>([])),
    changeMemberRole: vi.fn(() => of(undefined)),
    removeMember: vi.fn(() => of(undefined)),
    transferOwnership: vi.fn(() => of(undefined)),
    deleteOrganization: vi.fn(() => of(undefined)),
    listInvitations: vi.fn(() => of<Invitation[]>([])),
    revokeInvitation: vi.fn(() => of(undefined)),
    resendInvitation: vi.fn(() => of(createdOf('i-0', 'nadie@acme.test'))),
    ...spies,
  };
}
