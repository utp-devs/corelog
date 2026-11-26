import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { AppError } from '@/core/shared/app-error'
import {
  createFakeAdminRepository,
  createdOf,
  invitationOf,
  memberOf,
  ORG,
} from '@/adapters/in/ui/organizations/members/test-fakes'
import { createOrgAdminStore } from './org-admin.store'

const conflict = new AppError({ status: 409, code: 'org.last_owner', message: 'Es el último propietario.' })

describe('org admin store', () => {
  beforeEach(() => {
    // jsdom has no object URL support.
    URL.createObjectURL = vi.fn(() => 'blob:logo-1')
    URL.revokeObjectURL = vi.fn()
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  it('loads members and reports the loading status', async () => {
    const repo = createFakeAdminRepository({
      listMembers: vi.fn(async () => [memberOf('u-1', 'Ana', 'owner')]),
    })
    const store = createOrgAdminStore(repo)
    const pending = store.getState().fetchMembers(ORG.id)
    expect(store.getState().membersStatus).toBe('loading')
    await pending
    expect(store.getState().membersStatus).toBe('ready')
    expect(store.getState().members).toHaveLength(1)
  })

  it('stores a load failure as an AppError without throwing', async () => {
    const repo = createFakeAdminRepository({
      listMembers: vi.fn(async () => {
        throw new AppError({ status: 403, code: 'org.forbidden', message: 'No' })
      }),
    })
    const store = createOrgAdminStore(repo)
    await store.getState().fetchMembers(ORG.id)
    expect(store.getState().membersStatus).toBe('error')
    expect(store.getState().membersError?.code).toBe('org.forbidden')
  })

  it('updates the role locally after a successful change', async () => {
    const repo = createFakeAdminRepository()
    const store = createOrgAdminStore(repo)
    store.setState({ members: [memberOf('u-2', 'Luis', 'member')] })
    await store.getState().changeMemberRole(ORG.id, 'u-2', 'admin')
    expect(repo.changeMemberRole).toHaveBeenCalledWith(ORG.id, 'u-2', 'admin')
    expect(store.getState().members[0].role).toBe('admin')
  })

  it('rejects with the API error and keeps the list when a role change fails (last owner)', async () => {
    const repo = createFakeAdminRepository({ changeMemberRole: vi.fn(async () => Promise.reject(conflict)) })
    const store = createOrgAdminStore(repo)
    store.setState({ members: [memberOf('u-1', 'Ana', 'owner')] })
    await expect(store.getState().changeMemberRole(ORG.id, 'u-1', 'member')).rejects.toBe(conflict)
    expect(store.getState().members[0].role).toBe('owner')
    expect(store.getState().error?.code).toBe('org.last_owner')
  })

  it('removes a member from the list after success', async () => {
    const store = createOrgAdminStore(createFakeAdminRepository())
    store.setState({ members: [memberOf('u-1', 'Ana', 'owner'), memberOf('u-2', 'Luis', 'guest')] })
    await store.getState().removeMember(ORG.id, 'u-2')
    expect(store.getState().members.map((m) => m.userId)).toEqual(['u-1'])
  })

  it('keeps invitations in the current scope and refreshes after create', async () => {
    const listInvitations = vi.fn(async () => [invitationOf('i-1', 'ana@acme.test')])
    const repo = createFakeAdminRepository({
      listInvitations,
      createInvitations: vi.fn(async () => [createdOf('i-1', 'ana@acme.test')]),
    })
    const store = createOrgAdminStore(repo)
    await store.getState().fetchInvitations(ORG.id, 'all')
    expect(listInvitations).toHaveBeenLastCalledWith(ORG.id, 'all')

    const created = await store.getState().createInvitations(ORG.id, [{ email: 'ana@acme.test', role: 'member' }])
    expect(created[0].acceptUrl).toMatch(/\/invite\//)
    expect(store.getState().createdInvitations).toHaveLength(1)
    expect(listInvitations).toHaveBeenLastCalledWith(ORG.id, 'all')
    expect(store.getState().invitations).toHaveLength(1)
  })

  it('replaces the one-time link on resend and clears it on request', async () => {
    const repo = createFakeAdminRepository({
      resendInvitation: vi.fn(async () => createdOf('i-2', 'luis@acme.test')),
    })
    const store = createOrgAdminStore(repo)
    await store.getState().resendInvitation(ORG.id, 'i-2')
    expect(store.getState().createdInvitations.map((i) => i.id)).toEqual(['i-2'])
    store.getState().clearCreatedInvitations()
    expect(store.getState().createdInvitations).toEqual([])
  })

  it('loads the logo as an object URL and revokes the previous one', async () => {
    const blob = new Blob(['png'], { type: 'image/png' })
    const getLogo = vi.fn(async () => blob)
    const store = createOrgAdminStore(createFakeAdminRepository({ getLogo }))
    await store.getState().loadLogo(ORG.id)
    expect(store.getState().logoUrl).toBe('blob:logo-1')

    await store.getState().loadLogo(ORG.id)
    expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:logo-1')
  })

  it('treats a missing logo as no logo', async () => {
    const store = createOrgAdminStore(createFakeAdminRepository({ getLogo: vi.fn(async () => null) }))
    await store.getState().loadLogo(ORG.id)
    expect(store.getState().logoUrl).toBeNull()
  })

  it('skips the request and clears the preview when the organization has no logo', async () => {
    const getLogo = vi.fn(async () => new Blob(['x']))
    const store = createOrgAdminStore(createFakeAdminRepository({ getLogo }))
    await store.getState().loadLogo(ORG.id, false)
    expect(getLogo).not.toHaveBeenCalled()
    expect(store.getState().logoUrl).toBeNull()
  })

  it('reports logo uploads and removals to the organization list', async () => {
    const onLogoChanged = vi.fn()
    const store = createOrgAdminStore(
      createFakeAdminRepository({ getLogo: vi.fn(async () => new Blob(['x'])) }),
      { onLogoChanged },
    )
    await store.getState().uploadLogo(ORG.id, new File(['x'], 'logo.png', { type: 'image/png' }))
    expect(onLogoChanged).toHaveBeenLastCalledWith(ORG.id, true)
    await store.getState().removeLogo(ORG.id)
    expect(onLogoChanged).toHaveBeenLastCalledWith(ORG.id, false)
  })

  it('uploads a logo and then reloads it', async () => {
    const uploadLogo = vi.fn(async () => '/organizations/org-1/logo')
    const getLogo = vi.fn(async () => new Blob(['x'], { type: 'image/webp' }))
    const store = createOrgAdminStore(createFakeAdminRepository({ uploadLogo, getLogo }))
    const file = new File(['x'], 'logo.webp', { type: 'image/webp' })
    await store.getState().uploadLogo(ORG.id, file)
    expect(uploadLogo).toHaveBeenCalledWith(ORG.id, file)
    expect(getLogo).toHaveBeenCalledWith(ORG.id)
  })

  it('sends the typed slug when deleting the organization', async () => {
    const deleteOrganization = vi.fn(async () => undefined)
    const store = createOrgAdminStore(createFakeAdminRepository({ deleteOrganization }))
    await store.getState().deleteOrganization(ORG.id, ORG.slug)
    expect(deleteOrganization).toHaveBeenCalledWith(ORG.id, 'acme')
  })

  it('reset clears data and revokes the logo URL', async () => {
    const store = createOrgAdminStore(createFakeAdminRepository({ getLogo: vi.fn(async () => new Blob(['x'])) }))
    await store.getState().loadLogo(ORG.id)
    store.setState({ members: [memberOf('u-1', 'Ana', 'owner')] })
    store.getState().reset()
    expect(store.getState().members).toEqual([])
    expect(store.getState().logoUrl).toBeNull()
    expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:logo-1')
  })
})
