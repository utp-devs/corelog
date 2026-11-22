import type { ProjectBoard } from '@/core/projects/domain/board'
import { apiClient } from './api-client'

export class BoardHttpRepository {
  async getProjectBoard(projectId: string): Promise<ProjectBoard> {
    const { data } = await apiClient.get<{ data: ProjectBoard }>(`/projects/${projectId}/board`)
    return data.data
  }
}
