import { useEffect, useState } from 'react'

/** Same key and format as the DataTable density choice (JSON string), so both stay in step. */
export const DENSITY_STORAGE_KEY = 'corelog.density'
/** Sidebar state: 'collapsed' or 'expanded'. Absent means "follow the viewport width". */
export const SIDEBAR_STORAGE_KEY = 'corelog.sidebar'

export type Density = 'comfortable' | 'compact'

/** Window event dispatched on every density change so open tables follow the choice at once. */
export const DENSITY_EVENT = 'corelog:density'

function isDensity(value: unknown): value is Density {
  return value === 'comfortable' || value === 'compact'
}

export function readDensity(): Density {
  try {
    const parsed: unknown = JSON.parse(window.localStorage.getItem(DENSITY_STORAGE_KEY) ?? 'null')
    return isDensity(parsed) ? parsed : 'comfortable'
  } catch {
    return 'comfortable'
  }
}

export function writeDensity(density: Density): void {
  try {
    window.localStorage.setItem(DENSITY_STORAGE_KEY, JSON.stringify(density))
  } catch {
    // Storage can be unavailable (private mode); the choice then lasts for the session only.
  }
  window.dispatchEvent(new CustomEvent<Density>(DENSITY_EVENT, { detail: density }))
}

/** Stored sidebar choice, or null when the user has not chosen yet. */
export function readSidebarCollapsed(): boolean | null {
  try {
    const stored = window.localStorage.getItem(SIDEBAR_STORAGE_KEY)
    if (stored === 'collapsed') return true
    if (stored === 'expanded') return false
    return null
  } catch {
    return null
  }
}

export function writeSidebarCollapsed(collapsed: boolean): void {
  try {
    window.localStorage.setItem(SIDEBAR_STORAGE_KEY, collapsed ? 'collapsed' : 'expanded')
  } catch {
    // Same fallback as density: keep the choice in memory only.
  }
}

/** Density toggle state; applies data-density on <html> and persists the choice. */
export function useDensity(): [Density, (density: Density) => void] {
  const [density, setDensityState] = useState<Density>(readDensity)

  useEffect(() => {
    document.documentElement.dataset.density = density
  }, [density])

  useEffect(() => {
    const onChange = (event: Event) => {
      const next = (event as CustomEvent<unknown>).detail
      if (isDensity(next)) setDensityState(next)
    }
    window.addEventListener(DENSITY_EVENT, onChange)
    return () => window.removeEventListener(DENSITY_EVENT, onChange)
  }, [])

  const setDensity = (next: Density) => {
    writeDensity(next)
    setDensityState(next)
  }

  return [density, setDensity]
}
