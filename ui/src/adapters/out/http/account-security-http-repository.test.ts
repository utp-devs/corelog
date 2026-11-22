import { afterEach, describe, expect, it, vi } from 'vitest'

import { apiClient } from './api-client'
import { AccountSecurityHttpRepository } from './account-security-http-repository'

describe('AccountSecurityHttpRepository', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('maps sessions from snake_case and keeps the current flag', async () => {
    vi.spyOn(apiClient, 'get').mockResolvedValue({
      data: {
        data: [
          { id: 's-1', user_agent: 'Chrome', created_at: '2026-11-22T10:00:00Z', last_seen_at: '2026-11-22T11:00:00Z', current: true },
        ],
      },
    })

    const [session] = await new AccountSecurityHttpRepository().listSessions()

    expect(session).toEqual({ id: 's-1', userAgent: 'Chrome', createdAt: '2026-11-22T10:00:00Z', lastSeenAt: '2026-11-22T11:00:00Z', current: true })
  })

  it('revokes one session, the others, and deactivates with the password', async () => {
    const del = vi.spyOn(apiClient, 'delete').mockResolvedValue({ data: {} })
    const post = vi.spyOn(apiClient, 'post').mockResolvedValue({ data: {} })
    const repo = new AccountSecurityHttpRepository()

    await repo.revokeSession('s-2')
    await repo.revokeOtherSessions()
    await repo.deactivateAccount('clave-prueba')

    expect(del).toHaveBeenCalledWith('/users/me/sessions/s-2')
    expect(post).toHaveBeenNthCalledWith(1, '/users/me/sessions/revoke-others')
    expect(post).toHaveBeenNthCalledWith(2, '/users/me/deactivate', { password: 'clave-prueba' })
  })

  it('sends the given token explicitly on logout', async () => {
    const post = vi.spyOn(apiClient, 'post').mockResolvedValue({ data: {} })

    await new AccountSecurityHttpRepository().logout('token-actual')

    expect(post).toHaveBeenCalledWith('/auth/logout', undefined, { headers: { Authorization: 'Bearer token-actual' } })
  })
})
