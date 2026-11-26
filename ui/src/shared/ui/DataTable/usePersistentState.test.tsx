import { act, renderHook } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'

import { usePersistentState } from './usePersistentState'

describe('usePersistentState', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  it('keeps two tables with the same density key in step without a reload', () => {
    const first = renderHook(() => usePersistentState<'comfortable' | 'compact'>('corelog.density', 'comfortable'))
    const second = renderHook(() => usePersistentState<'comfortable' | 'compact'>('corelog.density', 'comfortable'))

    act(() => first.result.current[1]('compact'))
    expect(first.result.current[0]).toBe('compact')
    expect(second.result.current[0]).toBe('compact')
  })

  it('follows density changes made by the layout preferences', () => {
    const { result } = renderHook(() => usePersistentState<'comfortable' | 'compact'>('corelog.density', 'comfortable'))
    act(() => {
      window.dispatchEvent(new CustomEvent('corelog:density', { detail: 'compact' }))
    })
    expect(result.current[0]).toBe('compact')
  })

  it('ignores density events for other keys', () => {
    const { result } = renderHook(() => usePersistentState<string>('corelog.other', 'a'))
    act(() => {
      window.dispatchEvent(new CustomEvent('corelog:density', { detail: 'compact' }))
    })
    expect(result.current[0]).toBe('a')
  })
})
