import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const repositories = vi.hoisted(() => ({ search: vi.fn() }))

vi.mock('@/adapters/out/http/search-http-repository', () => ({
  SearchHttpRepository: class {
    search = repositories.search
  },
}))

import { GlobalSearch } from './GlobalSearch'

describe('GlobalSearch', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    repositories.search.mockResolvedValue([
      { kind: 'issue', id: 'i-1', title: 'Exportar datos', subtitle: 'CORE', link: '/issues/i-1' },
    ])
  })

  it('does not query the API for a single character', async () => {
    render(<MemoryRouter><GlobalSearch /></MemoryRouter>)

    fireEvent.change(screen.getByLabelText('Buscar proyectos e incidencias'), { target: { value: 'e' } })

    await new Promise((resolve) => setTimeout(resolve, 350))
    expect(repositories.search).not.toHaveBeenCalled()
  })

  it('searches after the user pauses and shows the hits', async () => {
    render(<MemoryRouter><GlobalSearch /></MemoryRouter>)

    fireEvent.change(screen.getByLabelText('Buscar proyectos e incidencias'), { target: { value: 'ex' } })

    expect(await screen.findByText('Exportar datos')).toBeInTheDocument()
    expect(repositories.search).toHaveBeenCalledWith('ex')
  })

  it('reports when nothing matches', async () => {
    repositories.search.mockResolvedValue([])
    render(<MemoryRouter><GlobalSearch /></MemoryRouter>)

    fireEvent.change(screen.getByLabelText('Buscar proyectos e incidencias'), { target: { value: 'zz' } })

    await waitFor(() => expect(screen.getByText('Sin resultados')).toBeInTheDocument())
  })
})
