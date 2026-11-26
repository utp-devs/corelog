import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { useOrgStore } from '@/adapters/in/state/org.store'
import { OrganizationHttpRepository } from '@/adapters/out/http/organization-http-repository'
import type { Organization } from '@/core/organizations/domain/organization'
import { AppError } from '@/core/shared/app-error'

import { RequireOrg } from './RequireOrg'

const acme: Organization = {
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
  role: 'member',
  hasLogo: false,
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
}

function renderGuard() {
  return render(
    <MemoryRouter initialEntries={['/projects']}>
      <Routes>
        <Route path="/onboarding" element={<p>Asistente de alta</p>} />
        <Route element={<RequireOrg />}>
          <Route path="/projects" element={<p>Contenido protegido</p>} />
        </Route>
      </Routes>
    </MemoryRouter>,
  )
}

describe('RequireOrg', () => {
  beforeEach(() => {
    localStorage.clear()
    useOrgStore.getState().reset()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('shows a spinner while the organizations load, then renders the route', async () => {
    let resolve: (orgs: Organization[]) => void = () => undefined
    vi.spyOn(OrganizationHttpRepository.prototype, 'list').mockReturnValue(
      new Promise<Organization[]>((done) => {
        resolve = done
      }),
    )
    const { container } = renderGuard()
    expect(container.querySelector('.ant-spin')).not.toBeNull()
    expect(screen.queryByText('Contenido protegido')).not.toBeInTheDocument()
    resolve([acme])
    expect(await screen.findByText('Contenido protegido')).toBeInTheDocument()
    expect(useOrgStore.getState().currentOrgId).toBe('o1')
  })

  it('redirects a user without organizations to /onboarding', async () => {
    vi.spyOn(OrganizationHttpRepository.prototype, 'list').mockResolvedValue([])
    renderGuard()
    expect(await screen.findByText('Asistente de alta')).toBeInTheDocument()
  })

  it('fetches the organizations only once when they are already loaded', async () => {
    const list = vi.spyOn(OrganizationHttpRepository.prototype, 'list').mockResolvedValue([acme])
    useOrgStore.setState({ orgs: [acme], loaded: true, currentOrgId: 'o1' })
    renderGuard()
    expect(screen.getByText('Contenido protegido')).toBeInTheDocument()
    await waitFor(() => expect(list).not.toHaveBeenCalled())
  })

  it('offers a retry when the list fails and does not redirect', async () => {
    const list = vi
      .spyOn(OrganizationHttpRepository.prototype, 'list')
      .mockRejectedValueOnce(new AppError({ status: 500, message: 'Error del servidor.' }))
      .mockResolvedValueOnce([acme])
    renderGuard()
    expect(await screen.findByRole('alert')).toHaveTextContent('Error del servidor.')
    expect(screen.queryByText('Asistente de alta')).not.toBeInTheDocument()
    screen.getByRole('button', { name: 'Reintentar' }).click()
    expect(await screen.findByText('Contenido protegido')).toBeInTheDocument()
    expect(list).toHaveBeenCalledTimes(2)
  })
})
