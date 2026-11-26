import { renderHook, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { OrganizationHttpRepository } from '@/adapters/out/http/organization-http-repository'
import { AppError } from '@/core/shared/app-error'

import { resetOrgMembersByIdCache, useOrgMembersById } from './useOrgMembersById'

function memberDto(userId: string, name: string) {
  return {
    userId,
    name,
    displayName: '',
    email: `${userId}@example.com`,
    jobTitle: '',
    department: '',
    hasAvatar: false,
    role: 'member',
    joinedAt: '2026-01-01T00:00:00Z',
  } as unknown as Awaited<ReturnType<OrganizationHttpRepository['listMembers']>>[number]
}

describe('useOrgMembersById', () => {
  beforeEach(() => {
    resetOrgMembersByIdCache()
    vi.restoreAllMocks()
  })

  afterEach(() => {
    resetOrgMembersByIdCache()
  })

  it('maps user ids to names once the organization members load', async () => {
    const listMembers = vi.spyOn(OrganizationHttpRepository.prototype, 'listMembers')
      .mockResolvedValue([memberDto('u1', 'Ana Pérez'), memberDto('u2', 'Luis Gómez')])

    const { result } = renderHook(() => useOrgMembersById('o1'))
    expect(result.current.status).toBe('loading')
    expect(result.current.members.size).toBe(0)

    await waitFor(() => expect(result.current.status).toBe('ready'))
    expect(result.current.members.get('u1')).toEqual({ name: 'Ana Pérez' })
    expect(result.current.members.get('u2')).toEqual({ name: 'Luis Gómez' })
    expect(result.current.members.get('missing')).toBeUndefined()
    expect(listMembers).toHaveBeenCalledWith('o1')
  })

  it('requests each organization once and reuses the cached map on later renders', async () => {
    const listMembers = vi.spyOn(OrganizationHttpRepository.prototype, 'listMembers')
      .mockResolvedValue([memberDto('u1', 'Ana Pérez')])

    const first = renderHook(() => useOrgMembersById('o1'))
    await waitFor(() => expect(first.result.current.status).toBe('ready'))
    first.unmount()

    const second = renderHook(() => useOrgMembersById('o1'))
    await waitFor(() => expect(second.result.current.status).toBe('ready'))
    expect(second.result.current.members.get('u1')).toEqual({ name: 'Ana Pérez' })
    expect(listMembers).toHaveBeenCalledTimes(1)
  })

  it('stays idle and empty while the organization is unknown', () => {
    const listMembers = vi.spyOn(OrganizationHttpRepository.prototype, 'listMembers')

    const { result } = renderHook(() => useOrgMembersById(undefined))
    expect(result.current.status).toBe('idle')
    expect(result.current.members.size).toBe(0)
    expect(listMembers).not.toHaveBeenCalled()
  })

  it('reports an error with an empty map and retries the request on the next mount', async () => {
    const listMembers = vi.spyOn(OrganizationHttpRepository.prototype, 'listMembers')
      .mockRejectedValueOnce(new AppError({ status: 500, message: 'boom' }))
      .mockResolvedValueOnce([memberDto('u1', 'Ana Pérez')])

    const failed = renderHook(() => useOrgMembersById('o1'))
    await waitFor(() => expect(failed.result.current.status).toBe('error'))
    expect(failed.result.current.members.size).toBe(0)
    failed.unmount()

    const retried = renderHook(() => useOrgMembersById('o1'))
    await waitFor(() => expect(retried.result.current.status).toBe('ready'))
    expect(retried.result.current.members.get('u1')).toEqual({ name: 'Ana Pérez' })
    expect(listMembers).toHaveBeenCalledTimes(2)
  })
})
