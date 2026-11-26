import type { AxiosAdapter, InternalAxiosRequestConfig } from 'axios'
import { afterEach, describe, expect, it } from 'vitest'

import { apiClient } from './api-client'
import { OrganizationHttpRepository } from './organization-http-repository'

const originalAdapter = apiClient.defaults.adapter

interface Captured {
  method: string | undefined
  url: string | undefined
  params: unknown
  body: unknown
}

/** Answers every request with `payload` (or `status` for errors) and records method, url, params and body. */
function respondWith(payload: unknown, status = 200): { calls: Captured[] } {
  const calls: Captured[] = []
  const adapter: AxiosAdapter = async (config) => {
    const typed = config as InternalAxiosRequestConfig
    const raw: unknown = typed.data
    let body: unknown = raw
    if (typeof raw === 'string') body = JSON.parse(raw)
    calls.push({ method: typed.method, url: typed.url, params: typed.params, body })
    if (status >= 400) {
      return Promise.reject(
        Object.assign(new Error('failed'), {
          isAxiosError: true,
          config: typed,
          response: { status, data: payload, statusText: '', headers: {}, config: typed },
        }),
      )
    }
    return { data: payload, status, statusText: '', headers: {}, config: typed }
  }
  apiClient.defaults.adapter = adapter
  return { calls }
}

const memberDto = {
  user_id: 'u-2',
  name: 'Luis Pardo',
  email: 'luis@acme.test',
  role: 'member',
  joined_at: '2026-02-01T00:00:00Z',
}

const invitationDto = {
  id: 'i-1',
  email: 'ana@acme.test',
  role: 'admin',
  status: 'pending',
  invited_by: 'u-1',
  expires_at: '2026-12-01T00:00:00Z',
  accepted_at: null,
  revoked_at: null,
  created_at: '2026-11-20T00:00:00Z',
}

afterEach(() => {
  apiClient.defaults.adapter = originalAdapter
})

describe('OrganizationHttpRepository admin calls', () => {
  it('sends only the changed fields of a settings update in snake_case', async () => {
    const { calls } = respondWith({
      data: {
        id: 'org-1',
        slug: 'acme',
        name: 'Acme Labs',
        description: '',
        settings: {
          who_can_create_projects: 'admins',
          who_can_create_teams: 'admins',
          default_project_visibility: 'private',
          require_2fa: false,
          max_upload_bytes: 2097152,
          hours_per_day: 7.5,
          days_per_week: 5,
          timezone: 'Europe/Madrid',
        },
        created_at: '2026-01-01T00:00:00Z',
        updated_at: '2026-01-02T00:00:00Z',
      },
    })
    const org = await new OrganizationHttpRepository().updateOrganization('org-1', {
      name: 'Acme Labs',
      settings: { whoCanCreateProjects: 'admins', hoursPerDay: 7.5 },
    })
    expect(calls[0]).toMatchObject({ method: 'patch', url: '/organizations/org-1' })
    expect(calls[0].body).toEqual({
      name: 'Acme Labs',
      settings: { who_can_create_projects: 'admins', hours_per_day: 7.5 },
    })
    expect(org.settings.hoursPerDay).toBe(7.5)
    expect(org.settings.defaultProjectVisibility).toBe('private')
  })

  it('uploads the logo as multipart and returns the logo URL', async () => {
    const { calls } = respondWith({ data: { logo_url: '/organizations/org-1/logo' } })
    const file = new File(['png'], 'logo.png', { type: 'image/png' })
    const url = await new OrganizationHttpRepository().uploadLogo('org-1', file)
    expect(calls[0]).toMatchObject({ method: 'put', url: '/organizations/org-1/logo' })
    expect(calls[0].body).toBeInstanceOf(FormData)
    expect((calls[0].body as FormData).get('file')).toBeInstanceOf(File)
    expect(url).toBe('/organizations/org-1/logo')
  })

  it('returns null when the organization has no logo', async () => {
    respondWith({ error: { code: 'org.logo_not_found', message: 'No logo' } }, 404)
    const blob = await new OrganizationHttpRepository().getLogo('org-1')
    expect(blob).toBeNull()
  })

  it('maps members to camelCase', async () => {
    const { calls } = respondWith({ data: [memberDto] })
    const members = await new OrganizationHttpRepository().listMembers('org-1')
    expect(calls[0]).toMatchObject({ method: 'get', url: '/organizations/org-1/members' })
    expect(members).toEqual([
      {
        userId: 'u-2',
        name: 'Luis Pardo',
        displayName: '',
        email: 'luis@acme.test',
        jobTitle: '',
        department: '',
        hasAvatar: false,
        role: 'member',
        joinedAt: '2026-02-01T00:00:00Z',
      },
    ])
  })

  it('changes a role with a PATCH on the member', async () => {
    const { calls } = respondWith(undefined, 204)
    await new OrganizationHttpRepository().changeMemberRole('org-1', 'u-2', 'admin')
    expect(calls[0]).toMatchObject({ method: 'patch', url: '/organizations/org-1/members/u-2' })
    expect(calls[0].body).toEqual({ role: 'admin' })
  })

  it('surfaces org.last_owner from a removal as an AppError', async () => {
    respondWith({ error: { code: 'org.last_owner', message: 'Es el último propietario' } }, 409)
    await expect(new OrganizationHttpRepository().removeMember('org-1', 'u-1')).rejects.toMatchObject({
      status: 409,
      code: 'org.last_owner',
    })
  })

  it('removes a member with DELETE on the member path', async () => {
    const { calls } = respondWith(undefined, 204)
    await new OrganizationHttpRepository().removeMember('org-1', 'u-2')
    expect(calls[0]).toMatchObject({ method: 'delete', url: '/organizations/org-1/members/u-2' })
  })

  it('transfers ownership with the target user id', async () => {
    const { calls } = respondWith(undefined, 204)
    await new OrganizationHttpRepository().transferOwnership('org-1', 'u-2')
    expect(calls[0]).toMatchObject({ method: 'post', url: '/organizations/org-1/transfer-ownership' })
    expect(calls[0].body).toEqual({ user_id: 'u-2' })
  })

  it('sends the confirmation slug when deleting the organization', async () => {
    const { calls } = respondWith(undefined, 204)
    await new OrganizationHttpRepository().deleteOrganization('org-1', 'acme')
    expect(calls[0]).toMatchObject({ method: 'delete', url: '/organizations/org-1' })
    expect(calls[0].body).toEqual({ confirm_slug: 'acme' })
  })

  it('lists invitations with the requested status scope', async () => {
    const { calls } = respondWith({ data: [invitationDto] })
    const invitations = await new OrganizationHttpRepository().listInvitations('org-1', 'all')
    expect(calls[0]).toMatchObject({ method: 'get', url: '/organizations/org-1/invitations', params: { status: 'all' } })
    expect(invitations[0]).toMatchObject({ id: 'i-1', invitedBy: 'u-1', expiresAt: '2026-12-01T00:00:00Z' })
  })

  it('revokes an invitation with DELETE', async () => {
    const { calls } = respondWith(undefined, 204)
    await new OrganizationHttpRepository().revokeInvitation('org-1', 'i-1')
    expect(calls[0]).toMatchObject({ method: 'delete', url: '/organizations/org-1/invitations/i-1' })
  })

  it('resends an invitation and returns the new plain link', async () => {
    const { calls } = respondWith({
      data: { ...invitationDto, token: 'tok-123-abcdefghijklmnop', accept_url: 'https://corelog.test/invite/tok-123' },
    })
    const created = await new OrganizationHttpRepository().resendInvitation('org-1', 'i-1')
    expect(calls[0]).toMatchObject({ method: 'post', url: '/organizations/org-1/invitations/i-1/resend' })
    expect(created).toMatchObject({ token: 'tok-123-abcdefghijklmnop', acceptUrl: 'https://corelog.test/invite/tok-123' })
  })
})
