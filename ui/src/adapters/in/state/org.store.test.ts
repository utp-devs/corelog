import { beforeEach, describe, expect, it } from 'vitest'

import type { OrganizationRepositoryPort } from '@/core/organizations/application/ports/organization-repository.port'
import type { Organization } from '@/core/organizations/domain/organization'
import { AppError } from '@/core/shared/app-error'
import { CURRENT_ORG_STORAGE_KEY, createOrgStore, getCurrentOrgId } from './org.store'

function org(id: string, role: Organization['role'] = 'member'): Organization {
  return {
    id,
    slug: `org-${id}`,
    name: `Org ${id}`,
    description: '',
    hasLogo: false,
    settings: {
      whoCanCreateProjects: 'members',
      whoCanCreateTeams: 'members',
      defaultProjectVisibility: 'open',
      require2fa: false,
      maxUploadBytes: 1,
      hoursPerDay: 8,
      daysPerWeek: 5,
      timezone: 'UTC',
    },
    role,
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
  }
}

function fakeRepo(initial: Organization[] = []) {
  const state = { orgs: initial, failList: false, left: [] as string[] }
  const repo: OrganizationRepositoryPort = {
    list: async () => {
      if (state.failList) throw new AppError({ status: 500, message: 'boom' })
      return state.orgs
    },
    create: async (input) => {
      const created = org('new', 'owner')
      created.name = input.name
      state.orgs = [...state.orgs, created]
      return created
    },
    leave: async (id) => {
      state.left.push(id)
      state.orgs = state.orgs.filter((item) => item.id !== id)
    },
    createInvitations: async () => [],
    previewInvitation: async () => {
      throw new AppError({ status: 404, code: 'org.invitation_not_found', message: 'no' })
    },
    acceptInvitation: async () => ({ organizationId: 'a', userId: 'u', role: 'member', joinedAt: '2026-01-01T00:00:00Z' }),
  }
  return { repo, state }
}

describe('org.store', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  it('selects the first organization when nothing is persisted', async () => {
    const { repo } = fakeRepo([org('a'), org('b')])
    const store = createOrgStore(repo)
    await store.getState().fetchOrgs()
    expect(store.getState().currentOrgId).toBe('a')
    expect(localStorage.getItem(CURRENT_ORG_STORAGE_KEY)).toBe('a')
    expect(store.getState().loaded).toBe(true)
    expect(store.getState().loading).toBe(false)
  })

  it('keeps the persisted organization when it is still in the list', async () => {
    localStorage.setItem(CURRENT_ORG_STORAGE_KEY, 'b')
    const { repo } = fakeRepo([org('a'), org('b')])
    const store = createOrgStore(repo)
    expect(store.getState().currentOrgId).toBe('b')
    await store.getState().fetchOrgs()
    expect(store.getState().currentOrgId).toBe('b')
  })

  it('falls back to the first organization when the persisted one is gone', async () => {
    localStorage.setItem(CURRENT_ORG_STORAGE_KEY, 'removed')
    const { repo } = fakeRepo([org('a')])
    const store = createOrgStore(repo)
    await store.getState().fetchOrgs()
    expect(store.getState().currentOrgId).toBe('a')
  })

  it('selects null when the user has no organization', async () => {
    localStorage.setItem(CURRENT_ORG_STORAGE_KEY, 'x')
    const store = createOrgStore(fakeRepo([]).repo)
    await store.getState().fetchOrgs()
    expect(store.getState().currentOrgId).toBeNull()
    expect(localStorage.getItem(CURRENT_ORG_STORAGE_KEY)).toBeNull()
  })

  it('stores a typed error and keeps loaded false when the list fails', async () => {
    const { repo, state } = fakeRepo([org('a')])
    state.failList = true
    const store = createOrgStore(repo)
    await store.getState().fetchOrgs()
    expect(store.getState().error).toBeInstanceOf(AppError)
    expect(store.getState().loaded).toBe(false)
    expect(store.getState().loading).toBe(false)
  })

  it('setCurrentOrg persists the choice', () => {
    const store = createOrgStore(fakeRepo().repo)
    store.getState().setCurrentOrg('z')
    expect(store.getState().currentOrgId).toBe('z')
    expect(localStorage.getItem(CURRENT_ORG_STORAGE_KEY)).toBe('z')
  })

  it('createOrg adds the organization and makes it current', async () => {
    const store = createOrgStore(fakeRepo([org('a')]).repo)
    await store.getState().fetchOrgs()
    const created = await store.getState().createOrg({ name: 'Nueva' })
    expect(store.getState().orgs.map((item) => item.id)).toEqual(['a', 'new'])
    expect(store.getState().currentOrgId).toBe(created.id)
  })

  it('leaveOrg removes the organization and selects another one when it was current', async () => {
    const { repo, state } = fakeRepo([org('a'), org('b')])
    const store = createOrgStore(repo)
    await store.getState().fetchOrgs()
    await store.getState().leaveOrg('a')
    expect(state.left).toEqual(['a'])
    expect(store.getState().orgs.map((item) => item.id)).toEqual(['b'])
    expect(store.getState().currentOrgId).toBe('b')
  })

  it('reset clears the list, the error and the persisted choice', async () => {
    const store = createOrgStore(fakeRepo([org('a')]).repo)
    await store.getState().fetchOrgs()
    store.getState().reset()
    expect(store.getState().orgs).toEqual([])
    expect(store.getState().loaded).toBe(false)
    expect(store.getState().currentOrgId).toBeNull()
    expect(localStorage.getItem(CURRENT_ORG_STORAGE_KEY)).toBeNull()
  })

  it('getCurrentOrgId reads the default store (null without a choice)', () => {
    expect(getCurrentOrgId()).toBeNull()
  })
})

describe('org.store logo state', () => {
  it('setOrgLogo updates hasLogo and bumps the logo revision of that organization only', () => {
    const store = createOrgStore(fakeRepo([org('a'), org('b')]).repo)
    store.setState({ orgs: [org('a'), org('b')] })
    store.getState().setOrgLogo('a', true)
    const state = store.getState()
    expect(state.orgs.find((item) => item.id === 'a')?.hasLogo).toBe(true)
    expect(state.orgs.find((item) => item.id === 'b')?.hasLogo).toBe(false)
    expect(state.logoRevision).toEqual({ a: 1 })
    store.getState().setOrgLogo('a', false)
    expect(store.getState().logoRevision).toEqual({ a: 2 })
  })
})
