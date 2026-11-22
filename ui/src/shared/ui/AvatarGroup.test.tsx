import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { AvatarGroup } from './AvatarGroup'
import { initialsOf } from './initials'

describe('initialsOf', () => {
  it('uses the first letter of the first two words, in uppercase', () => {
    expect(initialsOf('ana torres ruiz')).toBe('AT')
    expect(initialsOf('  pablo  ')).toBe('P')
    expect(initialsOf('')).toBe('')
  })
})

describe('AvatarGroup', () => {
  const users = [
    { id: '1', name: 'Ana Torres' },
    { id: '2', name: 'Luis Paredes' },
    { id: '3', name: 'Karina Navarro' },
  ]

  it('renders the initials of every user when all fit', () => {
    render(<AvatarGroup users={users} max={4} />)
    expect(screen.getByText('AT')).toBeInTheDocument()
    expect(screen.getByText('LP')).toBeInTheDocument()
    expect(screen.getByText('KN')).toBeInTheDocument()
  })

  it('summarizes the users beyond max as a counter', () => {
    render(<AvatarGroup users={users} max={1} />)
    expect(screen.getByText('AT')).toBeInTheDocument()
    expect(screen.getByText('+2')).toBeInTheDocument()
    expect(screen.queryByText('LP')).not.toBeInTheDocument()
  })
})
