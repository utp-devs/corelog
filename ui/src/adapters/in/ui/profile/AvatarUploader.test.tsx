import { App } from 'antd'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { AppError } from '@/core/shared/app-error'

const mocks = vi.hoisted(() => ({
  uploadMyAvatar: vi.fn(),
  deleteMyAvatar: vi.fn(),
  getAvatar: vi.fn(),
  getMe: vi.fn(),
}))

vi.mock('@/adapters/out/http/avatar-http-repository', () => ({
  AvatarHttpRepository: class {
    uploadMyAvatar = mocks.uploadMyAvatar
    deleteMyAvatar = mocks.deleteMyAvatar
    getAvatar = mocks.getAvatar
  },
}))

vi.mock('@/adapters/out/http/people-http-repository', () => ({
  ProfileHttpRepository: class {
    getMe = mocks.getMe
  },
}))

import { AvatarUploader } from './AvatarUploader'

const user = {
  id: 'user-1',
  name: 'Ana Torres',
  email: 'ana@corelog.test',
  role: 'user' as const,
  teamId: null,
  displayName: '',
  jobTitle: '',
  department: '',
  avatarKey: null as string | null,
  timezone: 'UTC',
  createdAt: '2026-01-10T00:00:00Z',
  lastLoginAt: null,
  emailVerifiedAt: null,
}

function pick(file: File) {
  const input = screen.getByLabelText('Elegir foto de perfil') as HTMLInputElement
  fireEvent.change(input, { target: { files: [file] } })
}

function image(name: string, type: string, size: number) {
  const file = new File(['x'], name, { type })
  Object.defineProperty(file, 'size', { value: size })
  return file
}

describe('AvatarUploader', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    URL.createObjectURL = vi.fn(() => 'blob:preview')
    URL.revokeObjectURL = vi.fn()
    mocks.uploadMyAvatar.mockResolvedValue('/users/user-1/avatar')
    mocks.deleteMyAvatar.mockResolvedValue(undefined)
    mocks.getMe.mockResolvedValue({ ...user, avatarKey: 'k-1' })
    mocks.getAvatar.mockRejectedValue(new Error('404'))
  })

  it('rejects a file that is not PNG, JPEG or WebP without uploading it', async () => {
    const onUserChange = vi.fn()
    render(<App><AvatarUploader user={user} onUserChange={onUserChange} /></App>)

    pick(image('notas.gif', 'image/gif', 1000))

    expect(await screen.findByText(/Formato no válido/)).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Guardar foto' })).not.toBeInTheDocument()
    expect(mocks.uploadMyAvatar).not.toHaveBeenCalled()
  })

  it('rejects a file larger than 2 MB', async () => {
    render(<App><AvatarUploader user={user} onUserChange={vi.fn()} /></App>)

    pick(image('grande.png', 'image/png', 2 * 1024 * 1024 + 1))

    expect(await screen.findByText(/supera los 2 MB/)).toBeInTheDocument()
    expect(mocks.uploadMyAvatar).not.toHaveBeenCalled()
  })

  it('shows a preview, uploads the chosen file and refreshes the user', async () => {
    const onUserChange = vi.fn()
    render(<App><AvatarUploader user={user} onUserChange={onUserChange} /></App>)

    const file = image('foto.png', 'image/png', 1024)
    pick(file)
    expect(await screen.findByText('Vista previa')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Guardar foto' }))

    await waitFor(() => expect(mocks.uploadMyAvatar).toHaveBeenCalledWith(file))
    await waitFor(() => expect(onUserChange).toHaveBeenCalledWith(expect.objectContaining({ avatarKey: 'k-1' })))
  })

  it('maps the server code user.avatar_too_large to a Spanish message', async () => {
    mocks.uploadMyAvatar.mockRejectedValueOnce(new AppError({ status: 413, code: 'user.avatar_too_large', message: 'too big' }))
    render(<App><AvatarUploader user={user} onUserChange={vi.fn()} /></App>)

    pick(image('foto.png', 'image/png', 1024))
    fireEvent.click(await screen.findByRole('button', { name: 'Guardar foto' }))

    expect(await screen.findByText('La imagen supera los 2 MB. Elige una más ligera.')).toBeInTheDocument()
  })

  it('maps the server code user.avatar_invalid to a Spanish message', async () => {
    mocks.uploadMyAvatar.mockRejectedValueOnce(new AppError({ status: 422, code: 'user.avatar_invalid', message: 'bad' }))
    render(<App><AvatarUploader user={user} onUserChange={vi.fn()} /></App>)

    pick(image('foto.webp', 'image/webp', 1024))
    fireEvent.click(await screen.findByRole('button', { name: 'Guardar foto' }))

    expect(await screen.findByText('La imagen no es válida. Usa un PNG, JPEG o WebP.')).toBeInTheDocument()
  })

  it('removes the current photo when the user has one', async () => {
    const onUserChange = vi.fn()
    render(<App><AvatarUploader user={{ ...user, avatarKey: 'k-1' }} onUserChange={onUserChange} /></App>)

    fireEvent.click(screen.getByRole('button', { name: 'Quitar foto' }))

    await waitFor(() => expect(mocks.deleteMyAvatar).toHaveBeenCalled())
    await waitFor(() => expect(onUserChange).toHaveBeenCalled())
  })

  it('does not offer removal when there is no photo', () => {
    render(<App><AvatarUploader user={user} onUserChange={vi.fn()} /></App>)
    expect(screen.queryByRole('button', { name: 'Quitar foto' })).not.toBeInTheDocument()
  })
})
