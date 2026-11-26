import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { Avatar } from './Avatar'
import { avatarColorIndex, hashString } from './avatarColor'

describe('Avatar', () => {
  it('shows the initials and uses the full name as accessible name', () => {
    render(<Avatar id="u1" name="Ana Pérez" />)
    expect(screen.getByRole('img', { name: 'Ana Pérez' })).toHaveTextContent('AP')
  })

  it('renders the photo with the name as alt text when src is given', () => {
    render(<Avatar id="u1" name="Ana Pérez" src="/media/ana.png" />)
    const img = screen.getByRole('img', { name: 'Ana Pérez' })
    expect(img.tagName).toBe('IMG')
    expect(img).toHaveAttribute('src', '/media/ana.png')
  })

  it('gives the same color class for the same id', () => {
    const first = render(<Avatar id="user-42" name="Ana" />)
    const second = render(<Avatar id="user-42" name="Otra Persona" />)
    const firstClass = first.container.firstElementChild?.className
    const secondClass = second.container.firstElementChild?.className
    expect(firstClass).toBeTruthy()
    expect(firstClass).toBe(secondClass)
  })

  it('maps ids to one of the ten avatar colors and spreads them over several', () => {
    const indexes = new Set<number>()
    for (let i = 0; i < 200; i += 1) {
      const index = avatarColorIndex(`user-${i}`)
      expect(index).toBeGreaterThanOrEqual(1)
      expect(index).toBeLessThanOrEqual(10)
      indexes.add(index)
    }
    expect(indexes.size).toBeGreaterThanOrEqual(8)
  })

  it('hashes deterministically', () => {
    expect(hashString('abc')).toBe(hashString('abc'))
    expect(hashString('abc')).not.toBe(hashString('abd'))
  })

  it('renders hostile names as text, never as markup', () => {
    const name = '<script>alert(1)</script>'
    const { container } = render(<Avatar id="u1" name={name} />)
    expect(container.querySelector('script')).toBeNull()
    expect(screen.getByRole('img', { name })).toBeInTheDocument()
  })
})
