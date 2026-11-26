import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { IssueKey } from './IssueKey'

describe('IssueKey', () => {
  afterEach(() => {
    Reflect.deleteProperty(navigator, 'clipboard')
  })

  it('renders the key as a link to the issue', () => {
    render(<IssueKey issueKey="CL-120" href="/incidencias/CL-120" type="bug" />)
    expect(screen.getByRole('link', { name: 'CL-120' })).toHaveAttribute('href', '/incidencias/CL-120')
    expect(screen.getByText('Error')).toBeInTheDocument()
  })

  it('marks resolved issues with a struck-through key', () => {
    render(<IssueKey issueKey="CL-7" href="/x" resolved data-testid="key" />)
    expect(screen.getByTestId('key')).toHaveAttribute('data-resolved', 'true')
    expect(screen.getByRole('link', { name: 'CL-7' }).className).toMatch(/resolved/)
  })

  it('copies the key with the copy button', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true })
    render(<IssueKey issueKey="CL-9" href="/x" />)
    fireEvent.click(screen.getByRole('button', { name: 'Copiar clave' }))
    await waitFor(() => expect(writeText).toHaveBeenCalledWith('CL-9'))
  })

  it('shows the summary in a tooltip on hover', async () => {
    render(<IssueKey issueKey="CL-3" href="/x" summary="Error al guardar el perfil" />)
    fireEvent.mouseEnter(screen.getByRole('link', { name: 'CL-3' }))
    expect(await screen.findByRole('tooltip')).toHaveTextContent('Error al guardar el perfil')
  })
})
