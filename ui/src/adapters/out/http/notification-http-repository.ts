import type { Notification } from '@/core/notifications/domain/notification'
import { apiClient } from './api-client'

interface NotificationDto {
  id: string
  kind: string
  message: string
  link: string
  read: boolean
  created_at: string
}

function toNotification(dto: NotificationDto): Notification {
  return { id: dto.id, kind: dto.kind, message: dto.message, link: dto.link, read: dto.read, createdAt: dto.created_at }
}

export class NotificationHttpRepository {
  async list(): Promise<Notification[]> {
    const { data } = await apiClient.get<{ data: NotificationDto[] }>('/notifications')
    return data.data.map(toNotification)
  }

  async unreadCount(): Promise<number> {
    const { data } = await apiClient.get<{ data: { count: number } }>('/notifications/unread-count')
    return data.data.count
  }

  async markRead(id: string): Promise<void> {
    await apiClient.post(`/notifications/${id}/read`)
  }

  async markAllRead(): Promise<void> {
    await apiClient.post('/notifications/read-all')
  }
}
