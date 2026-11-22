import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { IssueTypeIcon } from './IssueTypeIcon'

describe('IssueTypeIcon', () => {
  it('exposes the type name to assistive technology', () => {
    render(<IssueTypeIcon type="bug" />)
    expect(screen.getByTitle('Error')).toBeInTheDocument()
    expect(screen.getByText('Error')).toBeInTheDocument()
  })

  it('shows the visible label when requested', () => {
    render(<IssueTypeIcon type="epic" showLabel />)
    expect(screen.getByText('Épica')).toBeInTheDocument()
  })
})
