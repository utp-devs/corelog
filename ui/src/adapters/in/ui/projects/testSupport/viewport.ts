import { vi } from 'vitest'

/**
 * Fakes window.matchMedia for the viewport width. Understands "(max-width: Npx)" and "(min-width: Npx)".
 * Returns the restore function; call it in afterEach.
 */
export function installViewport(width: number): () => void {
  const original = window.matchMedia
  window.matchMedia = vi.fn((query: string) => {
    const max = /max-width:\s*([\d.]+)px/.exec(query)
    const min = /min-width:\s*([\d.]+)px/.exec(query)
    const matches = max ? width <= Number(max[1]) : min ? width >= Number(min[1]) : false
    return {
      matches,
      media: query,
      onchange: null,
      addListener: () => undefined,
      removeListener: () => undefined,
      addEventListener: () => undefined,
      removeEventListener: () => undefined,
      dispatchEvent: () => false,
    }
  }) as unknown as typeof window.matchMedia
  return () => {
    window.matchMedia = original
  }
}
