import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'

import { ConfirmModal } from './ConfirmModal'

describe('ConfirmModal', () => {
  it('shows the title and the consequence description when open', () => {
    render(
      <ConfirmModal
        open
        title="Eliminar proyecto"
        description="Se borrarán todas sus incidencias."
        onConfirm={() => {}}
        onCancel={() => {}}
      />,
    )
    expect(screen.getByText('Eliminar proyecto')).toBeInTheDocument()
    expect(screen.getByText('Se borrarán todas sus incidencias.')).toBeInTheDocument()
  })

  it('calls onConfirm when the confirm button is pressed', () => {
    const onConfirm = vi.fn()
    render(<ConfirmModal open title="Confirmar acción" confirmText="Sí, eliminar" onConfirm={onConfirm} onCancel={() => {}} />)
    fireEvent.click(screen.getByRole('button', { name: 'Sí, eliminar' }))
    expect(onConfirm).toHaveBeenCalledTimes(1)
  })

  it('calls onCancel when the cancel button is pressed', () => {
    const onCancel = vi.fn()
    render(<ConfirmModal open title="Confirmar acción" onConfirm={() => {}} onCancel={onCancel} />)
    fireEvent.click(screen.getByRole('button', { name: 'Cancelar' }))
    expect(onCancel).toHaveBeenCalledTimes(1)
  })
})
