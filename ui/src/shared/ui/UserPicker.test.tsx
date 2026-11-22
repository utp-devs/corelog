import { render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'

import { UserPicker } from './UserPicker'

const users = [{ id: 'u1', name: 'Ana Torres' }]

describe('UserPicker', () => {
  it('shows the selected user name', () => {
    render(<UserPicker users={users} value="u1" onChange={() => {}} />)
    expect(screen.getByText('Ana Torres')).toBeInTheDocument()
  })

  it('renders the placeholder when nothing is selected', () => {
    render(<UserPicker users={[]} onChange={() => {}} placeholder="Elegir responsable" />)
    expect(screen.getByText('Elegir responsable')).toBeInTheDocument()
  })

  it('accepts a search callback without calling it on render', () => {
    const onSearch = vi.fn()
    render(<UserPicker users={users} onChange={() => {}} onSearch={onSearch} />)
    expect(onSearch).not.toHaveBeenCalled()
  })
})
