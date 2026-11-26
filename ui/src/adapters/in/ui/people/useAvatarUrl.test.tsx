import { renderHook, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({ getAvatar: vi.fn() }))

vi.mock('@/adapters/out/http/avatar-http-repository', () => ({
  AvatarHttpRepository: class {
    getAvatar = mocks.getAvatar
  },
}))

import { useAvatarUrl } from './useAvatarUrl'

let urlCount = 0

describe('useAvatarUrl', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    urlCount = 0
    URL.createObjectURL = vi.fn(() => `blob:avatar-${++urlCount}`)
    URL.revokeObjectURL = vi.fn()
    mocks.getAvatar.mockImplementation(async () => new Blob(['png'], { type: 'image/png' }))
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('does not fetch when the user has no avatar', () => {
    const { result } = renderHook(() => useAvatarUrl('user-none', false))

    expect(result.current).toBeNull()
    expect(mocks.getAvatar).not.toHaveBeenCalled()
  })

  it('fetches the avatar once and returns an object URL', async () => {
    const { result } = renderHook(() => useAvatarUrl('user-a', true, 'v1'))

    await waitFor(() => expect(result.current).toBe('blob:avatar-1'))
    expect(mocks.getAvatar).toHaveBeenCalledWith('user-a')
  })

  it('shares one fetch between components showing the same user and version', async () => {
    const first = renderHook(() => useAvatarUrl('user-b', true, 'v1'))
    const second = renderHook(() => useAvatarUrl('user-b', true, 'v1'))

    await waitFor(() => expect(second.result.current).toBe('blob:avatar-1'))
    expect(first.result.current).toBe('blob:avatar-1')
    expect(mocks.getAvatar).toHaveBeenCalledTimes(1)
  })

  it('revokes the object URL only after the last user unmounts', async () => {
    const first = renderHook(() => useAvatarUrl('user-c', true, 'v1'))
    const second = renderHook(() => useAvatarUrl('user-c', true, 'v1'))
    await waitFor(() => expect(second.result.current).toBe('blob:avatar-1'))

    first.unmount()
    expect(URL.revokeObjectURL).not.toHaveBeenCalled()

    second.unmount()
    expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:avatar-1')
  })

  it('fetches again when the version changes and releases the old URL', async () => {
    const { result, rerender } = renderHook(({ version }) => useAvatarUrl('user-d', true, version), {
      initialProps: { version: 'v1' },
    })
    await waitFor(() => expect(result.current).toBe('blob:avatar-1'))

    rerender({ version: 'v2' })

    await waitFor(() => expect(result.current).toBe('blob:avatar-2'))
    expect(mocks.getAvatar).toHaveBeenCalledTimes(2)
    expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:avatar-1')
  })

  it('returns null when the fetch fails so the caller shows initials', async () => {
    mocks.getAvatar.mockRejectedValueOnce(new Error('404'))
    const { result } = renderHook(() => useAvatarUrl('user-e', true, 'v1'))

    await waitFor(() => expect(mocks.getAvatar).toHaveBeenCalled())
    expect(result.current).toBeNull()
  })

  it('releases the URL when the user goes away before the blob arrives', async () => {
    let resolve: (blob: Blob) => void = () => undefined
    mocks.getAvatar.mockImplementationOnce(() => new Promise<Blob>((done) => { resolve = done }))
    const { unmount } = renderHook(() => useAvatarUrl('user-f', true, 'v1'))

    unmount()
    resolve(new Blob(['png']))
    await waitFor(() => expect(URL.createObjectURL).not.toHaveBeenCalled())
  })
})
