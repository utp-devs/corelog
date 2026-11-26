import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { ProjectHttpRepository } from '@/adapters/out/http/project-http-repository'

import { ProjectWorkspace } from './ProjectWorkspace'

const project = {
  id: 'p1',
  key: 'DEM',
  name: 'Proyecto demo',
  description: '',
  type: 'scrum',
  visibility: 'private',
} as unknown as Awaited<ReturnType<ProjectHttpRepository['get']>>

describe('ProjectWorkspace recents', () => {
  beforeEach(() => {
    window.localStorage.clear()
    vi.spyOn(ProjectHttpRepository.prototype, 'get').mockResolvedValue(project)
  })

  it('records the project in the recent items once its header is loaded', async () => {
    render(
      <MemoryRouter initialEntries={['/projects/p1/board']}>
        <Routes>
          <Route path="/projects/:id" element={<ProjectWorkspace />}>
            <Route path="board" element={<p>Tablero</p>} />
          </Route>
        </Routes>
      </MemoryRouter>,
    )
    expect(await screen.findByText('Proyecto demo')).toBeInTheDocument()
    await waitFor(() => {
      const recent = JSON.parse(window.localStorage.getItem('corelog.recent') ?? '[]') as { kind: string; id: string; subtitle: string }[]
      expect(recent[0]).toMatchObject({ kind: 'project', id: 'p1', subtitle: 'DEM' })
    })
  })
})
