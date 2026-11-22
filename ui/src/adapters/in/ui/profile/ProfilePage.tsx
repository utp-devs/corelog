import { App, Card } from 'antd'
import { useState } from 'react'

import type { ChangePasswordPayload, UpdateProfilePayload } from '@/core/auth/application/ports/auth-repository.port'
import { useAuthStore } from '@/adapters/in/state/auth.store'
import { extractErrorMessage } from '@/adapters/out/http/api-client'
import { PasswordForm } from './PasswordForm'
import { ProfileForm } from './ProfileForm'
import styles from './ProfilePage.module.css'
import { PageHeader } from '@/shared/ui/PageHeader'

export function ProfilePage() {
  const { message } = App.useApp()
  const user = useAuthStore((s) => s.user)
  const updateProfile = useAuthStore((s) => s.updateProfile)
  const changePassword = useAuthStore((s) => s.changePassword)

  const [isSavingProfile, setIsSavingProfile] = useState(false)
  const [isSavingPassword, setIsSavingPassword] = useState(false)

  if (!user) {
    return null
  }

  const handleProfileSubmit = async (payload: UpdateProfilePayload) => {
    setIsSavingProfile(true)
    try {
      await updateProfile(payload)
      message.success('Perfil actualizado')
    } catch (error) {
      message.error(extractErrorMessage(error, 'No se pudo actualizar el perfil'))
    } finally {
      setIsSavingProfile(false)
    }
  }

  const handlePasswordSubmit = async (payload: ChangePasswordPayload) => {
    setIsSavingPassword(true)
    try {
      await changePassword(payload)
      message.success('Contrasena cambiada')
    } catch (error) {
      message.error(extractErrorMessage(error, 'No se pudo cambiar la contrasena'))
      throw error
    } finally {
      setIsSavingPassword(false)
    }
  }

  return (
    <div className={styles.wrapper}>
      <PageHeader
        title="Mi perfil"
        description="Administra tus datos personales y contraseña."
      />

      <Card title="Datos personales">
        <ProfileForm
          initialName={user.name}
          initialEmail={user.email}
          isSubmitting={isSavingProfile}
          onSubmit={handleProfileSubmit}
        />
      </Card>

      <Card title="Contrasena">
        <PasswordForm isSubmitting={isSavingPassword} onSubmit={handlePasswordSubmit} />
      </Card>
    </div>
  )
}
