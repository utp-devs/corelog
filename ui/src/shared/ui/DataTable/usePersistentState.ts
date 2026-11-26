import { useCallback, useState } from 'react'

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

  const update = useCallback(
    (next: T) => {
      setValue(next)
      try {
        window.localStorage.setItem(storageKey, JSON.stringify(next))
      } catch {
        // Almacenamiento no disponible (modo privado o cuota): el estado sigue en memoria.
      }
    },
    [storageKey],
  )

  return [value, update]
}
