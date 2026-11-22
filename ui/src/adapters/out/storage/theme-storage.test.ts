import { beforeEach, describe, expect, it } from 'vitest'

import { getStoredTheme, setStoredTheme } from './theme-storage'

describe('theme-storage', () => {
  beforeEach(() => {
    localStorage.clear()
    document.documentElement.removeAttribute('data-theme')
    document.documentElement.style.colorScheme = ''
  })

  it('defaults to light when no theme stored', () => {
    expect(getStoredTheme()).toBe('light')
  })

  it('persists and retrieves light theme', () => {
    setStoredTheme('light')
    expect(localStorage.getItem('corelog.theme')).toBe('light')
    expect(getStoredTheme()).toBe('light')
    expect(document.documentElement.getAttribute('data-theme')).toBe('light')
    expect(document.documentElement.style.colorScheme).toBe('light')
  })

  it('persists and retrieves dark theme', () => {
    setStoredTheme('dark')
    expect(localStorage.getItem('corelog.theme')).toBe('dark')
    expect(getStoredTheme()).toBe('dark')
    expect(document.documentElement.getAttribute('data-theme')).toBe('dark')
    expect(document.documentElement.style.colorScheme).toBe('dark')
  })
})
