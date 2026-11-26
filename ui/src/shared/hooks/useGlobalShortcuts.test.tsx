import { act, renderHook } from '@testing-library/react'
import type { ReactNode } from 'react'
import { MemoryRouter, useLocation } from 'react-router-dom'
import { describe, expect, it } from 'vitest'

import { useGlobalShortcuts } from './useGlobalShortcuts'

function wrapper({ children }: { children: ReactNode }) {
  return <MemoryRouter initialEntries={['/issues']}>{children}</MemoryRouter>
}

function useShortcutsAndLocation() {
  useGlobalShortcuts()
  return useLocation().pathname
}

describe('useGlobalShortcuts', () => {
  it('navega a la creacion de incidencia con la tecla c', () => {
    const { result } = renderHook(() => useShortcutsAndLocation(), { wrapper })
    act(() => {
      window.dispatchEvent(new KeyboardEvent('keydown', { key: 'c' }))
    })
    expect(result.current).toBe('/issues/new')
  })

  it('no dispara los atajos mientras se escribe en un campo', () => {
    const { result } = renderHook(() => useShortcutsAndLocation(), { wrapper })
    const input = document.createElement('input')
    document.body.appendChild(input)
    act(() => {
      input.dispatchEvent(new KeyboardEvent('keydown', { key: 'c', bubbles: true }))
    })
    expect(result.current).toBe('/issues')
    input.remove()
  })

  it('no longer binds the slash key, which the command palette host owns', () => {
    const search = document.createElement('input')
    search.setAttribute('aria-label', 'Buscar proyectos e incidencias')
    document.body.appendChild(search)
    const { result } = renderHook(() => useShortcutsAndLocation(), { wrapper })
    act(() => {
      window.dispatchEvent(new KeyboardEvent('keydown', { key: '/' }))
    })
    expect(document.activeElement).not.toBe(search)
    expect(result.current).toBe('/issues')
    search.remove()
  })
})
