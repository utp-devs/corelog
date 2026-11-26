import { afterEach, describe, expect, it, vi } from 'vitest'

import { apiClient } from './api-client'
import { PeopleHttpRepository, ProfileHttpRepository } from './people-http-repository'

describe('PeopleHttpRepository', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('lists the organization members with the org role', async () => {
    const get = vi.spyOn(apiClient, 'get').mockResolvedValue({
      data: {
        data: [{ user_id: 'u-1', name: 'Ana', email: 'ana@corelog.test', role: 'admin', joined_at: '2026-01-10T00:00:00Z' }],
      },
    })

    const members = await new PeopleHttpRepository().listMembers('org-1')

    expect(get).toHaveBeenCalledWith('/organizations/org-1/members')
    expect(members).toEqual([{ userId: 'u-1', name: 'Ana', email: 'ana@corelog.test', role: 'admin', joinedAt: '2026-01-10T00:00:00Z' }])
  })

  it('searches users inside the given organization', async () => {
    const get = vi.spyOn(apiClient, 'get').mockResolvedValue({
      data: { data: [{ id: 'u-1', name: 'Ana', email: 'a@x.test', role: 'user', job_title: 'Analista', department: 'Ops', avatar_key: 'k', timezone: 'UTC', created_at: '2026-01-10T00:00:00Z' }] },
    })

    const users = await new PeopleHttpRepository().searchUsers('org-1', 'ana')

    expect(get).toHaveBeenCalledWith('/users/search', { params: { q: 'ana', organization_id: 'org-1' } })
    expect(users[0].jobTitle).toBe('Analista')
    expect(users[0].avatarKey).toBe('k')
  })
})

describe('ProfileHttpRepository', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('sends only the provided fields in snake_case on update', async () => {
    const patch = vi.spyOn(apiClient, 'patch').mockResolvedValue({
      data: { data: { id: 'u-1', name: 'Ana', email: 'a@x.test', role: 'user', display_name: 'Ana T.', timezone: 'America/Bogota', created_at: '2026-01-10T00:00:00Z' } },
    })

    const user = await new ProfileHttpRepository().updateMe({ displayName: 'Ana T.', timezone: 'America/Bogota' })

    expect(patch).toHaveBeenCalledWith('/users/me', { display_name: 'Ana T.', timezone: 'America/Bogota' })
    expect(user.displayName).toBe('Ana T.')
    expect(user.timezone).toBe('America/Bogota')
  })

  it('loads the current user', async () => {
    vi.spyOn(apiClient, 'get').mockResolvedValue({
      data: { data: { id: 'u-1', name: 'Ana', email: 'a@x.test', role: 'admin', timezone: 'UTC', created_at: '2026-01-10T00:00:00Z' } },
    })

    const user = await new ProfileHttpRepository().getMe()

    expect(user.role).toBe('admin')
  })
})
