import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { AppError } from '@/core/shared/app-error'

const ORG_ID = '00000000-0000-4000-8000-0000000000aa'
let currentOrg: string | null = ORG_ID

const repo = vi.hoisted(() => ({
  list: vi.fn(),
  get: vi.fn(),
  create: vi.fn(),
  update: vi.fn(),
  remove: vi.fn(),
  listMembers: vi.fn(),
  addMember: vi.fn(),
  updateMemberRole: vi.fn(),
  removeMember: vi.fn(),
}))

const directory = vi.hoisted(() => ({ search: vi.fn() }))

vi.mock('@/adapters/out/http/team-http-repository', () => ({
  TeamHttpRepository: class {
    list = repo.list
    get = repo.get
    create = repo.create
    update = repo.update
    remove = repo.remove
    listMembers = repo.listMembers
    addMember = repo.addMember
    updateMemberRole = repo.updateMemberRole
    removeMember = repo.removeMember
  },
}))

vi.mock('@/adapters/out/http/user-directory-http-repository', () => ({
  UserDirectoryHttpRepository: class {
    search = directory.search
  },
}))

vi.mock('@/adapters/in/state/org.store', () => ({
  getCurrentOrgId: () => currentOrg,
}))

import { useTeamsStore } from './teams.store'

const team = (id: string, name: string) => ({
  id,
  organizationId: ORG_ID,
  name,
  description: '',
  avatarColor: null,
  createdBy: null,
  createdAt: '2026-11-01T10:00:00Z',
  updatedAt: '2026-11-01T10:00:00Z',
})

describe('teams.store', () => {
  beforeEach(() => {
    currentOrg = ORG_ID
    useTeamsStore.getState().reset()
    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  it('loads teams and keeps an AppError with its code when the list fails', async () => {
    repo.list.mockResolvedValueOnce([team('t1', 'Alfa')])
    await useTeamsStore.getState().fetchTeams()
    expect(useTeamsStore.getState().teams.map((t) => t.id)).toEqual(['t1'])
    expect(useTeamsStore.getState().status).toBe('idle')

    repo.list.mockRejectedValueOnce(new AppError({ status: 404, code: 'team.organization_not_found', message: 'no existe' }))
    await useTeamsStore.getState().fetchTeams()
    const state = useTeamsStore.getState()
    expect(state.status).toBe('error')
    expect(state.error?.code).toBe('team.organization_not_found')
  })

  it('creates a team using the current organization id', async () => {
    repo.create.mockResolvedValueOnce(team('t2', 'Beta'))

    await useTeamsStore.getState().createTeam({ name: 'Beta' })

    expect(repo.create).toHaveBeenCalledWith({ name: 'Beta', organizationId: ORG_ID })
    expect(useTeamsStore.getState().teams.map((t) => t.name)).toEqual(['Beta'])
  })

  it('refuses to create a team without an active organization', async () => {
    currentOrg = null

    const error = await useTeamsStore.getState().createTeam({ name: 'Gamma' }).catch((e: unknown) => e)

    expect((error as AppError).code).toBe('team.organization_required')
    expect(repo.create).not.toHaveBeenCalled()
  })

  it('surfaces a duplicate name code from create', async () => {
    repo.create.mockRejectedValueOnce(new AppError({ status: 409, code: 'team.name_taken', message: 'ya existe' }))

    const error = await useTeamsStore.getState().createTeam({ name: 'Beta' }).catch((e: unknown) => e)

    expect((error as AppError).code).toBe('team.name_taken')
    expect(useTeamsStore.getState().error?.code).toBe('team.name_taken')
  })

  it('replaces the updated team in the list', async () => {
    useTeamsStore.setState({ teams: [team('t1', 'Alfa')] })
    repo.update.mockResolvedValueOnce(team('t1', 'Alfa renombrado'))

    await useTeamsStore.getState().updateTeam('t1', { name: 'Alfa renombrado' })

    expect(repo.update).toHaveBeenCalledWith('t1', { name: 'Alfa renombrado' })
    expect(useTeamsStore.getState().teams[0].name).toBe('Alfa renombrado')
  })

  it('joins memberships with user names and emails', async () => {
    repo.listMembers.mockResolvedValueOnce([
      { teamId: 't1', userId: 'u1', role: 'maintainer', createdAt: '2026-11-01T10:00:00Z' },
      { teamId: 't1', userId: 'u2', role: 'member', createdAt: '2026-11-01T10:00:00Z' },
    ])
    directory.search.mockResolvedValueOnce([{ id: 'u1', name: 'Ana', email: 'ana@example.com' }])

    await useTeamsStore.getState().fetchMembers('t1')

    const members = useTeamsStore.getState().members
    expect(members[0]).toMatchObject({ userId: 'u1', role: 'maintainer', name: 'Ana', email: 'ana@example.com' })
    expect(members[1]).toMatchObject({ userId: 'u2', name: 'u2', email: '' })
    expect(useTeamsStore.getState().membersStatus).toBe('idle')
  })

  it('adds a member with a role and refreshes the list', async () => {
    repo.addMember.mockResolvedValueOnce({ teamId: 't1', userId: 'u3', role: 'maintainer', createdAt: '' })
    repo.listMembers.mockResolvedValueOnce([])
    directory.search.mockResolvedValueOnce([])

    await useTeamsStore.getState().addMember('t1', 'u3', 'maintainer')

    expect(repo.addMember).toHaveBeenCalledWith('t1', 'u3', 'maintainer')
    expect(repo.listMembers).toHaveBeenCalledWith('t1')
  })

  it('passes team.user_not_in_org from addMember through to the caller', async () => {
    repo.addMember.mockRejectedValueOnce(new AppError({ status: 422, code: 'team.user_not_in_org', message: 'no pertenece' }))

    const error = await useTeamsStore.getState().addMember('t1', 'u9').catch((e: unknown) => e)

    expect((error as AppError).code).toBe('team.user_not_in_org')
  })

  it('changes a member role in place', async () => {
    useTeamsStore.setState({
      members: [{ teamId: 't1', userId: 'u1', role: 'member', createdAt: '', name: 'Ana', email: '' }],
    })
    repo.updateMemberRole.mockResolvedValueOnce({ teamId: 't1', userId: 'u1', role: 'maintainer', createdAt: '' })

    await useTeamsStore.getState().updateMemberRole('t1', 'u1', 'maintainer')

    expect(useTeamsStore.getState().members[0].role).toBe('maintainer')
  })

  it('removes a member from the list', async () => {
    useTeamsStore.setState({
      members: [{ teamId: 't1', userId: 'u1', role: 'member', createdAt: '', name: 'Ana', email: '' }],
    })
    repo.removeMember.mockResolvedValueOnce(undefined)

    await useTeamsStore.getState().removeMember('t1', 'u1')

    expect(useTeamsStore.getState().members).toEqual([])
  })
})
