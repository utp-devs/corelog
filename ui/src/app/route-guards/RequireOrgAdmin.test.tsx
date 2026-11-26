import { render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { beforeEach, describe, expect, it } from 'vitest'

import { useOrgStore } from '@/adapters/in/state/org.store'
import type { OrgRole, Organization } from '@/core/organizations/domain/organization'

import { RequireOrgAdmin } from './RequireOrgAdmin'

function org(role: OrgRole): Organization {
  return {
    id: 'o1',
    slug: 'acme',
    name: 'Acme',
    description: '',
    settings: {
      whoCanCreateProjects: 'members',
      whoCanCreateTeams: 'members',
      defaultProjectVisibility: 'open',
      require2fa: false,
      maxUploadBytes: 1,
      hoursPerDay: 8,
      daysPerWeek: 5,
      timezone: 'UTC',
    },
    role,
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
  }
}

function renderAdminRoute() {
  return render(
    <MemoryRouter initialEntries={['/organization/members']}>
      <Routes>
        <Route element={<RequireOrgAdmin />}>
          <Route path="/organization/members" element={<p>Miembros de la organización</p>} />
        </Route>
      </Routes>
    </MemoryRouter>,
  )
}

describe('RequireOrgAdmin', () => {
  beforeEach(() => {
    useOrgStore.setState({ orgs: [], currentOrgId: 'o1', loaded: true })
  })

  it.each<OrgRole>(['owner', 'admin'])('renders the content for an organization %s', (role) => {
    useOrgStore.setState({ orgs: [org(role)] })
    renderAdminRoute()
    expect(screen.getByText('Miembros de la organización')).toBeInTheDocument()
  })

  it.each<OrgRole>(['member', 'guest'])('renders the 403 page for an organization %s', (role) => {
    useOrgStore.setState({ orgs: [org(role)] })
    renderAdminRoute()
    expect(screen.getByText('403')).toBeInTheDocument()
    expect(screen.queryByText('Miembros de la organización')).not.toBeInTheDocument()
  })
})
