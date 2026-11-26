import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import type { SearchHit } from '@/shared/types/search'
import { CommandPalette, PALETTE_DEBOUNCE_MS, type CommandPaletteProps } from './CommandPalette'
import { clearCommandRegistry, registerCommand } from './commandRegistry'
import { RECENT_STORAGE_KEY } from './recent'

const hits: SearchHit[] = [
  { kind: 'project', id: 'p-1', title: 'Proyecto Core', subtitle: 'CORE', link: '/projects/p-1' },
  { kind: 'issue', id: 'i-1', title: 'Exportar datos', subtitle: 'CORE-12', link: '/issues/i-1' },
]

function setup(overrides: Partial<CommandPaletteProps> = {}) {
  const props: CommandPaletteProps = {
    open: true,
    onClose: vi.fn(),
    search: vi.fn().mockResolvedValue(hits),
    onOpenHit: vi.fn(),
    ...overrides,
  }
  const view = render(<CommandPalette {...props} />)
  const input = screen.getByRole('combobox', { name: 'Buscar o escribir un comando' })
  return { props, view, input }
}

function type(input: HTMLElement, value: string) {
  fireEvent.change(input, { target: { value } })
}

describe('CommandPalette', () => {
  beforeEach(() => {
    window.localStorage.clear()
    registerCommand({ id: 'nav-issues', title: 'Ir a Incidencias', group: 'Navegación', run: vi.fn() })
    registerCommand({ id: 'act-theme', title: 'Cambiar tema', keywords: ['oscuro'], group: 'Acciones', run: vi.fn() })
  })

  afterEach(() => {
    clearCommandRegistry()
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  it('renders nothing while closed', () => {
    render(<CommandPalette open={false} onClose={vi.fn()} search={vi.fn()} onOpenHit={vi.fn()} />)
    expect(screen.queryByRole('dialog', { name: 'Paleta de comandos' })).not.toBeInTheDocument()
  })

  it('lists commands grouped by section and marks the first option active', () => {
    setup()
    const listbox = screen.getByRole('listbox', { name: 'Resultados' })
    expect(within(listbox).getByRole('group', { name: 'Acciones' })).toHaveTextContent('Cambiar tema')
    expect(within(listbox).getByRole('group', { name: 'Navegación' })).toHaveTextContent('Ir a Incidencias')
    const options = screen.getAllByRole('option')
    expect(options[0]).toHaveAttribute('aria-selected', 'true')
  })

  it('moves the active option with Up and Down and points aria-activedescendant at it', () => {
    const { input } = setup()
    const options = screen.getAllByRole('option')
    fireEvent.keyDown(screen.getByRole('dialog'), { key: 'ArrowDown' })
    expect(options[1]).toHaveAttribute('aria-selected', 'true')
    expect(input).toHaveAttribute('aria-activedescendant', options[1].id)
    fireEvent.keyDown(screen.getByRole('dialog'), { key: 'ArrowUp' })
    fireEvent.keyDown(screen.getByRole('dialog'), { key: 'ArrowUp' })
    expect(options[1]).toHaveAttribute('aria-selected', 'true')
  })

  it('runs the active command on Enter and closes first', () => {
    const run = vi.fn()
    clearCommandRegistry()
    registerCommand({ id: 'only', title: 'Solo comando', group: 'Acciones', run })
    const { props } = setup()
    fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Enter' })
    expect(props.onClose).toHaveBeenCalledTimes(1)
    expect(run).toHaveBeenCalledTimes(1)
  })

  it('closes on Escape', () => {
    const { props } = setup()
    fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Escape' })
    expect(props.onClose).toHaveBeenCalledTimes(1)
  })

  it('filters commands by title and keywords', () => {
    setup()
    type(screen.getByRole('combobox'), 'oscuro')
    expect(screen.getByRole('option', { name: /Cambiar tema/ })).toBeInTheDocument()
    expect(screen.queryByRole('option', { name: /Ir a Incidencias/ })).not.toBeInTheDocument()
  })

  it('shows the hint for a single character and does not search', () => {
    vi.useFakeTimers()
    const { props } = setup()
    type(screen.getByRole('combobox'), 'e')
    act(() => {
      vi.advanceTimersByTime(PALETTE_DEBOUNCE_MS * 2)
    })
    expect(props.search).not.toHaveBeenCalled()
    expect(screen.getByText(/Escribe al menos 2 caracteres/)).toBeInTheDocument()
  })

  it('debounces the remote search by 150 ms and shows grouped hits', async () => {
    vi.useFakeTimers()
    const { props } = setup()
    const input = screen.getByRole('combobox')
    type(input, 'ex')
    type(input, 'exp')
    type(input, 'expo')
    expect(props.search).not.toHaveBeenCalled()
    await act(async () => {
      vi.advanceTimersByTime(PALETTE_DEBOUNCE_MS - 1)
    })
    expect(props.search).not.toHaveBeenCalled()
    await act(async () => {
      vi.advanceTimersByTime(1)
    })
    expect(props.search).toHaveBeenCalledTimes(1)
    expect(props.search).toHaveBeenCalledWith('expo')
    await act(async () => {
      await Promise.resolve()
    })
    expect(screen.getByRole('group', { name: 'Proyectos' })).toHaveTextContent('Proyecto Core')
    expect(screen.getByRole('group', { name: 'Incidencias' })).toHaveTextContent('Exportar datos')
  })

  it('shows the loading state while the search is pending', async () => {
    vi.useFakeTimers()
    setup({ search: vi.fn(() => new Promise<SearchHit[]>(() => {})) })
    type(screen.getByRole('combobox'), 'ex')
    await act(async () => {
      vi.advanceTimersByTime(PALETTE_DEBOUNCE_MS)
    })
    expect(screen.getByRole('status')).toHaveTextContent('Buscando…')
  })

  it('shows the empty state when nothing matches', async () => {
    setup({ search: vi.fn().mockResolvedValue([]) })
    type(screen.getByRole('combobox'), 'zzz')
    expect(await screen.findByText('Sin resultados')).toBeInTheDocument()
  })

  it('shows the error state with a retry that searches again', async () => {
    const search = vi.fn().mockRejectedValueOnce(new Error('boom')).mockResolvedValueOnce(hits)
    setup({ search })
    type(screen.getByRole('combobox'), 'ex')
    expect(await screen.findByText(/No se pudo buscar/)).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: /Reintentar/ }))
    expect(await screen.findByText('Exportar datos')).toBeInTheDocument()
    expect(search).toHaveBeenCalledTimes(2)
  })

  it('lists recent items from localStorage and opens them with Enter', async () => {
    window.localStorage.setItem(
      RECENT_STORAGE_KEY,
      JSON.stringify([{ kind: 'issue', id: 'r-1', title: 'Reciente X', subtitle: 'CORE-3', link: '/issues/r-1' }]),
    )
    const { props } = setup()
    expect(screen.getByRole('group', { name: 'Recientes' })).toHaveTextContent('Reciente X')
    const recentOption = screen.getByRole('option', { name: /Reciente X/ })
    fireEvent.mouseEnter(recentOption)
    fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Enter' })
    expect(props.onOpenHit).toHaveBeenCalledWith(expect.objectContaining({ id: 'r-1', link: '/issues/r-1' }))
  })

  it('moves focus into the input on open and restores the previous element on close', () => {
    const trigger = document.createElement('button')
    trigger.textContent = 'Abrir'
    document.body.appendChild(trigger)
    trigger.focus()

    const { view, props } = setup()
    expect(screen.getByRole('combobox')).toHaveFocus()

    view.rerender(<CommandPalette {...props} open={false} />)
    expect(trigger).toHaveFocus()
    trigger.remove()
  })

  it('keeps Tab inside the dialog', async () => {
    setup({ search: vi.fn().mockRejectedValue(new Error('x')) })
    type(screen.getByRole('combobox'), 'ex')
    const retry = await screen.findByRole('button', { name: /Reintentar/ })
    retry.focus()
    fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Tab' })
    expect(screen.getByRole('combobox')).toHaveFocus()
  })

  it('closes when the backdrop is clicked', () => {
    const { props } = setup()
    fireEvent.mouseDown(screen.getByRole('dialog').parentElement as HTMLElement)
    expect(props.onClose).toHaveBeenCalled()
  })

  it('pulls focus back when something outside the dialog takes it', async () => {
    const outside = document.createElement('input')
    document.body.appendChild(outside)
    setup()
    outside.focus()
    await waitFor(() => expect(screen.getByRole('combobox')).toHaveFocus())
    outside.remove()
  })
})
