import type { AxiosAdapter, InternalAxiosRequestConfig } from 'axios'
import { afterEach, describe, expect, it } from 'vitest'

import { apiClient } from './api-client'
import { OrganizationHttpRepository } from './organization-http-repository'

const originalAdapter = apiClient.defaults.adapter

interface Captured {
  method: string | undefined
  url: string | undefined
  body: unknown
}

/** Answers every request with `payload` and records method, url and JSON body. */
function respondWith(payload: unknown, status = 200): { adapter: AxiosAdapter; calls: Captured[] } {
  const calls: Captured[] = []
  const adapter: AxiosAdapter = async (config) => {
    const typed = config as InternalAxiosRequestConfig
    const raw: unknown = typed.data
    calls.push({
      method: typed.method,
      url: typed.url,
      body: typeof raw === 'string' ? JSON.parse(raw) : raw,
    })
    return { data: payload, status, statusText: '', headers: {}, config: typed }
  }
  return { adapter, calls }
}

const orgDto = {
  id: '11111111-1111-1111-1111-111111111111',
  slug: 'acme',
  name: 'Acme',
  description: '',
  settings: {
    who_can_create_projects: 'members',
    who_can_create_teams: 'admins',
    default_project_visibility: 'open',
    require_2fa: false,
    max_upload_bytes: 10485760,
    hours_per_day: 8,
    days_per_week: 5,
    timezone: 'Europe/Madrid',
  },
  role: 'admin',
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-02T00:00:00Z',
}

afterEach(() => {
  apiClient.defaults.adapter = originalAdapter
})

describe('OrganizationHttpRepository', () => {
  it('lists organizations and maps settings to camelCase', async () => {
    const { adapter, calls } = respondWith({ data: [orgDto] })
    apiClient.defaults.adapter = adapter
    const orgs = await new OrganizationHttpRepository().list()
    expect(calls[0]).toMatchObject({ method: 'get', url: '/organizations' })
    expect(orgs[0]).toMatchObject({ id: orgDto.id, slug: 'acme', role: 'admin', createdAt: '2026-01-01T00:00:00Z' })
    expect(orgs[0].settings).toEqual({
      whoCanCreateProjects: 'members',
      whoCanCreateTeams: 'admins',
      defaultProjectVisibility: 'open',
      require2fa: false,
      maxUploadBytes: 10485760,
      hoursPerDay: 8,
      daysPerWeek: 5,
      timezone: 'Europe/Madrid',
    })
  })

  it('creates an organization without empty optional fields and defaults the role to owner', async () => {
    const { adapter, calls } = respondWith({ data: { ...orgDto, role: undefined } }, 201)
    apiClient.defaults.adapter = adapter
    const org = await new OrganizationHttpRepository().create({ name: 'Acme' })
    expect(calls[0].body).toEqual({ name: 'Acme' })
    expect(org.role).toBe('owner')
  })

  it('sends the slug when given', async () => {
    const { adapter, calls } = respondWith({ data: orgDto }, 201)
    apiClient.defaults.adapter = adapter
    await new OrganizationHttpRepository().create({ name: 'Acme', slug: 'acme' })
    expect(calls[0].body).toEqual({ name: 'Acme', slug: 'acme' })
  })

  it('posts all invitations of an organization in one request and keeps the plain token', async () => {
    const invitation = {
      id: 'i1',
      email: 'ana@example.com',
      role: 'member',
      status: 'pending',
      invited_by: null,
      expires_at: '2026-01-08T00:00:00Z',
      accepted_at: null,
      revoked_at: null,
      created_at: '2026-01-01T00:00:00Z',
      token: 'tok-abcdefghijklmnop',
      accept_url: 'http://localhost/invite/tok-abcdefghijklmnop',
    }
    const { adapter, calls } = respondWith({ data: { invitations: [invitation] } }, 201)
    apiClient.defaults.adapter = adapter
    const created = await new OrganizationHttpRepository().createInvitations(orgDto.id, [
      { email: 'ana@example.com', role: 'member' },
    ])
    expect(calls[0]).toMatchObject({ method: 'post', url: `/organizations/${orgDto.id}/invitations` })
    expect(calls[0].body).toEqual({ invitations: [{ email: 'ana@example.com', role: 'member' }] })
    expect(created[0]).toMatchObject({
      token: 'tok-abcdefghijklmnop',
      acceptUrl: invitation.accept_url,
      status: 'pending',
      expiresAt: '2026-01-08T00:00:00Z',
    })
  })

  it('previews a token with a GET and maps the preview', async () => {
    const { adapter, calls } = respondWith({
      data: {
        organization_name: 'Acme',
        role: 'guest',
        email: 'ana@example.com',
        status: 'pending',
        expires_at: '2026-01-08T00:00:00Z',
      },
    })
    apiClient.defaults.adapter = adapter
    const preview = await new OrganizationHttpRepository().previewInvitation('tok-1')
    expect(calls[0]).toMatchObject({ method: 'get', url: '/invitations/tok-1' })
    expect(preview).toEqual({
      organizationName: 'Acme',
      role: 'guest',
      email: 'ana@example.com',
      status: 'pending',
      expiresAt: '2026-01-08T00:00:00Z',
    })
  })

  it('accepts an invitation and maps the membership', async () => {
    const { adapter, calls } = respondWith(
      { data: { organization_id: orgDto.id, user_id: 'u1', role: 'member', joined_at: '2026-01-02T00:00:00Z' } },
      201,
    )
    apiClient.defaults.adapter = adapter
    const membership = await new OrganizationHttpRepository().acceptInvitation('tok-1')
    expect(calls[0]).toMatchObject({ method: 'post', url: '/invitations/tok-1/accept' })
    expect(membership).toEqual({
      organizationId: orgDto.id,
      userId: 'u1',
      role: 'member',
      joinedAt: '2026-01-02T00:00:00Z',
    })
  })

  it('leaves an organization with a POST to its leave endpoint', async () => {
    const { adapter, calls } = respondWith(undefined, 204)
    apiClient.defaults.adapter = adapter
    await new OrganizationHttpRepository().leave(orgDto.id)
    expect(calls[0]).toMatchObject({ method: 'post', url: `/organizations/${orgDto.id}/leave` })
  })
})
