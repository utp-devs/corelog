import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it } from 'vitest'

import type { Team } from '@/core/teams/domain/team'

import { TeamCard } from './TeamCard'

const base: Team = {
  id: 't-1',
  organizationId: 'org-1',
  name: 'Plataforma',
  description: 'Infra',
  avatarColor: null,
  createdBy: null,
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
}

function renderCard(team: Team) {
  render(
    <MemoryRouter>
      <TeamCard team={team} />
    </MemoryRouter>,
  )
}

describe('TeamCard', () => {
  it('shows the member count with the plural form', () => {
    renderCard({ ...base, memberCount: 4 })
    expect(screen.getByText('4 miembros')).toBeInTheDocument()
  })

  it('uses the singular for one member and zero when the count is unknown', () => {
    const { unmount } = render(
      <MemoryRouter>
        <TeamCard team={{ ...base, memberCount: 1 }} />
      </MemoryRouter>,
    )
    expect(screen.getByText('1 miembro')).toBeInTheDocument()
    unmount()
    renderCard(base)
    expect(screen.getByText('0 miembros')).toBeInTheDocument()
  })

  it('shows the Responsable lozenge only when the caller is a maintainer', () => {
    const { unmount } = render(
      <MemoryRouter>
        <TeamCard team={{ ...base, myRole: 'maintainer' }} />
      </MemoryRouter>,
    )
    expect(screen.getByText('Responsable')).toBeInTheDocument()
    unmount()

    renderCard({ ...base, myRole: 'member' })
    expect(screen.queryByText('Responsable')).not.toBeInTheDocument()
  })

  it('links the whole card to the team page', () => {
    renderCard({ ...base, memberCount: 2 })
    expect(screen.getByRole('link', { name: /Plataforma/ })).toHaveAttribute('href', '/teams/t-1')
  })
})
