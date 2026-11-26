import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'

import { ConfirmDangerModal } from './ConfirmDangerModal'
import { isConfirmationMatch } from './confirmation'

describe('isConfirmationMatch', () => {
  it('requires an exact, non-empty match', () => {
    expect(isConfirmationMatch('CoreLog', 'CoreLog')).toBe(true)
    expect(isConfirmationMatch('corelog', 'CoreLog')).toBe(false)
    expect(isConfirmationMatch('CoreLog ', 'CoreLog')).toBe(false)
    expect(isConfirmationMatch('', '')).toBe(false)
  })
})

describe('ConfirmDangerModal', () => {
  it('keeps the danger button disabled until the exact name is typed', () => {
    render(<ConfirmDangerModal open title="Eliminar proyecto" confirmation="Proyecto Alfa" onConfirm={() => {}} onCancel={() => {}} />)
    const confirm = screen.getByRole('button', { name: 'Eliminar' })
    expect(confirm).toBeDisabled()

    const input = screen.getByLabelText(/para confirmar/)
    fireEvent.change(input, { target: { value: 'proyecto alfa' } })
    expect(confirm).toBeDisabled()

    fireEvent.change(input, { target: { value: 'Proyecto Alfa' } })
    expect(confirm).toBeEnabled()
  })

  it('calls onConfirm only once the name matches', () => {
    const onConfirm = vi.fn()
    render(<ConfirmDangerModal open title="Eliminar organización" confirmation="Acme" onConfirm={onConfirm} onCancel={() => {}} />)
    fireEvent.click(screen.getByRole('button', { name: 'Eliminar' }))
    expect(onConfirm).not.toHaveBeenCalled()
    fireEvent.change(screen.getByLabelText(/para confirmar/), { target: { value: 'Acme' } })
    fireEvent.click(screen.getByRole('button', { name: 'Eliminar' }))
    expect(onConfirm).toHaveBeenCalledTimes(1)
  })

  it('calls onCancel from the cancel button', () => {
    const onCancel = vi.fn()
    render(<ConfirmDangerModal open title="Eliminar" confirmation="X" onConfirm={() => {}} onCancel={onCancel} />)
    fireEvent.click(screen.getByRole('button', { name: 'Cancelar' }))
    expect(onCancel).toHaveBeenCalledTimes(1)
  })
})
