import type { SearchHit } from '@/core/search/domain/search'
import { apiClient } from './api-client'

interface SearchHitDto {
  kind: SearchHit['kind']
  id: string
  title: string
  subtitle: string
  link: string
}

export class SearchHttpRepository {
  async search(query: string, limit = 10): Promise<SearchHit[]> {
    const { data } = await apiClient.get<{ data: SearchHitDto[] }>('/search', { params: { q: query, limit } })
    return data.data
  }
}
