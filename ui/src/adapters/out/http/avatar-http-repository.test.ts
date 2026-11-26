import { afterEach, describe, expect, it, vi } from 'vitest'

import { apiClient } from './api-client'
import { AvatarHttpRepository } from './avatar-http-repository'

describe('AvatarHttpRepository', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('requests the avatar of a user as a blob', async () => {
    const blob = new Blob(['png'], { type: 'image/png' })
    const get = vi.spyOn(apiClient, 'get').mockResolvedValue({ data: blob })

    const result = await new AvatarHttpRepository().getAvatar('u-1')

    expect(get).toHaveBeenCalledWith('/users/u-1/avatar', { responseType: 'blob' })
    expect(result).toBe(blob)
  })

  it('uploads the file as multipart field "file" and returns the avatar URL', async () => {
    const put = vi.spyOn(apiClient, 'put').mockResolvedValue({ data: { data: { avatar_url: '/users/u-1/avatar' } } })
    const file = new File(['png'], 'foto.png', { type: 'image/png' })

    const url = await new AvatarHttpRepository().uploadMyAvatar(file)

    expect(url).toBe('/users/u-1/avatar')
    const [path, body, config] = put.mock.calls[0]
    expect(path).toBe('/users/me/avatar')
    expect(body).toBeInstanceOf(FormData)
    expect((body as FormData).get('file')).toBeInstanceOf(File)
    expect(config).toEqual({ headers: { 'Content-Type': 'multipart/form-data' } })
  })

  it('deletes the own avatar', async () => {
    const del = vi.spyOn(apiClient, 'delete').mockResolvedValue({ data: {} })

    await new AvatarHttpRepository().deleteMyAvatar()

    expect(del).toHaveBeenCalledWith('/users/me/avatar')
  })
})
