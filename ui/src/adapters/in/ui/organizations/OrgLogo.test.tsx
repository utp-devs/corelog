import { render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import type { OrganizationLogoPort } from '@/core/organizations/application/ports/organization-logo.port'

import { OrgLogo } from './OrgLogo'

function fakeRepository(fetchLogoBlob: OrganizationLogoPort['fetchLogoBlob']) {
  return { fetchLogoBlob: vi.fn(fetchLogoBlob) }
}

const png = () => Promise.resolve(new Blob(['png'], { type: 'image/png' }))

describe('OrgLogo', () => {
  beforeEach(() => {
    URL.createObjectURL = vi.fn(() => 'blob:org-logo')
    URL.revokeObjectURL = vi.fn()
  })

  it('makes no request and shows initials when the organization has no logo', () => {
    const repository = fakeRepository(png)
    render(<OrgLogo orgId="o1" name="Acme Corp" hasLogo={false} repository={repository} />)
    expect(screen.getByTestId('org-logo-fallback')).toHaveTextContent('AC')
    expect(repository.fetchLogoBlob).not.toHaveBeenCalled()
  })

  it('falls back to initials when the port resolves with no logo', async () => {
    const repository = fakeRepository(() => Promise.resolve(null))
    render(<OrgLogo orgId="o1" name="Acme Corp" hasLogo repository={repository} />)
    await waitFor(() => expect(repository.fetchLogoBlob).toHaveBeenCalledWith('o1'))
    expect(screen.getByTestId('org-logo-fallback')).toHaveTextContent('AC')
    expect(screen.queryByTestId('org-logo')).not.toBeInTheDocument()
  })

  it('renders the fetched logo image when the organization has one', async () => {
    const repository = fakeRepository(png)
    render(<OrgLogo orgId="o1" name="Acme" hasLogo repository={repository} />)
    expect(await screen.findByTestId('org-logo')).toHaveAttribute('src', 'blob:org-logo')
    expect(screen.queryByTestId('org-logo-fallback')).not.toBeInTheDocument()
  })

  it('fetches again when the revision changes after a logo change', async () => {
    const repository = fakeRepository(png)
    const { rerender } = render(<OrgLogo orgId="o1" name="Acme" hasLogo revision={0} repository={repository} />)
    await screen.findByTestId('org-logo')
    rerender(<OrgLogo orgId="o1" name="Acme" hasLogo revision={1} repository={repository} />)
    await waitFor(() => expect(repository.fetchLogoBlob).toHaveBeenCalledTimes(2))
  })

  it('shows initials again once the logo is removed (hasLogo false, no request)', async () => {
    const repository = fakeRepository(png)
    const { rerender } = render(<OrgLogo orgId="o1" name="Acme" hasLogo repository={repository} />)
    await screen.findByTestId('org-logo')
    repository.fetchLogoBlob.mockClear()
    rerender(<OrgLogo orgId="o1" name="Acme" hasLogo={false} revision={1} repository={repository} />)
    expect(screen.getByTestId('org-logo-fallback')).toBeInTheDocument()
    expect(repository.fetchLogoBlob).not.toHaveBeenCalled()
  })

  it('revokes the object URL when the logo unmounts', async () => {
    const repository = fakeRepository(png)
    const { unmount } = render(<OrgLogo orgId="o1" name="Acme" hasLogo repository={repository} />)
    await screen.findByTestId('org-logo')
    unmount()
    expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:org-logo')
  })
})
