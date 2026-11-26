import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'

import { useHotkeys } from '@/shared/hooks/useHotkeys'

import { ShortcutsDialog } from './ShortcutsDialog'

function Registered() {
  useHotkeys([
    { keys: 'c', description: 'Crear incidencia', group: 'General', run: () => undefined },
    { keys: 'g b', description: 'Ir al tablero del proyecto', group: 'Navegación', run: () => undefined },
    { keys: 'n', description: 'Notas rápidas', group: 'General', comingSoon: true },
  ])
  return null
}

describe('ShortcutsDialog', () => {
  it('lists registered shortcuts grouped with their keys', () => {
    render(
      <>
        <Registered />
        <ShortcutsDialog open onClose={() => undefined} />
      </>,
    )
    expect(screen.getByRole('dialog', { name: 'Atajos de teclado' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'General' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Navegación' })).toBeInTheDocument()
    expect(screen.getByText('Crear incidencia')).toBeInTheDocument()
    expect(screen.getByText('luego')).toBeInTheDocument()
    expect(screen.getByText('Notas rápidas').parentElement).toHaveTextContent('Próximamente')
  })

  it('shows an empty message when nothing is registered', () => {
    render(<ShortcutsDialog open onClose={() => undefined} />)
    expect(screen.getByText('No hay atajos disponibles en esta pantalla.')).toBeInTheDocument()
  })

  it('calls onClose from the dialog close button', () => {
    const onClose = vi.fn()
    render(<ShortcutsDialog open onClose={onClose} />)
    fireEvent.click(screen.getByRole('button', { name: 'Close' }))
    expect(onClose).toHaveBeenCalled()
  })

  it('stays closed when open is false', () => {
    render(<ShortcutsDialog open={false} onClose={() => undefined} />)
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })
})
