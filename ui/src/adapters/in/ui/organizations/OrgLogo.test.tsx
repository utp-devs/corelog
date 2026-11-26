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

  it('makes no request and shows initials when the organization has no logo', () => {
    const get = vi.spyOn(apiClient, 'get')
    render(<OrgLogo orgId="o1" name="Acme Corp" hasLogo={false} />)
    expect(screen.getByTestId('org-logo-fallback')).toHaveTextContent('AC')
    expect(get).not.toHaveBeenCalled()
  })

  it('falls back to initials when the logo request fails', async () => {
    vi.spyOn(apiClient, 'get').mockRejectedValue(new Error('500'))
    render(<OrgLogo orgId="o1" name="Acme Corp" hasLogo />)
    await waitFor(() => expect(apiClient.get).toHaveBeenCalledWith('/organizations/o1/logo', { responseType: 'blob' }))
    expect(screen.getByTestId('org-logo-fallback')).toHaveTextContent('AC')
    expect(screen.queryByTestId('org-logo')).not.toBeInTheDocument()
  })

  it('renders the fetched logo image when the organization has one', async () => {
    vi.spyOn(apiClient, 'get').mockResolvedValue({ data: new Blob(['png'], { type: 'image/png' }) })
    render(<OrgLogo orgId="o1" name="Acme" hasLogo />)
    expect(await screen.findByTestId('org-logo')).toHaveAttribute('src', 'blob:org-logo')
    expect(screen.queryByTestId('org-logo-fallback')).not.toBeInTheDocument()
  })

  it('fetches again when the revision changes after a logo change', async () => {
    const get = vi.spyOn(apiClient, 'get').mockResolvedValue({ data: new Blob(['png'], { type: 'image/png' }) })
    const { rerender } = render(<OrgLogo orgId="o1" name="Acme" hasLogo revision={0} />)
    await screen.findByTestId('org-logo')
    rerender(<OrgLogo orgId="o1" name="Acme" hasLogo revision={1} />)
    await waitFor(() => expect(get).toHaveBeenCalledTimes(2))
  })

  it('shows initials again once the logo is removed (hasLogo false, no request)', async () => {
    const get = vi.spyOn(apiClient, 'get').mockResolvedValue({ data: new Blob(['png'], { type: 'image/png' }) })
    const { rerender } = render(<OrgLogo orgId="o1" name="Acme" hasLogo />)
    await screen.findByTestId('org-logo')
    get.mockClear()
    rerender(<OrgLogo orgId="o1" name="Acme" hasLogo={false} revision={1} />)
    expect(screen.getByTestId('org-logo-fallback')).toBeInTheDocument()
    expect(get).not.toHaveBeenCalled()
  })
})
