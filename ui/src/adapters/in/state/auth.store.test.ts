import { beforeEach, describe, expect, it } from 'vitest'

import { useAuthStore } from './auth.store'
import { useOrgStore } from './org.store'
import { useProjectsStore } from './projects.store'
import { useTeamsStore } from './teams.store'
import { tokenStorage } from '@/adapters/out/storage/token-storage'

describe('auth.store', () => {
  beforeEach(() => {
    localStorage.clear()
    useAuthStore.setState({
      user: null,
      status: 'idle',
      error: null,
      sessionChecked: false,
    })
  })

  it('initializes with default state', () => {
    const state = useAuthStore.getState()
    expect(state.user).toBeNull()
    expect(state.status).toBe('idle')
    expect(state.error).toBeNull()
    expect(state.sessionChecked).toBe(false)
  })

  it('clears session and token on logout', () => {
    tokenStorage.set('sample-jwt-token')
    useAuthStore.setState({
      user: {
        id: '123',
        name: 'Ada',
        email: 'ada@example.com',
        role: 'admin',
        teamId: null,
        displayName: '',
        jobTitle: '',
        department: '',
        avatarKey: null,
        timezone: 'UTC',
        lastLoginAt: null,
        emailVerifiedAt: null,
        createdAt: '2026-01-01T00:00:00Z',
      },
      status: 'authenticated',
      sessionChecked: true,
    })

    useAuthStore.getState().logout()

    const state = useAuthStore.getState()
    expect(state.user).toBeNull()
    expect(state.status).toBe('idle')
    expect(state.sessionChecked).toBe(true)
    expect(tokenStorage.get()).toBeNull()
  })

  it('clears organization, projects and teams on logout', () => {
    localStorage.setItem('corelog.currentOrg', 'o1')
    useAuthStore.setState({ user: null, status: 'authenticated', sessionChecked: true })
    useOrgStore.setState({ orgs: [{ id: 'o1' } as never], currentOrgId: 'o1', loaded: true })
    useProjectsStore.setState({ projects: [{ id: 'p1' } as never] })
    useTeamsStore.setState({ teams: [{ id: 't1' } as never], currentTeam: null })

    useAuthStore.getState().logout()

    expect(useOrgStore.getState().orgs).toEqual([])
    expect(useOrgStore.getState().currentOrgId).toBeNull()
    expect(useOrgStore.getState().loaded).toBe(false)
    expect(localStorage.getItem('corelog.currentOrg')).toBeNull()
    expect(useProjectsStore.getState().projects).toEqual([])
    expect(useTeamsStore.getState().teams).toEqual([])
  })
})
