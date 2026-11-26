import { afterEach, describe, expect, it, vi } from 'vitest'

import { apiClient } from './api-client'
import { PeopleHttpRepository, ProfileHttpRepository } from './people-http-repository'

describe('PeopleHttpRepository', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('lists members with job title, department and avatar flag', async () => {
    const get = vi.spyOn(apiClient, 'get').mockResolvedValue({
      data: {
        data: [
          {
            user_id: 'u-1',
            name: 'Ana',
            display_name: 'Ana T.',
            email: 'ana@corelog.test',
            job_title: 'Analista',
            department: 'Ops',
            has_avatar: true,
            role: 'admin',
            joined_at: '2026-01-10T00:00:00Z',
          },
        ],
      },
    })

    const members = await new PeopleHttpRepository().listMembers('org-1')

    expect(get).toHaveBeenCalledWith('/organizations/org-1/members')
    expect(members).toEqual([
      {
        userId: 'u-1',
        name: 'Ana',
        displayName: 'Ana T.',
        email: 'ana@corelog.test',
        jobTitle: 'Analista',
        department: 'Ops',
        hasAvatar: true,
        role: 'admin',
        joinedAt: '2026-01-10T00:00:00Z',
      },
    ])
  })

  it('defaults missing member fields instead of leaving them undefined', async () => {
    vi.spyOn(apiClient, 'get').mockResolvedValue({
      data: { data: [{ user_id: 'u-2', name: 'Luis', email: 'l@x.test', role: 'member', joined_at: '2026-01-11T00:00:00Z' }] },
    })

    const [member] = await new PeopleHttpRepository().listMembers('org-1')

    expect(member).toMatchObject({ displayName: '', jobTitle: '', department: '', hasAvatar: false })
  })

  it('gets one person profile from the people endpoint', async () => {
    const get = vi.spyOn(apiClient, 'get').mockResolvedValue({
      data: {
        data: {
          user_id: 'u-1',
          name: 'Ana',
          display_name: '',
          email: 'ana@corelog.test',
          job_title: 'Analista',
          department: 'Ops',
          timezone: 'America/Bogota',
          has_avatar: false,
          org_role: 'member',
          joined_at: '2026-01-10T00:00:00Z',
        },
      },
    })

    const person = await new PeopleHttpRepository().getPerson('org-1', 'u-1')

    expect(get).toHaveBeenCalledWith('/organizations/org-1/people/u-1')
    expect(person).toEqual({
      userId: 'u-1',
      name: 'Ana',
      displayName: '',
      email: 'ana@corelog.test',
      jobTitle: 'Analista',
      department: 'Ops',
      timezone: 'America/Bogota',
      hasAvatar: false,
      orgRole: 'member',
      joinedAt: '2026-01-10T00:00:00Z',
    })
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
