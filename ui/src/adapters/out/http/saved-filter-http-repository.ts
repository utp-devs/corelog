import { apiClient } from './api-client'
import type { SavedFilter, SavedFilterCriteria } from '@/core/tickets/domain/saved-filter'

interface SavedFilterDto {
  id: string
  name: string
  filters: {
    status?: SavedFilterCriteria['status']
    priority?: SavedFilterCriteria['priority']
    query?: string
    issue_type?: SavedFilterCriteria['issueType']
    due_from?: string
    due_to?: string
  }
  created_at: string
  updated_at: string
}

function fromDto(dto: SavedFilterDto): SavedFilter {
  return {
    id: dto.id,
    name: dto.name,
    criteria: {
      status: dto.filters.status,
      priority: dto.filters.priority,
      query: dto.filters.query,
      issueType: dto.filters.issue_type,
      dueFrom: dto.filters.due_from,
      dueTo: dto.filters.due_to,
    },
    createdAt: dto.created_at,
    updatedAt: dto.updated_at,
  }
}

function toDto(criteria: SavedFilterCriteria): SavedFilterDto['filters'] {
  return {
    status: criteria.status,
    priority: criteria.priority,
    query: criteria.query,
    issue_type: criteria.issueType,
    due_from: criteria.dueFrom,
    due_to: criteria.dueTo,
  }
}

export class SavedFilterHttpRepository {
  async list(): Promise<SavedFilter[]> {
    const { data } = await apiClient.get<{ data: SavedFilterDto[] }>('/saved-filters')
    return data.data.map(fromDto)
  }

  async create(name: string, criteria: SavedFilterCriteria): Promise<SavedFilter> {
    const { data } = await apiClient.post<{ data: SavedFilterDto }>('/saved-filters', {
      name,
      filters: toDto(criteria),
    })
    return fromDto(data.data)
  }

  async delete(id: string): Promise<void> {
    await apiClient.delete(`/saved-filters/${id}`)
  }
}
