import type { AccountSession } from '@/core/auth/domain/session'
import { apiClient } from './api-client'

interface SessionDto {
  id: string
  user_agent: string
  created_at: string
  last_seen_at: string
  current: boolean
}

function toSession(dto: SessionDto): AccountSession {
  return {
    id: dto.id,
    userAgent: dto.user_agent,
    createdAt: dto.created_at,
    lastSeenAt: dto.last_seen_at,
    current: dto.current,
  }
}

export class AccountSecurityHttpRepository {
  async listSessions(): Promise<AccountSession[]> {
    const { data } = await apiClient.get<{ data: SessionDto[] }>('/users/me/sessions')
    return data.data.map(toSession)
  }

  async revokeSession(id: string): Promise<void> {
    await apiClient.delete(`/users/me/sessions/${id}`)
  }

  async revokeOtherSessions(): Promise<void> {
    await apiClient.post('/users/me/sessions/revoke-others')
  }

  async deactivateAccount(password: string): Promise<void> {
    await apiClient.post('/users/me/deactivate', { password })
  }

  // Cierra la sesion indicada. Se pasa el token explicito porque el llamador puede estar limpiando el almacenamiento.
  async logout(token: string): Promise<void> {
    await apiClient.post('/auth/logout', undefined, { headers: { Authorization: `Bearer ${token}` } })
  }
}
