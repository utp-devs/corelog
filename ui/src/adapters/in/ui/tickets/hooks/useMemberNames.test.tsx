import { renderHook, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import type { OrganizationMember } from '@/core/people/domain/member'
import type { MemberSource } from './useMemberNames'
import { useOrgStore } from '@/adapters/in/state/org.store'
import {
  LOADING_MEMBER_NAME,
  UNKNOWN_MEMBER_NAME,
  resetMemberNamesCache,
  useMemberNames,
} from './useMemberNames'

function member(userId: string, name: string): OrganizationMember {
  return {
    userId,
    name,
    displayName: '',
    email: `${userId}@example.test`,
    jobTitle: '',
    department: '',
    hasAvatar: false,
    role: 'member',
    joinedAt: '2026-01-01T00:00:00Z',
  }
}

function fakeSource(listMembers: MemberSource['listMembers']) {
  return { listMembers: vi.fn(listMembers) } satisfies MemberSource & { listMembers: ReturnType<typeof vi.fn> }
}

describe('useMemberNames', () => {
  beforeEach(() => {
    resetMemberNamesCache()
    useOrgStore.setState({ currentOrgId: 'org-1' })
  })

  it('resolves ids to member names and falls back for ids that are not members', async () => {
    const source = fakeSource(async () => [member('user-1', 'Ana García'), member('user-2', 'Luis Pérez')])
    const { result } = renderHook(() => useMemberNames(source))

    await waitFor(() => expect(result.current.nameOf('user-1')).toBe('Ana García'))
    expect(result.current.nameOf('user-2')).toBe('Luis Pérez')
    expect(result.current.nameOf('removed-user')).toBe(UNKNOWN_MEMBER_NAME)
    expect(source.listMembers).toHaveBeenCalledWith('org-1')
  })

  it('shows a loading placeholder until the member list arrives', async () => {
    let resolve: (members: OrganizationMember[]) => void = () => {}
    const source = fakeSource(() => new Promise<OrganizationMember[]>((done) => { resolve = done }))
    const { result } = renderHook(() => useMemberNames(source))

    expect(result.current.nameOf('user-1')).toBe(LOADING_MEMBER_NAME)
    expect(result.current.isLoading).toBe(true)

    resolve([member('user-1', 'Ana García')])
    await waitFor(() => expect(result.current.nameOf('user-1')).toBe('Ana García'))
    expect(result.current.isLoading).toBe(false)
  })

  it('requests the member list once per organization and shares it between components', async () => {
    const source = fakeSource(async () => [member('user-1', 'Ana García')])
    const first = renderHook(() => useMemberNames(source))
    const second = renderHook(() => useMemberNames(source))

    await waitFor(() => expect(first.result.current.nameOf('user-1')).toBe('Ana García'))
    await waitFor(() => expect(second.result.current.nameOf('user-1')).toBe('Ana García'))
    expect(source.listMembers).toHaveBeenCalledTimes(1)
  })

  it('falls back to the unknown label when the member list cannot be loaded and retries on the next mount', async () => {
    const source = fakeSource(async () => {
      throw new Error('network failure')
    })
    const { result, unmount } = renderHook(() => useMemberNames(source))

    await waitFor(() => expect(result.current.isLoading).toBe(false))
    expect(result.current.nameOf('user-1')).toBe(UNKNOWN_MEMBER_NAME)
    unmount()

    const retry = fakeSource(async () => [member('user-1', 'Ana García')])
    const again = renderHook(() => useMemberNames(retry))
    await waitFor(() => expect(again.result.current.nameOf('user-1')).toBe('Ana García'))
    expect(retry.listMembers).toHaveBeenCalledTimes(1)
  })

  it('does not request anything without an active organization', () => {
    useOrgStore.setState({ currentOrgId: null })
    const source = fakeSource(async () => [member('user-1', 'Ana García')])
    const { result } = renderHook(() => useMemberNames(source))

    expect(source.listMembers).not.toHaveBeenCalled()
    expect(result.current.nameOf('user-1')).toBe(UNKNOWN_MEMBER_NAME)
    expect(result.current.isLoading).toBe(false)
  })

  it('loads the members of the new organization when the active one changes', async () => {
    const source = fakeSource(async (orgId: string) => [member('user-1', orgId === 'org-1' ? 'Ana García' : 'Ana G. (otra org)')])
    const { result } = renderHook(() => useMemberNames(source))
    await waitFor(() => expect(result.current.nameOf('user-1')).toBe('Ana García'))

    useOrgStore.setState({ currentOrgId: 'org-2' })

    await waitFor(() => expect(result.current.nameOf('user-1')).toBe('Ana G. (otra org)'))
    expect(source.listMembers).toHaveBeenCalledWith('org-2')
  })
})
