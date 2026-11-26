import { useSyncExternalStore } from 'react'

type PaletteState = {
  open: boolean
  /** Text typed in the header search field, handed to the palette when it opens. */
  seed: string
}

type Listener = () => void

let state: PaletteState = { open: false, seed: '' }
const listeners = new Set<Listener>()

function setState(next: PaletteState): void {
  state = next
  listeners.forEach((listener) => listener())
}

function subscribe(listener: Listener): () => void {
  listeners.add(listener)
  return () => {
    listeners.delete(listener)
  }
}

export function openPalette(seed = ''): void {
  setState({ open: true, seed })
}

export function closePalette(): void {
  if (!state.open) return
  setState({ open: false, seed: '' })
}

export function togglePalette(): void {
  if (state.open) closePalette()
  else openPalette()
}

export function getPaletteState(): PaletteState {
  return state
}

export function usePaletteState(): PaletteState {
  return useSyncExternalStore(subscribe, getPaletteState, getPaletteState)
}
