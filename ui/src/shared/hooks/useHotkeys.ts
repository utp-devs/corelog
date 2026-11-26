import { useEffect, useRef, useState, useSyncExternalStore } from 'react'

/** Time allowed between the two keys of a sequence such as "g b". */
export const SEQUENCE_TIMEOUT_MS = 1000

export type HotkeyBinding = {
  /** A single key ('c', '/', '?') or a two-key sequence separated by one space ('g b'). */
  keys: string
  /** Spanish text shown in the shortcuts dialog. */
  description: string
  /** Heading of the shortcuts dialog section. */
  group: string
  /** Listed in the dialog as "Próximamente" and never fired. */
  comingSoon?: boolean
  run?: (event: KeyboardEvent) => void
}

export type RegisteredHotkey = Pick<HotkeyBinding, 'keys' | 'description' | 'group' | 'comingSoon'>

type Listener = () => void

const registry = new Map<symbol, RegisteredHotkey[]>()
const listeners = new Set<Listener>()
let snapshot: readonly RegisteredHotkey[] = []

function publish(): void {
  snapshot = Array.from(registry.values()).flat()
  listeners.forEach((listener) => listener())
}

function subscribe(listener: Listener): () => void {
  listeners.add(listener)
  return () => {
    listeners.delete(listener)
  }
}

/** Every shortcut currently mounted, in registration order. Used by ShortcutsDialog. */
export function useRegisteredHotkeys(): readonly RegisteredHotkey[] {
  return useSyncExternalStore(subscribe, () => snapshot, () => snapshot)
}

// Un atajo no se dispara cuando el foco esta en un campo editable.
export function isEditableTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false
  if (target.isContentEditable || target.closest('[contenteditable]:not([contenteditable="false"])')) return true
  return ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName)
}

/**
 * Registers keyboard bindings on window. Single keys fire immediately; sequences fire when the
 * second key arrives within SEQUENCE_TIMEOUT_MS. Nothing fires while typing in a field, and
 * bindings with Ctrl, Meta or Alt are ignored.
 */
export function useHotkeys(bindings: readonly HotkeyBinding[]): void {
  const [instance] = useState(() => Symbol('hotkeys'))
  const latest = useRef(bindings)
  const pending = useRef<{ prefix: string; timer: number } | null>(null)

  useEffect(() => {
    latest.current = bindings
  })

  // Serialized metadata keeps the registry stable when the caller passes a new array with the same content.
  const meta = JSON.stringify(
    bindings.map(({ keys, description, group, comingSoon }) => ({ keys, description, group, comingSoon: comingSoon === true })),
  )

  useEffect(() => {
    registry.set(instance, JSON.parse(meta) as RegisteredHotkey[])
    publish()
    return () => {
      registry.delete(instance)
      publish()
    }
  }, [instance, meta])

  useEffect(() => {
    const clearPending = () => {
      if (pending.current) {
        window.clearTimeout(pending.current.timer)
        pending.current = null
      }
    }

    const onKeyDown = (event: KeyboardEvent) => {
      if (event.ctrlKey || event.metaKey || event.altKey || event.isComposing || isEditableTarget(event.target)) {
        clearPending()
        return
      }

      const key = event.key
      const active = latest.current.filter((binding) => !binding.comingSoon && binding.run)
      const prefix = pending.current?.prefix
      clearPending()

      if (prefix !== undefined) {
        const sequence = active.find((binding) => binding.keys === `${prefix} ${key}`)
        if (sequence?.run) {
          event.preventDefault()
          sequence.run(event)
          return
        }
      }

      const single = active.find((binding) => binding.keys === key)
      if (single?.run) {
        event.preventDefault()
        single.run(event)
        return
      }

      if (active.some((binding) => binding.keys.startsWith(`${key} `))) {
        pending.current = {
          prefix: key,
          timer: window.setTimeout(clearPending, SEQUENCE_TIMEOUT_MS),
        }
      }
    }

    window.addEventListener('keydown', onKeyDown)
    return () => {
      window.removeEventListener('keydown', onKeyDown)
      clearPending()
    }
  }, [])
}
