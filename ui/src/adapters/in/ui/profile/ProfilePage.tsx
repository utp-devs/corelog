import { App, Card, Segmented, Typography } from 'antd'
import { useState } from 'react'

import type { ChangePasswordPayload, UpdateProfilePayload } from '@/core/auth/application/ports/auth-repository.port'
import { useAuthStore } from '@/adapters/in/state/auth.store'
import { extractErrorMessage } from '@/adapters/out/http/api-client'
import type { ThemeMode } from '@/adapters/out/storage/theme-storage'
import { useThemeMode } from '@/app/theme-context'
import { PageHeader } from '@/shared/ui/PageHeader'
import { PasswordForm } from './PasswordForm'
import { ProfileForm } from './ProfileForm'
import styles from './ProfilePage.module.css'

const themeOptions: { value: ThemeMode; label: string }[] = [
  { value: 'light', label: 'Claro' },
  { value: 'dark', label: 'Oscuro' },
]

export function ProfilePage() {
  const { message } = App.useApp()
  const user = useAuthStore((s) => s.user)
  const updateProfile = useAuthStore((s) => s.updateProfile)
  const changePassword = useAuthStore((s) => s.changePassword)
  const { themeMode, setThemeMode } = useThemeMode()

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
      message.success('Contraseña cambiada')
    } catch (error) {
      message.error(extractErrorMessage(error, 'No se pudo cambiar la contraseña'))
      throw error
    } finally {
      setIsSavingPassword(false)
    }
  }

  return (
    <div className={styles.wrapper}>
      <PageHeader
        title="Mi perfil"
        description="Administra tus datos personales, tu contraseña y la apariencia de la aplicación."
      />

      <Card title="Datos personales" className={styles.card}>
        <ProfileForm
          initialName={user.name}
          initialEmail={user.email}
          isSubmitting={isSavingProfile}
          onSubmit={handleProfileSubmit}
        />
      </Card>

      <Card title="Contraseña" className={styles.card}>
        <PasswordForm isSubmitting={isSavingPassword} onSubmit={handlePasswordSubmit} />
      </Card>

      <Card title="Apariencia" className={styles.card}>
        <Typography.Paragraph type="secondary" className={styles.hint}>
          El tema se guarda en este navegador y se aplica al abrir la aplicación.
        </Typography.Paragraph>
        <Segmented
          aria-label="Tema de la aplicación"
          options={themeOptions}
          value={themeMode}
          onChange={(value) => setThemeMode(value as ThemeMode)}
        />
      </Card>
    </div>
  )
}
