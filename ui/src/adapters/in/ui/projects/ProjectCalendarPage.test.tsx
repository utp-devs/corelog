import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const repositories = vi.hoisted(() => ({ getProject: vi.fn(), events: vi.fn() }))

vi.mock('@/adapters/out/http/project-http-repository', () => ({
  ProjectHttpRepository: class {
    get = repositories.getProject
  },
}))

vi.mock('@/adapters/out/http/calendar-http-repository', () => ({
  CalendarHttpRepository: class {
    events = repositories.events
  },
}))

import { AppError } from '@/core/shared/app-error'

import { ProjectCalendarPage } from './ProjectCalendarPage'
import { guardConsole } from './testSupport/consoleGuard'

function renderPage() {
  return render(
    <MemoryRouter initialEntries={['/projects/project-1/calendar']}>
      <Routes><Route path="/projects/:id/calendar" element={<ProjectCalendarPage />} /></Routes>
    </MemoryRouter>,
  )
}

describe('ProjectCalendarPage', () => {
  const consoleGuard = guardConsole()

  beforeEach(() => {
    // Mes fijo (noviembre de 2026) para que los hitos de la prueba caigan en la vista.
    vi.useFakeTimers({ toFake: ['Date'] })
    vi.setSystemTime(new Date(2026, 10, 15))
    vi.clearAllMocks()
    repositories.getProject.mockResolvedValue({ id: 'project-1', key: 'CORE', name: 'CoreLog' })
    repositories.events.mockResolvedValue([
      { kind: 'issue_due', date: '2026-11-05', title: 'Entregar exportacion', sourceId: 'issue-1', link: '/issues/issue-1' },
      { kind: 'sprint_start', date: '2026-11-02', title: 'Sprint 1', sourceId: 'sprint-1', link: '/projects/project-1/sprints' },
    ])
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('shows the month grid with each event on its day', async () => {
    renderPage()

    expect(await screen.findByText('Entregar exportacion')).toBeInTheDocument()
    expect(screen.getByRole('gridcell', { name: /^5 de/ })).toHaveTextContent('Entregar exportacion')
    expect(screen.getByRole('gridcell', { name: /^2 de/ })).toHaveTextContent('Inicio de sprint')
  })

  it('lists the events grouped by date in list view', async () => {
    renderPage()
    await screen.findByText('Entregar exportacion')

    fireEvent.click(screen.getByText('Lista'))

    expect(screen.getByRole('region', { name: '2026-11-05' })).toHaveTextContent('Entregar exportacion')
    expect(screen.getByRole('region', { name: '2026-11-02' })).toHaveTextContent('Sprint 1')
  })

  it('reloads the events when moving to another month', async () => {
    renderPage()
    await screen.findByText('Entregar exportacion')
    const before = repositories.events.mock.calls.length

    fireEvent.click(screen.getByRole('button', { name: 'Mes siguiente' }))

    await waitFor(() => expect(repositories.events.mock.calls.length).toBeGreaterThan(before))
  })

  it('shows a useful error when the calendar cannot be loaded', async () => {
    repositories.events.mockRejectedValue(new Error('network'))
    renderPage()

    expect(await screen.findByRole('alert')).toHaveTextContent('No se pudo cargar el calendario')
  })

  it('shows an empty state inside the calendar when the month has no milestones', async () => {
    repositories.events.mockResolvedValue([])
    renderPage()

    expect(await screen.findByText('No hay hitos en este mes')).toBeInTheDocument()
    consoleGuard.expectNoConsoleOutput()
  })

  it('shows the 403 page when the calendar is forbidden', async () => {
    repositories.events.mockRejectedValue(new AppError({ status: 403, message: 'forbidden' }))
    renderPage()

    expect(await screen.findByText('No tienes acceso a esta página')).toBeInTheDocument()
    consoleGuard.expectNoConsoleOutput()
  })
})
