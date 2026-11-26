import { fireEvent, screen, waitFor } from '@testing-library/react'
import { vi } from 'vitest'

import type { OrganizationAdminRepositoryPort } from '@/core/organizations/application/ports/organization-admin-repository.port'
import type {
  Invitation,
  InvitationCreated,
  Organization,
  OrganizationMember,
  OrgRole,
} from '@/core/organizations/domain/organization'

/** Test-only helpers shared by the organization admin page tests. Not imported by application code. */

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
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-02T00:00:00Z',
}

export const ME_ID = 'me'

export function memberOf(userId: string, name: string, role: OrgRole): OrganizationMember {
  return { userId, name, email: `${userId}@acme.test`, role, joinedAt: '2026-02-01T00:00:00Z' }
}

export function invitationOf(id: string, email: string, status: Invitation['status'] = 'pending'): Invitation {
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
  }
}

export function createdOf(id: string, email: string): InvitationCreated {
  return {
    ...invitationOf(id, email),
    token: `token-${id}-abcdefghijklmnop`,
    acceptUrl: `https://corelog.test/invite/token-${id}-abcdefghijklmnop`,
  }
}

const notStubbed = async (): Promise<never> => {
  throw new Error('not stubbed')
}

/** Fake port: every method is a spy that resolves with an empty answer unless overridden. */
export function createFakeAdminRepository(
  overrides: Partial<OrganizationAdminRepositoryPort> = {},
): OrganizationAdminRepositoryPort {
  return {
    list: vi.fn(async () => [ORG]),
    create: vi.fn(notStubbed),
    leave: vi.fn(async () => undefined),
    createInvitations: vi.fn(async () => []),
    previewInvitation: vi.fn(notStubbed),
    acceptInvitation: vi.fn(notStubbed),
    updateOrganization: vi.fn(async () => ORG),
    uploadLogo: vi.fn(async () => '/organizations/org-1/logo'),
    getLogo: vi.fn(async () => null),
    deleteLogo: vi.fn(async () => undefined),
    listMembers: vi.fn(async () => []),
    changeMemberRole: vi.fn(async () => undefined),
    removeMember: vi.fn(async () => undefined),
    transferOwnership: vi.fn(async () => undefined),
    deleteOrganization: vi.fn(async () => undefined),
    listInvitations: vi.fn(async () => []),
    revokeInvitation: vi.fn(async () => undefined),
    resendInvitation: vi.fn(notStubbed),
    ...overrides,
  }
}

/**
 * Opens an antd Select and picks an option by its label. The label is looked up inside the open dropdown
 * (antd keeps closed dropdowns in the DOM), and the mouse sequence is sent to it, because a plain click
 * on the role=option element does not select in jsdom.
 */
export async function pickSelectOption(combobox: HTMLElement, label: string): Promise<void> {
  fireEvent.mouseDown(combobox)
  const item = await waitFor(() => {
    const match = screen
      .queryAllByText(label)
      .find((element) => element.closest(".ant-select-dropdown:not(.ant-select-dropdown-hidden)") !== null)
    if (!match) throw new Error(`Option "${label}" is not open`)
    return match
  })
  fireEvent.mouseDown(item)
  fireEvent.mouseUp(item)
  fireEvent.click(item)
}
