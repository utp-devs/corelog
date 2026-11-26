import { useCallback, useEffect, useState } from 'react'

/** Same key and event as adapters/in/ui/layout/preferences.ts: density changes reach every open table. */
const DENSITY_KEY = 'corelog.density'
const DENSITY_EVENT = 'corelog:density'

function readStored<T>(storageKey: string, fallback: T): T {
  try {
    const raw = window.localStorage.getItem(storageKey)
    return raw === null ? fallback : (JSON.parse(raw) as T)
  } catch {
    return fallback
  }
}

/** Estado persistido en localStorage como JSON. Si el valor guardado no se puede leer, usa el inicial. */
export function usePersistentState<T>(storageKey: string, initial: T): [T, (next: T) => void] {
  const [value, setValue] = useState<T>(() => readStored(storageKey, initial))

  useEffect(() => {
    if (storageKey !== DENSITY_KEY) return
    const onDensity = (event: Event) => {
      setValue((event as CustomEvent<T>).detail)
    }
    window.addEventListener(DENSITY_EVENT, onDensity)
    return () => window.removeEventListener(DENSITY_EVENT, onDensity)
  }, [storageKey])

  const update = useCallback(
    (next: T) => {
      setValue(next)
      try {
        window.localStorage.setItem(storageKey, JSON.stringify(next))
      } catch {
        // Almacenamiento no disponible (modo privado o cuota): el estado sigue en memoria.
      }
      if (storageKey === DENSITY_KEY) {
        window.dispatchEvent(new CustomEvent<T>(DENSITY_EVENT, { detail: next }))
      }
    },
    [storageKey],
  )

  return [value, update]
}
