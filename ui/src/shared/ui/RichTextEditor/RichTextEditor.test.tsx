import { useState } from 'react'
import { act, createEvent, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { RichTextEditor, type RichTextEditorProps } from './RichTextEditor'
import { draftStorageKey } from './draft'

const UID = '3f2a9c1e-7b4d-4e8a-9c2f-1a2b3c4d5e6f'

function Harness(props: Partial<RichTextEditorProps> & { initial?: string; onValue?: (v: string) => void }) {
  const [value, setValue] = useState(props.initial ?? '')
  return (
    <RichTextEditor
      ariaLabel="Descripción"
      {...props}
      value={value}
      onChange={(next) => {
        setValue(next)
        props.onValue?.(next)
      }}
    />
  )
}

function getTextarea() {
  return screen.getByRole('textbox', { name: 'Descripción' }) as HTMLTextAreaElement
}

describe('RichTextEditor toolbar', () => {
  it('wraps the selection with bold markers', () => {
    render(<Harness initial="hola mundo" />)
    const textarea = getTextarea()
    textarea.setSelectionRange(5, 10)
    fireEvent.click(screen.getByRole('button', { name: 'Negrita' }))
    expect(textarea.value).toBe('hola **mundo**')
  })

  it('inserts a bullet list prefix on the current line', () => {
    render(<Harness initial="uno" />)
    const textarea = getTextarea()
    textarea.setSelectionRange(0, 0)
    fireEvent.click(screen.getByRole('button', { name: 'Lista con viñetas' }))
    expect(textarea.value).toBe('- uno')
  })

  it('inserts a mention trigger and opens the user picker', async () => {
    const searchUsers = vi.fn().mockResolvedValue([{ id: UID, name: 'Ana Pérez' }])
    render(<Harness searchUsers={searchUsers} />)
    fireEvent.click(screen.getByRole('button', { name: 'Mencionar persona' }))
    expect(getTextarea().value).toBe('@')
    expect(await screen.findByRole('option', { name: /Ana Pérez/ })).toBeInTheDocument()
    expect(searchUsers).toHaveBeenCalledWith('')
  })

  it('switches to preview and renders the Markdown', () => {
    render(<Harness initial="**fuerte**" />)
    fireEvent.click(screen.getByRole('button', { name: /Vista previa/ }))
    expect(screen.getByRole('region', { name: 'Vista previa' }).querySelector('strong')).toHaveTextContent('fuerte')
    expect(screen.getByRole('button', { name: /Vista previa/ })).toHaveAttribute('aria-pressed', 'true')
  })
})

describe('RichTextEditor submit', () => {
  it('calls onSubmit on Ctrl+Enter', () => {
    const onSubmit = vi.fn()
    render(<Harness initial="texto" onSubmit={onSubmit} />)
    fireEvent.keyDown(getTextarea(), { key: 'Enter', ctrlKey: true })
    expect(onSubmit).toHaveBeenCalledTimes(1)
  })

  it('does not submit on plain Enter', () => {
    const onSubmit = vi.fn()
    render(<Harness initial="texto" onSubmit={onSubmit} />)
    fireEvent.keyDown(getTextarea(), { key: 'Enter' })
    expect(onSubmit).not.toHaveBeenCalled()
  })
})

describe('RichTextEditor mention picker', () => {
  it('navigates with arrows, selects with Enter and inserts the token', async () => {
    const searchUsers = vi.fn().mockResolvedValue([
      { id: UID, name: 'Ana Pérez' },
      { id: '00000000-0000-4000-8000-000000000002', name: 'Luis Gómez' },
    ])
    render(<Harness searchUsers={searchUsers} />)
    const textarea = getTextarea()
    fireEvent.change(textarea, { target: { value: 'Hola @an' } })

    const options = await screen.findAllByRole('option')
    expect(options).toHaveLength(2)
    expect(options[0]).toHaveAttribute('aria-selected', 'true')

    fireEvent.keyDown(textarea, { key: 'ArrowDown' })
    expect(screen.getAllByRole('option')[1]).toHaveAttribute('aria-selected', 'true')
    fireEvent.keyDown(textarea, { key: 'ArrowDown' })
    expect(screen.getAllByRole('option')[0]).toHaveAttribute('aria-selected', 'true')
    fireEvent.keyDown(textarea, { key: 'ArrowDown' })

    fireEvent.keyDown(textarea, { key: 'Enter' })
    expect(textarea.value).toBe(`Hola @[Luis Gómez](user:00000000-0000-4000-8000-000000000002) `)
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument()
  })

  it('closes with Escape without changing the text', async () => {
    const searchUsers = vi.fn().mockResolvedValue([{ id: UID, name: 'Ana' }])
    render(<Harness searchUsers={searchUsers} />)
    const textarea = getTextarea()
    fireEvent.change(textarea, { target: { value: '@a' } })
    await screen.findByRole('listbox')
    fireEvent.keyDown(textarea, { key: 'Escape' })
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument()
    expect(textarea.value).toBe('@a')
  })

  it('opens the issue picker on a key prefix and inserts the key', async () => {
    const searchIssues = vi.fn().mockResolvedValue([{ key: 'CL-12', summary: 'Error al guardar' }])
    render(<Harness searchIssues={searchIssues} />)
    const textarea = getTextarea()
    fireEvent.change(textarea, { target: { value: 'ver CL-1' } })
    const option = await screen.findByRole('option', { name: /CL-12/ })
    expect(searchIssues).toHaveBeenCalledWith('CL-1')
    fireEvent.mouseDown(option)
    expect(textarea.value).toBe('ver CL-12 ')
  })
})

describe('RichTextEditor image upload', () => {
  it('uploads a pasted image and inserts its URL', async () => {
    const uploadImage = vi.fn().mockResolvedValue({ url: 'https://cdn.example.com/a.png' })
    render(<Harness uploadImage={uploadImage} initial="antes " />)
    const textarea = getTextarea()
    textarea.setSelectionRange(6, 6)
    const file = new File(['png'], 'captura.png', { type: 'image/png' })
    const paste = createEvent.paste(textarea, { clipboardData: { files: [file], items: [] } })
    fireEvent(textarea, paste)
    await waitFor(() => expect(textarea.value).toBe('antes ![](https://cdn.example.com/a.png)\n'))
    expect(uploadImage).toHaveBeenCalledWith(file)
  })

  it('rejects files that are not accepted images', () => {
    const uploadImage = vi.fn()
    render(<Harness uploadImage={uploadImage} />)
    const input = screen.getByLabelText('Seleccionar imagen')
    fireEvent.change(input, { target: { files: [new File(['x'], 'a.txt', { type: 'text/plain' })] } })
    expect(screen.getByRole('alert')).toHaveTextContent('Formato no admitido')
    expect(uploadImage).not.toHaveBeenCalled()
  })

  it('shows an error when the upload fails', async () => {
    const uploadImage = vi.fn().mockRejectedValue(new Error('network'))
    render(<Harness uploadImage={uploadImage} />)
    const file = new File(['png'], 'a.png', { type: 'image/png' })
    fireEvent.change(screen.getByLabelText('Seleccionar imagen'), { target: { files: [file] } })
    expect(await screen.findByRole('alert')).toHaveTextContent('No se pudo subir la imagen')
  })
})

describe('RichTextEditor draft', () => {
  beforeEach(() => window.sessionStorage.clear())
  afterEach(() => window.sessionStorage.clear())

  it('saves the draft after edits, restores it on mount and clears it on submit', async () => {
    const key = 'issue-desc-1'
    const onValue = vi.fn()
    const first = render(<Harness draftKey={key} onValue={onValue} onSubmit={() => {}} />)
    fireEvent.change(getTextarea(), { target: { value: 'borrador' } })
    await waitFor(() => expect(window.sessionStorage.getItem(draftStorageKey(key))).toBe('borrador'))
    first.unmount()

    const restored = render(<Harness draftKey={key} onSubmit={() => {}} />)
    expect(await waitFor(() => getTextarea().value)).toBe('borrador')
    fireEvent.keyDown(getTextarea(), { key: 'Enter', ctrlKey: true })
    expect(window.sessionStorage.getItem(draftStorageKey(key))).toBeNull()
    restored.unmount()
  })

  it('removes the draft when the field is emptied', async () => {
    const key = 'issue-desc-2'
    window.sessionStorage.setItem(draftStorageKey(key), 'viejo')
    render(<Harness draftKey={key} />)
    fireEvent.change(getTextarea(), { target: { value: '' } })
    await waitFor(() => expect(window.sessionStorage.getItem(draftStorageKey(key))).toBeNull())
  })
})

describe('RichTextEditor disabled and autosize', () => {
  it('disables the textarea and toolbar', () => {
    render(<Harness disabled />)
    expect(getTextarea()).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Negrita' })).toBeDisabled()
  })

  it('grows with content through the layout effect', () => {
    render(<Harness minRows={2} maxRows={6} initial={'a\nb\nc'} />)
    act(() => {})
    expect(getTextarea().style.height).not.toBe('')
  })
})
