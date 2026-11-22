import { App } from 'antd'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const store = vi.hoisted(() => ({
  teams: [] as { id: string; name: string; createdAt: string }[],
  status: 'idle',
  members: [],
  membersStatus: 'idle',
  fetchTeams: vi.fn(),
  createTeam: vi.fn(),
  renameTeam: vi.fn(),
  removeTeam: vi.fn(),
  fetchMembers: vi.fn(),
  addMember: vi.fn(),
  removeMember: vi.fn(),
}))

vi.mock('@/adapters/in/state/teams.store', () => ({
  useTeamsStore: (selector: (state: unknown) => unknown) => selector(store),
}))

import { TeamsPage } from './TeamsPage'

function renderPage() {
  render(
    <App>
      <TeamsPage />
    </App>,
  )
}

describe('TeamsPage', () => {
  beforeEach(() => {
    store.teams = []
    store.removeTeam.mockReset().mockResolvedValue(undefined)
  })

  it('asks for a team selection before showing members', () => {
    renderPage()
    expect(screen.getByText('Selecciona un equipo')).toBeInTheDocument()
  })

  it('confirms the consequence before deleting a team', async () => {
    store.teams = [{ id: 'team-1', name: 'Soporte Nivel 1', createdAt: '2026-01-01T00:00:00Z' }]
    renderPage()

    fireEvent.click(screen.getByRole('button', { name: /Eliminar$/ }))
    const dialog = await screen.findByRole('dialog')
    expect(within(dialog).getByText('Eliminar el equipo Soporte Nivel 1')).toBeInTheDocument()
    expect(within(dialog).getByText(/no se puede deshacer/)).toBeInTheDocument()
    expect(store.removeTeam).not.toHaveBeenCalled()

    fireEvent.click(within(dialog).getByRole('button', { name: 'Eliminar equipo' }))
    await waitFor(() => expect(store.removeTeam).toHaveBeenCalledWith('team-1'))
  })

  it('keeps the team when the deletion is cancelled', async () => {
    store.teams = [{ id: 'team-1', name: 'Soporte Nivel 1', createdAt: '2026-01-01T00:00:00Z' }]
    renderPage()

    fireEvent.click(screen.getByRole('button', { name: /Eliminar$/ }))
    const dialog = await screen.findByRole('dialog')
    fireEvent.click(within(dialog).getByRole('button', { name: 'Cancelar' }))

    // jsdom does not finish the modal close animation, so assert the outcome instead of the dialog.
    await waitFor(() => expect(store.removeTeam).not.toHaveBeenCalled())
    expect(screen.getByText('Soporte Nivel 1')).toBeInTheDocument()
  })
})
