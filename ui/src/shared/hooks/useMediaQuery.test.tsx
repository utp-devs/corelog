import { act, renderHook } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { useBreakpoint, useMediaQuery } from './useMediaQuery'

type Listener = () => void

// Fake matchMedia: each query has its own state and listeners, so tests can flip it.
function installMatchMedia(initial: Record<string, boolean>) {
  const state = { ...initial }
  const listeners = new Map<string, Set<Listener>>()
  const original = window.matchMedia
  window.matchMedia = vi.fn((query: string) => ({
    get matches() {
      return state[query] ?? false
    },
    media: query,
    onchange: null,
    addListener: () => {},
    removeListener: () => {},
    addEventListener: (_type: string, cb: Listener) => {
      if (!listeners.has(query)) listeners.set(query, new Set())
      listeners.get(query)!.add(cb)
    },
    removeEventListener: (_type: string, cb: Listener) => {
      listeners.get(query)?.delete(cb)
    },
    dispatchEvent: () => false,
  })) as unknown as typeof window.matchMedia
  return {
    set(query: string, matches: boolean) {
      state[query] = matches
      listeners.get(query)?.forEach((cb) => cb())
    },
    restore() {
      window.matchMedia = original
    },
  }
}

describe('useMediaQuery', () => {
  let mm: ReturnType<typeof installMatchMedia>

  beforeEach(() => {
    mm = installMatchMedia({ '(max-width: 767.98px)': true })
  })

  afterEach(() => {
    mm.restore()
  })

  it('returns the current match of the query', () => {
    const { result } = renderHook(() => useMediaQuery('(max-width: 767.98px)'))
    expect(result.current).toBe(true)
  })

  it('updates when the media query changes', () => {
    const { result } = renderHook(() => useMediaQuery('(max-width: 767.98px)'))
    act(() => mm.set('(max-width: 767.98px)', false))
    expect(result.current).toBe(false)
  })

  it('returns false when matchMedia is unavailable (SSR-like environment)', () => {
    const original = window.matchMedia
    // @ts-expect-error simulating an environment without matchMedia
    window.matchMedia = undefined
    try {
      const { result } = renderHook(() => useMediaQuery('(min-width: 1px)'))
      expect(result.current).toBe(false)
    } finally {
      window.matchMedia = original
    }
  })
})

describe('useBreakpoint', () => {
  let mm: ReturnType<typeof installMatchMedia>

  afterEach(() => {
    mm.restore()
  })

  it('reports every breakpoint reached by the viewport (min-width semantics)', () => {
    mm = installMatchMedia({
      '(min-width: 576px)': true,
      '(min-width: 768px)': true,
      '(min-width: 992px)': true,
      '(min-width: 1280px)': false,
      '(min-width: 1680px)': false,
    })
    const { result } = renderHook(() => useBreakpoint())
    expect(result.current).toEqual({ xs: true, md: true, lg: true, xl: false, xxl: false })
  })

  it('reports all false for a narrow phone viewport', () => {
    mm = installMatchMedia({})
    const { result } = renderHook(() => useBreakpoint())
    expect(result.current).toEqual({ xs: false, md: false, lg: false, xl: false, xxl: false })
  })

  it('reports xxl for a wide desktop and updates on resize', () => {
    mm = installMatchMedia({ '(min-width: 1680px)': true })
    const { result } = renderHook(() => useBreakpoint())
    expect(result.current.xxl).toBe(true)
    act(() => mm.set('(min-width: 1680px)', false))
    expect(result.current.xxl).toBe(false)
  })
})
