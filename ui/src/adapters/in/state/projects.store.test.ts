import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { AppError } from '@/core/shared/app-error'

const ORG_ID = '00000000-0000-4000-8000-0000000000aa'
let currentOrg: string | null = ORG_ID

const repo = vi.hoisted(() => ({
  list: vi.fn(),
  create: vi.fn(),
  update: vi.fn(),
  listMembers: vi.fn(),
  addMember: vi.fn(),
  updateMember: vi.fn(),
  removeMember: vi.fn(),
}))

vi.mock('@/adapters/out/http/project-http-repository', () => ({
  ProjectHttpRepository: class {
    list = repo.list
    create = repo.create
    update = repo.update
    listMembers = repo.listMembers
    addMember = repo.addMember
    updateMember = repo.updateMember
    removeMember = repo.removeMember
  },
}))

vi.mock('@/adapters/in/state/org.store', () => ({
  getCurrentOrgId: () => currentOrg,
}))

import { useProjectsStore } from './projects.store'

const project = (id: string, name: string) => ({
  id,
  organizationId: ORG_ID,
  key: id.toUpperCase(),
  name,
  description: '',
  type: 'kanban' as const,
  leadId: 'u1',
  visibility: 'open' as const,
  archivedAt: null,
  createdAt: '2026-11-01T10:00:00Z',
  updatedAt: '2026-11-01T10:00:00Z',
})

describe('projects.store', () => {
  beforeEach(() => {
    currentOrg = ORG_ID
    useProjectsStore.getState().reset()
    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  it('creates a project in the current organization without sending a lead', async () => {
    repo.create.mockResolvedValueOnce(project('core', 'Core'))

    await useProjectsStore.getState().createProject({
      key: 'CORE',
      name: 'Core',
      description: '',
      type: 'kanban',
      visibility: 'open',
    })

    const input = repo.create.mock.calls[0][0] as Record<string, unknown>
    expect(input).toEqual({ key: 'CORE', name: 'Core', description: '', type: 'kanban', visibility: 'open', organizationId: ORG_ID })
    expect(input).not.toHaveProperty('leadId')
    expect(useProjectsStore.getState().projects).toHaveLength(1)
  })

  it('refuses to create a project without an active organization', async () => {
    currentOrg = null

    const error = await useProjectsStore
      .getState()
      .createProject({ key: 'CORE', name: 'Core', description: '', type: 'kanban' })
      .catch((e: unknown) => e)

    expect((error as AppError).code).toBe('project.organization_required')
    expect(repo.create).not.toHaveBeenCalled()
  })

  it('keeps the AppError code when listing fails', async () => {
    repo.list.mockRejectedValueOnce(new AppError({ status: 0, message: 'sin conexión' }))

    await useProjectsStore.getState().fetchProjects()

    expect(useProjectsStore.getState().status).toBe('error')
    expect(useProjectsStore.getState().error).toBeInstanceOf(AppError)
  })

  it('surfaces project.user_not_in_org when adding a member', async () => {
    repo.addMember.mockRejectedValueOnce(new AppError({ status: 422, code: 'project.user_not_in_org', message: 'fuera' }))

    const error = await useProjectsStore.getState().addMember('p1', 'u9', 'viewer').catch((e: unknown) => e)

    expect((error as AppError).code).toBe('project.user_not_in_org')
    expect(useProjectsStore.getState().error?.code).toBe('project.user_not_in_org')
  })

  it('surfaces project.user_not_found when adding a member', async () => {
    repo.addMember.mockRejectedValueOnce(new AppError({ status: 404, code: 'project.user_not_found', message: 'no existe' }))

    const error = await useProjectsStore.getState().addMember('p1', 'u9', 'viewer').catch((e: unknown) => e)

    expect((error as AppError).code).toBe('project.user_not_found')
  })

  it('adds a member to the list on success', async () => {
    repo.addMember.mockResolvedValueOnce({ projectId: 'p1', userId: 'u2', role: 'contributor', createdAt: '' })

    await useProjectsStore.getState().addMember('p1', 'u2', 'contributor')

    expect(useProjectsStore.getState().members.map((m) => m.userId)).toEqual(['u2'])
    expect(useProjectsStore.getState().error).toBeNull()
  })
})
