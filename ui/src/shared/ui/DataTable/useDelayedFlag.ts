import { useEffect, useState } from 'react'

/** Devuelve true solo si `active` se mantiene durante `delayMs`. Evita parpadeos en cargas rápidas. */
export function useDelayedFlag(active: boolean, delayMs: number): boolean {
  const [elapsed, setElapsed] = useState(false)

  useEffect(() => {
    if (!active) {
      return
    }
    const timer = window.setTimeout(() => setElapsed(true), delayMs)
    return () => {
      window.clearTimeout(timer)
      setElapsed(false)
    }
  }, [active, delayMs])

  return active && elapsed
}
