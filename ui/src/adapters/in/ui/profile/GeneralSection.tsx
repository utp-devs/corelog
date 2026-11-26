import { Card, Typography } from 'antd'
import { useCallback, useMemo, useState } from 'react'

import { ProfileHttpRepository } from '@/adapters/out/http/people-http-repository'
import { extractErrorMessage } from '@/adapters/out/http/api-client'
import type { User } from '@/core/auth/domain/user'
import type { ProfileUpdate } from '@/core/people/application/ports/people-repository.port'
import { profileFieldErrors } from '@/core/people/domain/profile-errors'
import { timezoneOptions } from '@/core/people/domain/timezones'
import { toast } from '@/shared/ui/Toaster'
import { AvatarUploader } from './AvatarUploader'
import { ProfileForm } from './ProfileForm'
import styles from './ProfilePage.module.css'

const profiles = new ProfileHttpRepository()

interface GeneralSectionProps {
  user: User
  onUserChange: (user: User) => void
}

/** Profile general: photo, personal data and timezone. */
export function GeneralSection({ user, onUserChange }: GeneralSectionProps) {
  const [isSaving, setIsSaving] = useState(false)
  const timezones = useMemo(() => timezoneOptions(user.timezone), [user.timezone])

  const save = useCallback(
    async (payload: ProfileUpdate) => {
      setIsSaving(true)
      try {
        onUserChange(await profiles.updateMe(payload))
        toast.success('Perfil actualizado')
      } catch (error) {
        // Field errors are shown by the form; anything else is reported here.
        if (Object.keys(profileFieldErrors(error)).length === 0) {
          toast.error(extractErrorMessage(error, 'No se pudo actualizar el perfil'))
        }
        throw error
      } finally {
        setIsSaving(false)
      }
    },
    [onUserChange],
  )

  return (
    <section id="perfil-general" aria-labelledby="perfil-general-title" className={styles.section}>
      <Typography.Title level={4} id="perfil-general-title" className={styles.sectionTitle}>General</Typography.Title>
      <Card title="Foto de perfil" className={styles.card}>
        <AvatarUploader user={user} onUserChange={onUserChange} />
      </Card>
      <Card title="Datos personales" className={styles.card}>
        <Typography.Paragraph type="secondary" className={styles.hint}>
          Tu nombre y tu correo son los que ven tus compañeros en las incidencias.
        </Typography.Paragraph>
        <ProfileForm
          user={user}
          timezones={timezones}
          isSubmitting={isSaving}
          onSave={save}
        />
      </Card>
    </section>
  )
}
