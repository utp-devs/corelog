import { fireEvent, render, screen, within } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { beforeEach, describe, expect, it } from 'vitest'

import { useOrgStore } from '@/adapters/in/state/org.store'
import type { Organization, OrgRole } from '@/core/organizations/domain/organization'

import { OrgSwitcher } from './OrgSwitcher'

function org(id: string, name: string, role: OrgRole): Organization {
  return {
    id,
    slug: id,
    name,
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

function renderSwitcher() {
  return render(
    <MemoryRouter initialEntries={['/issues']}>
      <Routes>
        <Route path="/issues" element={<OrgSwitcher />} />
        <Route path="/projects" element={<p>Proyectos</p>} />
        <Route path="/onboarding" element={<p>Asistente de alta</p>} />
        <Route path="/organization/settings" element={<p>Configuración de la organización</p>} />
      </Routes>
    </MemoryRouter>,
  )
}

async function openMenu() {
  fireEvent.click(screen.getByRole('button', { name: /Organización actual/ }))
  return screen.findByRole('menu')
}

describe('OrgSwitcher', () => {
  beforeEach(() => {
    localStorage.clear()
    useOrgStore.setState({
      orgs: [org('a', 'Acme', 'owner'), org('b', 'Beta', 'member')],
      currentOrgId: 'a',
      loaded: true,
    })
  })

  it('shows the current organization name on the trigger', () => {
    renderSwitcher()
    expect(screen.getByRole('button', { name: 'Organización actual: Acme' })).toHaveTextContent('Acme')
  })

  it('lists every organization and marks the current one', async () => {
    renderSwitcher()
    const menu = await openMenu()
    expect(within(menu).getByRole('menuitem', { name: /Acme/ })).toBeInTheDocument()
    expect(within(menu).getByRole('menuitem', { name: /Beta/ })).toBeInTheDocument()
    expect(within(within(menu).getByRole('menuitem', { name: /Acme/ })).getByLabelText('Organización actual')).toBeInTheDocument()
    expect(within(within(menu).getByRole('menuitem', { name: /Beta/ })).queryByLabelText('Organización actual')).not.toBeInTheDocument()
  })

  it('switches the organization and goes to projects', async () => {
    renderSwitcher()
    const menu = await openMenu()
    fireEvent.click(within(menu).getByRole('menuitem', { name: /Beta/ }))
    expect(useOrgStore.getState().currentOrgId).toBe('b')
    expect(localStorage.getItem('corelog.currentOrg')).toBe('b')
    expect(await screen.findByText('Proyectos')).toBeInTheDocument()
  })

  it('opens the wizard at the organization step from "Crear organización"', async () => {
    renderSwitcher()
    const menu = await openMenu()
    fireEvent.click(within(menu).getByRole('menuitem', { name: /Crear organización/ }))
    expect(await screen.findByText('Asistente de alta')).toBeInTheDocument()
  })

  it('shows organization settings to an owner or admin', async () => {
    renderSwitcher()
    const menu = await openMenu()
    fireEvent.click(within(menu).getByRole('menuitem', { name: /Configuración de la organización/ }))
    expect(await screen.findByText('Configuración de la organización', { selector: 'p' })).toBeInTheDocument()
  })

  it('hides organization settings from a member', async () => {
    useOrgStore.setState({ orgs: [org('b', 'Beta', 'member')], currentOrgId: 'b' })
    renderSwitcher()
    const menu = await openMenu()
    expect(within(menu).queryByRole('menuitem', { name: /Configuración de la organización/ })).not.toBeInTheDocument()
    expect(within(menu).getByRole('menuitem', { name: /Crear organización/ })).toBeInTheDocument()
  })
})
