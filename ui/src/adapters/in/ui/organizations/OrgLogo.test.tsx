import { render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { apiClient } from '@/adapters/out/http/api-client'

import { OrgLogo } from './OrgLogo'

describe('OrgLogo', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    URL.createObjectURL = vi.fn(() => 'blob:org-logo')
    URL.revokeObjectURL = vi.fn()
  })

  it('shows the initials of the organization name when there is no logo', async () => {
    vi.spyOn(apiClient, 'get').mockRejectedValue(new Error('404'))
    render(<OrgLogo orgId="o1" name="Acme Corp" />)
    expect(screen.getByTestId('org-logo-fallback')).toHaveTextContent('AC')
    await waitFor(() => expect(apiClient.get).toHaveBeenCalledWith('/organizations/o1/logo', { responseType: 'blob' }))
    expect(screen.queryByTestId('org-logo')).not.toBeInTheDocument()
  })

  it('renders the fetched logo image when the organization has one', async () => {
    vi.spyOn(apiClient, 'get').mockResolvedValue({ data: new Blob(['png'], { type: 'image/png' }) })
    render(<OrgLogo orgId="o1" name="Acme" />)
    expect(await screen.findByTestId('org-logo')).toHaveAttribute('src', 'blob:org-logo')
    expect(screen.queryByTestId('org-logo-fallback')).not.toBeInTheDocument()
  })
})
