import { beforeEach, describe, expect, it } from 'vitest'

import { useAuthStore } from './auth.store'
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
})
