import { act, fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { clearCommandRegistry } from './commandRegistry'
import { CommandPaletteHost } from './CommandPaletteHost'
import { PALETTE_DEBOUNCE_MS } from './CommandPalette'
import { closePalette } from './paletteState'

const repositories = vi.hoisted(() => ({ search: vi.fn() }))

vi.mock('@/adapters/out/http/search-http-repository', () => ({
  SearchHttpRepository: class {
    search = repositories.search
  },
}))

function Where() {
  const location = useLocation()
  return <p>Ruta: {location.pathname}</p>
}

function renderHost() {
  return render(
    <MemoryRouter initialEntries={['/dashboard']}>
      <CommandPaletteHost />
      <Routes>
        <Route path="*" element={<Where />} />
      </Routes>
    </MemoryRouter>,
  )
}

describe('CommandPaletteHost', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    window.localStorage.clear()
    repositories.search.mockResolvedValue([
      { kind: 'issue', id: 'i-9', title: 'Revisar login', subtitle: 'CORE-9', link: '/issues/i-9' },
    ])
  })

  afterEach(() => {
    closePalette()
    clearCommandRegistry()
    vi.useRealTimers()
  })

  it('opens with Ctrl+K and closes with Escape', () => {
    renderHost()
    expect(screen.queryByRole('dialog', { name: 'Paleta de comandos' })).not.toBeInTheDocument()
    fireEvent.keyDown(window, { key: 'k', ctrlKey: true })
    expect(screen.getByRole('dialog', { name: 'Paleta de comandos' })).toBeInTheDocument()
    fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Escape' })
    expect(screen.queryByRole('dialog', { name: 'Paleta de comandos' })).not.toBeInTheDocument()
  })

  it('opens with "/" when no field is focused', () => {
    renderHost()
    fireEvent.keyDown(window, { key: '/' })
    expect(screen.getByRole('dialog', { name: 'Paleta de comandos' })).toBeInTheDocument()
  })

  it('navigates to the registered built-in command on Enter', () => {
    renderHost()
    fireEvent.keyDown(window, { key: 'k', ctrlKey: true })
    fireEvent.click(screen.getByRole('option', { name: /Ir a Incidencias/ }))
    expect(screen.getByText('Ruta: /issues')).toBeInTheDocument()
    expect(screen.queryByRole('dialog', { name: 'Paleta de comandos' })).not.toBeInTheDocument()
  })

  it('navigates to the hit link when a remote result is chosen with Enter', async () => {
    vi.useFakeTimers()
    renderHost()
    fireEvent.keyDown(window, { key: 'k', ctrlKey: true })
    fireEvent.change(screen.getByRole('combobox'), { target: { value: 'revisar' } })
    await act(async () => {
      vi.advanceTimersByTime(PALETTE_DEBOUNCE_MS)
      await Promise.resolve()
    })
    const hit = screen.getByRole('option', { name: /Revisar login/ })
    fireEvent.mouseEnter(hit)
    fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Enter' })
    expect(screen.getByText('Ruta: /issues/i-9')).toBeInTheDocument()
    expect(repositories.search).toHaveBeenCalledWith('revisar')
  })
})
