import { useSyncExternalStore } from 'react'

// Breakpoints in px, compared against the viewport width.
export const BREAKPOINTS = {
  xs: 576,
  md: 768,
  lg: 992,
  xl: 1280,
  xxl: 1680,
} as const

export type Breakpoint = keyof typeof BREAKPOINTS

/** True when the media query matches. SSR-safe: returns false without a window. Updates on change. */
export function useMediaQuery(query: string): boolean {
  return useSyncExternalStore(
    (onChange) => {
      if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return () => {}
      const list = window.matchMedia(query)
      list.addEventListener('change', onChange)
      return () => list.removeEventListener('change', onChange)
    },
    () => {
      if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return false
      return window.matchMedia(query).matches
    },
    () => false,
  )
}

/**
 * Viewport at or above each breakpoint (min-width semantics):
 * xs >= 576, md >= 768, lg >= 992, xl >= 1280, xxl >= 1680.
 */
export function useBreakpoint(): Record<Breakpoint, boolean> {
  const xs = useMediaQuery(`(min-width: ${BREAKPOINTS.xs}px)`)
  const md = useMediaQuery(`(min-width: ${BREAKPOINTS.md}px)`)
  const lg = useMediaQuery(`(min-width: ${BREAKPOINTS.lg}px)`)
  const xl = useMediaQuery(`(min-width: ${BREAKPOINTS.xl}px)`)
  const xxl = useMediaQuery(`(min-width: ${BREAKPOINTS.xxl}px)`)
  return { xs, md, lg, xl, xxl }
}
