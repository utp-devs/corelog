import { afterEach, describe, expect, it, vi } from 'vitest'

import { AppError } from '@/core/shared/app-error'
import { apiClient } from './api-client'
import { ProjectHttpRepository } from './project-http-repository'

const ORG_ID = '00000000-0000-4000-8000-0000000000aa'

vi.mock('@/adapters/in/state/org.store', () => ({
  getCurrentOrgId: () => '00000000-0000-4000-8000-0000000000aa',
}))

const projectDto = {
  id: 'project-1',
  organization_id: ORG_ID,
  key: 'CORE',
  name: 'CoreLog',
  description: '',
  type: 'kanban' as const,
  lead_id: 'user-1',
  visibility: 'open' as const,
  archived_at: null,
  created_at: '2026-11-01T10:00:00Z',
  updated_at: '2026-11-01T10:00:00Z',
}

describe('ProjectHttpRepository (organization and visibility)', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('lists projects with organization_id and maps organizationId and open visibility', async () => {
    const get = vi.spyOn(apiClient, 'get').mockResolvedValue({ data: { data: [projectDto] } })

    const [project] = await new ProjectHttpRepository().list()

    expect(get).toHaveBeenCalledWith('/projects', { params: { organization_id: ORG_ID } })
    expect(project).toMatchObject({ organizationId: ORG_ID, visibility: 'open', leadId: 'user-1' })
  })

  it('creates projects with organization_id and without lead_id', async () => {
    const post = vi.spyOn(apiClient, 'post').mockResolvedValue({ data: { data: projectDto } })

    await new ProjectHttpRepository().create({
      organizationId: ORG_ID,
      key: 'CORE',
      name: 'CoreLog',
      description: '',
      type: 'kanban',
      visibility: 'open',
    })

    const body = post.mock.calls[0][1] as Record<string, unknown>
    expect(body).toEqual({ organization_id: ORG_ID, key: 'CORE', name: 'CoreLog', description: '', type: 'kanban', visibility: 'open' })
    expect(body).not.toHaveProperty('lead_id')
  })

  it('sends the lead and visibility on update', async () => {
    const patch = vi.spyOn(apiClient, 'patch').mockResolvedValue({ data: { data: projectDto } })

    await new ProjectHttpRepository().update('project-1', {
      name: 'CoreLog',
      description: '',
      type: 'scrum',
      leadId: 'user-2',
      visibility: 'private',
    })

    expect(patch).toHaveBeenCalledWith('/projects/project-1', {
      name: 'CoreLog',
      description: '',
      type: 'scrum',
      lead_id: 'user-2',
      visibility: 'private',
    })
  })

  it('lets project.user_not_in_org pass through from addMember', async () => {
    const failure = new AppError({ status: 422, code: 'project.user_not_in_org', message: 'fuera de la organización' })
    vi.spyOn(apiClient, 'post').mockRejectedValue(failure)

    const error = await new ProjectHttpRepository().addMember('project-1', 'user-9', 'viewer').catch((e: unknown) => e)

    expect((error as AppError).code).toBe('project.user_not_in_org')
  })
})
