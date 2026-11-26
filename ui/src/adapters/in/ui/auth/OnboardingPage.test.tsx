import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { useOrgStore } from '@/adapters/in/state/org.store'
import { OrganizationHttpRepository } from '@/adapters/out/http/organization-http-repository'
import type { Organization, InvitationCreated } from '@/core/organizations/domain/organization'
import { AppError } from '@/core/shared/app-error'

import { OnboardingPage } from './OnboardingPage'

function org(id: string, name: string): Organization {
  return {
    id,
    slug: 'acme-corp',
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
    role: 'owner',
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
  }
}

function invitation(email: string): InvitationCreated {
  return {
    id: 'i1',
    email,
    role: 'member',
    status: 'pending',
    invitedBy: 'u1',
    expiresAt: '2026-01-08T00:00:00Z',
    acceptedAt: null,
    revokedAt: null,
    createdAt: '2026-01-01T00:00:00Z',
    token: 'tok-abcdefghijklmnop',
    acceptUrl: 'http://localhost/invite/tok-abcdefghijklmnop',
  }
}

function renderWizard(path = '/onboarding') {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/onboarding" element={<OnboardingPage />} />
        <Route path="/projects" element={<p>Pantalla de proyectos</p>} />
        <Route path="/invite/:token" element={<p>Pantalla de invitación</p>} />
      </Routes>
    </MemoryRouter>,
  )
}

/** Fills the organization form (name derives the slug) and submits it. */
function fillOrgForm(name = 'Acme Corp') {
  fireEvent.change(screen.getByRole('textbox', { name: 'Nombre de la organización' }), { target: { value: name } })
  fireEvent.click(screen.getByRole('button', { name: 'Crear organización' }))
}

describe('OnboardingPage', () => {
  beforeEach(() => {
    localStorage.clear()
    useOrgStore.getState().reset()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('starts at the choice step with create and join options', () => {
    renderWizard()
    expect(screen.getByRole('button', { name: 'Crear organización' })).toBeInTheDocument()
    expect(screen.getByRole('textbox', { name: 'Enlace o código de invitación' })).toBeInTheDocument()
  })

  it('starts at the organization step with ?new=1', () => {
    renderWizard('/onboarding?new=1')
    expect(screen.getByRole('heading', { name: 'Datos de la organización' })).toBeInTheDocument()
    expect(screen.queryByRole('textbox', { name: 'Enlace o código de invitación' })).not.toBeInTheDocument()
  })

  it('derives the slug from the name and keeps it editable', () => {
    renderWizard('/onboarding?new=1')
    fireEvent.change(screen.getByRole('textbox', { name: 'Nombre de la organización' }), { target: { value: 'Acme Corp' } })
    expect(screen.getByRole('textbox', { name: 'Identificador de la organización' })).toHaveValue('acme-corp')
    fireEvent.change(screen.getByRole('textbox', { name: 'Identificador de la organización' }), { target: { value: 'mi-org' } })
    fireEvent.change(screen.getByRole('textbox', { name: 'Nombre de la organización' }), { target: { value: 'Otra' } })
    expect(screen.getByRole('textbox', { name: 'Identificador de la organización' })).toHaveValue('mi-org')
  })

  it('does not submit an invalid slug', async () => {
    renderWizard('/onboarding?new=1')
    const create = vi.spyOn(OrganizationHttpRepository.prototype, 'create')
    fireEvent.change(screen.getByRole('textbox', { name: 'Nombre de la organización' }), { target: { value: 'Acme' } })
    fireEvent.change(screen.getByRole('textbox', { name: 'Identificador de la organización' }), { target: { value: 'a' } })
    fireEvent.click(screen.getByRole('button', { name: 'Crear organización' }))
    await waitFor(() => expect(create).not.toHaveBeenCalled())
    expect(await screen.findByText(/Usa 3 a 40 caracteres/)).toBeInTheDocument()
  })

  it('shows the server message when the slug is taken and stays on the form', async () => {
    renderWizard('/onboarding?new=1')
    vi.spyOn(OrganizationHttpRepository.prototype, 'create').mockRejectedValue(
      new AppError({ status: 409, code: 'org.slug_taken', message: 'taken' }),
    )
    fillOrgForm()
    expect(await screen.findByText('Ese identificador ya está en uso. Elige otro.')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Datos de la organización' })).toBeInTheDocument()
  })

  it('shows the limit message when the user already owns five organizations', async () => {
    renderWizard('/onboarding?new=1')
    vi.spyOn(OrganizationHttpRepository.prototype, 'create').mockRejectedValue(
      new AppError({ status: 409, code: 'org.limit_reached', message: 'limit' }),
    )
    fillOrgForm()
    expect(await screen.findByText('Has alcanzado el máximo de organizaciones que puedes crear (5).')).toBeInTheDocument()
  })

  it('creates the organization, makes it current and moves to the invite step', async () => {
    renderWizard('/onboarding?new=1')
    const create = vi.spyOn(OrganizationHttpRepository.prototype, 'create').mockResolvedValue(org('o-new', 'Acme Corp'))
    fillOrgForm()
    expect(await screen.findByRole('heading', { name: 'Invita a tu equipo' })).toBeInTheDocument()
    expect(create).toHaveBeenCalledWith({ name: 'Acme Corp', slug: 'acme-corp' })
    expect(useOrgStore.getState().currentOrgId).toBe('o-new')
  })

  it('validates invitation emails and sends the valid ones', async () => {
    renderWizard('/onboarding?new=1')
    vi.spyOn(OrganizationHttpRepository.prototype, 'create').mockResolvedValue(org('o-new', 'Acme Corp'))
    const invite = vi
      .spyOn(OrganizationHttpRepository.prototype, 'createInvitations')
      .mockResolvedValue([invitation('ana@example.com')])
    fillOrgForm()
    await screen.findByRole('heading', { name: 'Invita a tu equipo' })

    fireEvent.change(screen.getByRole('textbox', { name: 'Correo del invitado 1' }), { target: { value: 'no-es-correo' } })
    fireEvent.click(screen.getByRole('button', { name: 'Crear invitaciones' }))
    expect(await screen.findByText('Introduce un correo electrónico válido')).toBeInTheDocument()
    expect(invite).not.toHaveBeenCalled()

    fireEvent.change(screen.getByRole('textbox', { name: 'Correo del invitado 1' }), { target: { value: 'ana@example.com' } })
    fireEvent.click(screen.getByRole('button', { name: 'Crear invitaciones' }))
    expect(await screen.findByRole('button', { name: 'Copiar enlace de ana@example.com' })).toBeInTheDocument()
    expect(invite).toHaveBeenCalledWith('o-new', [{ email: 'ana@example.com', role: 'member' }])
  })

  it('lets the user skip the invite step and finish in the projects page', async () => {
    renderWizard('/onboarding?new=1')
    vi.spyOn(OrganizationHttpRepository.prototype, 'create').mockResolvedValue(org('o-new', 'Acme Corp'))
    fillOrgForm()
    fireEvent.click(await screen.findByRole('button', { name: 'Omitir este paso' }))
    expect(await screen.findByRole('heading', { name: 'Todo listo' })).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Ir a proyectos' }))
    expect(await screen.findByText('Pantalla de proyectos')).toBeInTheDocument()
  })

  it('takes a pasted invitation link to the accept page', async () => {
    renderWizard()
    fireEvent.change(screen.getByRole('textbox', { name: 'Enlace o código de invitación' }), {
      target: { value: 'http://localhost:5173/invite/tok-abcdefghijklmnop' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Continuar' }))
    expect(await screen.findByText('Pantalla de invitación')).toBeInTheDocument()
  })

  it('rejects text that is not an invitation link', async () => {
    renderWizard()
    fireEvent.change(screen.getByRole('textbox', { name: 'Enlace o código de invitación' }), { target: { value: 'hola' } })
    fireEvent.click(screen.getByRole('button', { name: 'Continuar' }))
    expect(await screen.findByText('Introduce un enlace de invitación válido.')).toBeInTheDocument()
  })
})
