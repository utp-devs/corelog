import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'

import { INLINE_EDIT_SAVE_ERROR, InlineEdit } from './InlineEdit'

function openEditor(label: string, text: string) {
  fireEvent.click(screen.getByRole('button', { name: `Editar ${label}: ${text}` }))
}

describe('InlineEdit', () => {
  it('shows the value and opens the editor on click', () => {
    render(<InlineEdit label="Título" value="Login roto" onSave={() => {}} />)
    openEditor('Título', 'Login roto')
    expect(screen.getByRole('textbox', { name: 'Título' })).toHaveValue('Login roto')
  })

  it('saves the new value on Enter', async () => {
    const onSave = vi.fn()
    render(<InlineEdit label="Título" value="Viejo" onSave={onSave} />)
    openEditor('Título', 'Viejo')
    const input = screen.getByRole('textbox', { name: 'Título' })
    fireEvent.change(input, { target: { value: 'Nuevo' } })
    fireEvent.keyDown(input, { key: 'Enter' })
    expect(onSave).toHaveBeenCalledWith('Nuevo')
    expect(onSave).toHaveBeenCalledTimes(1)
  })

  it('cancels on Escape without saving and restores the value', () => {
    const onSave = vi.fn()
    render(<InlineEdit label="Título" value="Viejo" onSave={onSave} />)
    openEditor('Título', 'Viejo')
    const input = screen.getByRole('textbox', { name: 'Título' })
    fireEvent.change(input, { target: { value: 'Cambio descartado' } })
    fireEvent.keyDown(input, { key: 'Escape' })
    expect(onSave).not.toHaveBeenCalled()
    expect(screen.queryByRole('textbox', { name: 'Título' })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Editar Título: Viejo' })).toBeInTheDocument()
  })

  it('saves on blur, and does not save twice when Enter is followed by blur', () => {
    const onSave = vi.fn()
    render(<InlineEdit label="Título" value="Viejo" onSave={onSave} />)
    openEditor('Título', 'Viejo')
    const input = screen.getByRole('textbox', { name: 'Título' })
    fireEvent.change(input, { target: { value: 'Por blur' } })
    fireEvent.blur(input)
    expect(onSave).toHaveBeenCalledWith('Por blur')

    const second = render(<InlineEdit label="Estado" value="A" onSave={onSave} />)
    fireEvent.click(second.getByRole('button', { name: 'Editar Estado: A' }))
    const field = second.getByRole('textbox', { name: 'Estado' })
    fireEvent.change(field, { target: { value: 'B' } })
    fireEvent.keyDown(field, { key: 'Enter' })
    fireEvent.blur(field)
    expect(onSave).toHaveBeenCalledTimes(2)
  })

  it('does not save when the value is unchanged', () => {
    const onSave = vi.fn()
    render(<InlineEdit label="Título" value="Igual" onSave={onSave} />)
    openEditor('Título', 'Igual')
    fireEvent.keyDown(screen.getByRole('textbox', { name: 'Título' }), { key: 'Enter' })
    expect(onSave).not.toHaveBeenCalled()
  })

  it('shows a pending state while saving and the Spanish error when saving fails', async () => {
    let reject: (reason: Error) => void = () => {}
    const onSave = vi.fn(() => new Promise<void>((_resolve, rej) => { reject = rej }))
    render(<InlineEdit label="Título" value="Viejo" onSave={onSave} />)
    openEditor('Título', 'Viejo')
    const input = screen.getByRole('textbox', { name: 'Título' })
    fireEvent.change(input, { target: { value: 'Nuevo' } })
    fireEvent.keyDown(input, { key: 'Enter' })

    expect(await screen.findByRole('status')).toHaveTextContent('Guardando')
    expect(screen.getByRole('button', { name: /Editar Título/ })).toBeDisabled()

    reject(new Error('network'))
    expect(await screen.findByRole('alert')).toHaveTextContent(INLINE_EDIT_SAVE_ERROR)
    await waitFor(() => expect(screen.queryByRole('status')).not.toBeInTheDocument())
    expect(screen.getByRole('button', { name: 'Editar Título: Viejo' })).toBeInTheDocument()
  })

  it('renders hostile text values as text', () => {
    const { container } = render(<InlineEdit label="Título" value="<script>alert(1)</script>" onSave={() => {}} />)
    expect(container.querySelector('script')).toBeNull()
    expect(screen.getByText('<script>alert(1)</script>')).toBeInTheDocument()
  })

  it('edits a date with the native date input', () => {
    const onSave = vi.fn()
    const { container } = render(<InlineEdit label="Vence" type="date" value="2026-12-01" onSave={onSave} />)
    fireEvent.click(screen.getByRole('button', { name: 'Editar Vence: 2026-12-01' }))
    const input = container.querySelector('input[type="date"]') as HTMLInputElement
    fireEvent.change(input, { target: { value: '2026-12-15' } })
    fireEvent.keyDown(input, { key: 'Enter' })
    expect(onSave).toHaveBeenCalledWith('2026-12-15')
  })

  it('shows the placeholder for an empty value and saves null when cleared', () => {
    const onSave = vi.fn()
    render(<InlineEdit label="Responsable" type="select" value={null} options={[{ value: 'u1', label: 'Ana' }]} onSave={onSave} />)
    fireEvent.click(screen.getByRole('button', { name: 'Editar Responsable: Sin valor' }))
    const input = screen.getByRole('combobox', { name: 'Responsable' })
    fireEvent.keyDown(input, { key: 'Escape' })
    expect(onSave).not.toHaveBeenCalled()
  })
})
