import { useSyncExternalStore } from 'react'

/** Sections of the command list. Search hits use their own sections (Proyectos, Incidencias). */
export type CommandGroup = 'Acciones' | 'Navegación'

export type Command = {
  id: string
  /** Spanish label shown in the palette. */
  title: string
  /** Extra words that match the query (synonyms, English names). */
  keywords?: readonly string[]
  group: CommandGroup
  run: () => void
}

type Listener = () => void

// Insertion order is the display order. Re-registering an id replaces the command in place.
const commands = new Map<string, Command>()
const listeners = new Set<Listener>()
let snapshot: readonly Command[] = []

function publish(): void {
  snapshot = Array.from(commands.values())
  listeners.forEach((listener) => listener())
}

/**
 * Adds a command to the palette registry. The returned function removes it, but only if the
 * stored command is still this one, so a late cleanup cannot remove a newer registration.
 */
export function registerCommand(command: Command): () => void {
  commands.set(command.id, command)
  publish()
  return () => {
    if (commands.get(command.id) !== command) return
    commands.delete(command.id)
    publish()
  }
}

export function getCommands(): readonly Command[] {
  return snapshot
}

export function subscribeCommands(listener: Listener): () => void {
  listeners.add(listener)
  return () => {
    listeners.delete(listener)
  }
}

/** Every command currently registered, in registration order. */
export function useCommands(): readonly Command[] {
  return useSyncExternalStore(subscribeCommands, getCommands, getCommands)
}

/** Test helper: empties the registry between tests. */
export function clearCommandRegistry(): void {
  commands.clear()
  publish()
}
