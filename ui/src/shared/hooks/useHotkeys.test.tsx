import { act, renderHook } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { SEQUENCE_TIMEOUT_MS, useHotkeys, useRegisteredHotkeys, type HotkeyBinding } from './useHotkeys'

function press(key: string, init: KeyboardEventInit = {}, target: EventTarget = window) {
  act(() => {
    target.dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true, ...init }))
  })
}

describe('useHotkeys', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('fires a single key binding', () => {
    const run = vi.fn()
    renderHook(() => useHotkeys([{ keys: '?', description: 'Ayuda', group: 'General', run }]))
    press('?')
    expect(run).toHaveBeenCalledTimes(1)
  })

  it('fires a sequence when the second key arrives in time', () => {
    const run = vi.fn()
    renderHook(() => useHotkeys([{ keys: 'g b', description: 'Tablero', group: 'Nav', run }]))
    press('g')
    act(() => {
      vi.advanceTimersByTime(SEQUENCE_TIMEOUT_MS - 100)
    })
    press('b')
    expect(run).toHaveBeenCalledTimes(1)
  })

  it('drops a sequence after the timeout', () => {
    const run = vi.fn()
    renderHook(() => useHotkeys([{ keys: 'g b', description: 'Tablero', group: 'Nav', run }]))
    press('g')
    act(() => {
      vi.advanceTimersByTime(SEQUENCE_TIMEOUT_MS + 1)
    })
    press('b')
    expect(run).not.toHaveBeenCalled()
  })

  it('ignores keys typed inside inputs, textareas and contenteditable elements', () => {
    const run = vi.fn()
    renderHook(() => useHotkeys([{ keys: 'c', description: 'Crear', group: 'General', run }]))
    const input = document.createElement('input')
    const textarea = document.createElement('textarea')
    const editable = document.createElement('div')
    editable.setAttribute('contenteditable', 'true')
    document.body.append(input, textarea, editable)
    press('c', {}, input)
    press('c', {}, textarea)
    press('c', {}, editable)
    expect(run).not.toHaveBeenCalled()
    input.remove()
    textarea.remove()
    editable.remove()
  })

  it('ignores bindings with Ctrl, Meta or Alt', () => {
    const run = vi.fn()
    renderHook(() => useHotkeys([{ keys: 'c', description: 'Crear', group: 'General', run }]))
    press('c', { ctrlKey: true })
    press('c', { metaKey: true })
    press('c', { altKey: true })
    expect(run).not.toHaveBeenCalled()
  })

  it('never fires coming soon bindings but lists them', () => {
    const run = vi.fn()
    const { result } = renderHook(() => {
      useHotkeys([{ keys: 'n', description: 'Notas', group: 'General', comingSoon: true, run }])
      return useRegisteredHotkeys()
    })
    press('n')
    expect(run).not.toHaveBeenCalled()
    expect(result.current).toContainEqual(expect.objectContaining({ keys: 'n', comingSoon: true }))
  })

  it('removes its bindings from the registry on unmount', () => {
    const bindings: HotkeyBinding[] = [{ keys: 'c', description: 'Crear incidencia', group: 'General', run: () => undefined }]
    const first = renderHook(() => {
      useHotkeys(bindings)
    })
    const registry = renderHook(() => useRegisteredHotkeys())
    expect(registry.result.current.map((b) => b.description)).toContain('Crear incidencia')
    first.unmount()
    expect(registry.result.current.map((b) => b.description)).not.toContain('Crear incidencia')
  })
})
