import { afterEach, describe, expect, it, vi } from 'vitest'

import { apiClient } from './api-client'
import { SearchHttpRepository } from './search-http-repository'

describe('SearchHttpRepository', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('sends the query and returns the hits', async () => {
    const get = vi.spyOn(apiClient, 'get').mockResolvedValue({
      data: { data: [{ kind: 'issue', id: 'i-1', title: 'Exportar', subtitle: 'CORE', link: '/issues/i-1' }] },
    })

    const hits = await new SearchHttpRepository().search('exportar')

    expect(get).toHaveBeenCalledWith('/search', { params: { q: 'exportar', limit: 10 } })
    expect(hits[0]).toMatchObject({ kind: 'issue', link: '/issues/i-1' })
  })
})
