import type { AvatarRepositoryPort } from '@/core/people/application/ports/avatar-repository.port'
import { apiClient } from './api-client'

interface AvatarUploadDto {
  data: { avatar_url: string }
}

export class AvatarHttpRepository implements AvatarRepositoryPort {
  async getAvatar(userId: string): Promise<Blob> {
    // responseType blob: <img> cannot send the Authorization header, so the caller turns the blob into an object URL.
    const { data } = await apiClient.get<Blob>(`/users/${userId}/avatar`, { responseType: 'blob' })
    return data
  }

  async uploadMyAvatar(file: File): Promise<string> {
    const form = new FormData()
    form.append('file', file)
    const { data } = await apiClient.put<AvatarUploadDto>('/users/me/avatar', form, {
      headers: { 'Content-Type': 'multipart/form-data' },
    })
    return data.data.avatar_url
  }

  async deleteMyAvatar(): Promise<void> {
    await apiClient.delete('/users/me/avatar')
  }
}
