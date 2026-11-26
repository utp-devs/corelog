import { Alert, Button, Space, Typography } from 'antd'
import { useEffect, useRef, useState, type ChangeEvent } from 'react'

import { AvatarHttpRepository } from '@/adapters/out/http/avatar-http-repository'
import { extractErrorMessage } from '@/adapters/out/http/api-client'
import { ProfileHttpRepository } from '@/adapters/out/http/people-http-repository'
import type { User } from '@/core/auth/domain/user'
import { AVATAR_MAX_BYTES } from '@/core/people/domain/member'
import { avatarErrorMessage, validateAvatarFile } from '@/core/people/domain/avatar'
import { toast } from '@/shared/ui/Toaster'
import { UserAvatar } from '../people/UserAvatar'
import { Avatar } from '@/shared/ui/Avatar'
import styles from './AvatarUploader.module.css'

const avatars = new AvatarHttpRepository()
const profiles = new ProfileHttpRepository()

interface Preview {
  file: File
  url: string
}

interface AvatarUploaderProps {
  user: User
  onUserChange: (user: User) => void
}

/** Photo of the user with upload (PNG, JPEG, WebP up to 2 MB), preview and removal. */
export function AvatarUploader({ user, onUserChange }: AvatarUploaderProps) {
  const inputRef = useRef<HTMLInputElement>(null)
  const [preview, setPreview] = useState<Preview | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState<'upload' | 'remove' | null>(null)

  // Object URL of the preview is released when replaced or when the component goes away.
  useEffect(() => {
    if (!preview) return
    return () => URL.revokeObjectURL(preview.url)
  }, [preview])

  const onPick = (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0]
    event.target.value = ''
    if (!file) return
    const problem = validateAvatarFile(file)
    if (problem) {
      setError(problem)
      setPreview(null)
      return
    }
    setError(null)
    setPreview({ file, url: URL.createObjectURL(file) })
  }

  const refreshUser = async () => {
    onUserChange(await profiles.getMe())
  }

  const upload = async () => {
    if (!preview) return
    setBusy('upload')
    try {
      await avatars.uploadMyAvatar(preview.file)
      await refreshUser()
      setPreview(null)
      toast.success('Foto de perfil actualizada')
    } catch (failure) {
      setError(avatarErrorMessage(failure) ?? extractErrorMessage(failure, 'No se pudo subir la foto'))
    } finally {
      setBusy(null)
    }
  }

  const remove = async () => {
    setBusy('remove')
    try {
      await avatars.deleteMyAvatar()
      await refreshUser()
      setError(null)
      toast.success('Foto de perfil eliminada')
    } catch (failure) {
      setError(extractErrorMessage(failure, 'No se pudo quitar la foto'))
    } finally {
      setBusy(null)
    }
  }

  return (
    <div className={styles.root}>
      <div className={styles.figure}>
        {preview ? (
          <>
            <Avatar id={user.id} name={user.name} src={preview.url} size={64} data-testid="avatar-preview" />
            <Typography.Text type="secondary" className={styles.caption}>Vista previa</Typography.Text>
          </>
        ) : (
          <UserAvatar id={user.id} name={user.name} avatarKey={user.avatarKey} size={64} data-testid="avatar-current" />
        )}
      </div>
      <div className={styles.controls}>
        <input
          ref={inputRef}
          type="file"
          hidden
          accept="image/png,image/jpeg,image/webp"
          aria-label="Elegir foto de perfil"
          onChange={onPick}
        />
        {preview ? (
          <Space wrap>
            <Button type="primary" onClick={() => void upload()} loading={busy === 'upload'}>Guardar foto</Button>
            <Button onClick={() => setPreview(null)} disabled={busy !== null}>Cancelar</Button>
          </Space>
        ) : (
          <Space wrap>
            <Button onClick={() => inputRef.current?.click()} disabled={busy !== null}>Subir foto</Button>
            {user.avatarKey && (
              <Button danger type="text" onClick={() => void remove()} loading={busy === 'remove'}>Quitar foto</Button>
            )}
          </Space>
        )}
        <Typography.Text type="secondary" className={styles.hint}>
          PNG, JPEG o WebP de hasta {AVATAR_MAX_BYTES / (1024 * 1024)} MB.
        </Typography.Text>
        {error && <Alert type="error" showIcon message={error} className={styles.alert} />}
      </div>
    </div>
  )
}
