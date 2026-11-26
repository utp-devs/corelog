import { beforeEach, describe, expect, it } from 'vitest'

import { readDensity, readSidebarCollapsed, writeDensity, writeSidebarCollapsed } from './preferences'

describe('layout preferences', () => {
  beforeEach(() => {
    window.localStorage.clear()
  })

  it('defaults density to comfortable and reads the DataTable format', () => {
    expect(readDensity()).toBe('comfortable')
    window.localStorage.setItem('corelog.density', '"compact"')
    expect(readDensity()).toBe('compact')
  })

  it('ignores unknown density values', () => {
    window.localStorage.setItem('corelog.density', '"huge"')
    expect(readDensity()).toBe('comfortable')
  })

  it('writes density as JSON so DataTable can read it', () => {
    writeDensity('compact')
    expect(window.localStorage.getItem('corelog.density')).toBe('"compact"')
  })

  it('returns null for sidebar state until the user chooses', () => {
    expect(readSidebarCollapsed()).toBeNull()
    writeSidebarCollapsed(true)
    expect(readSidebarCollapsed()).toBe(true)
    writeSidebarCollapsed(false)
    expect(readSidebarCollapsed()).toBe(false)
  })
})
