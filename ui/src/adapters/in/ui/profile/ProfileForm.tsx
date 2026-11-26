import { zodResolver } from '@hookform/resolvers/zod'
import { Button, Input, Select } from 'antd'
import { useUnsavedChangesGuard } from '@/shared/hooks/useUnsavedChangesGuard'
import { Controller, useForm } from 'react-hook-form'
import { z } from 'zod'

import type { User } from '@/core/auth/domain/user'
import type { ProfileUpdate } from '@/core/people/application/ports/people-repository.port'
import { profileFieldErrors } from '@/core/people/domain/profile-errors'
import { FieldRow } from '@/shared/ui/FieldRow'
import styles from './ProfileForm.module.css'

const schema = z.object({
  name: z.string().trim().min(2, 'El nombre debe tener al menos 2 caracteres').max(120, 'Máximo 120 caracteres'),
  displayName: z.string().max(120, 'Máximo 120 caracteres'),
  jobTitle: z.string().max(120, 'Máximo 120 caracteres'),
  department: z.string().max(120, 'Máximo 120 caracteres'),
  timezone: z.string().min(1, 'Elige una zona horaria'),
})

export type ProfileFormValues = z.infer<typeof schema>

interface ProfileFormProps {
  user: User
  timezones: string[]
  isSubmitting: boolean
  /** Rejects on failure; field errors are shown on the form, other errors are the caller's to report. */
  onSave: (payload: ProfileUpdate) => Promise<void>
}

function valuesOf(user: User): ProfileFormValues {
  return {
    name: user.name,
    displayName: user.displayName,
    jobTitle: user.jobTitle,
    department: user.department,
    timezone: user.timezone || 'UTC',
  }
}

export function ProfileForm({ user, timezones, isSubmitting, onSave }: ProfileFormProps) {
  const {
    control,
    handleSubmit,
    setError,
    formState: { errors, isDirty },
  } = useForm<ProfileFormValues>({
    resolver: zodResolver(schema),
    values: valuesOf(user),
  })
  const unsavedGuard = useUnsavedChangesGuard(isDirty)

  const submit = handleSubmit(async (values) => {
    try {
      await onSave({ ...values, name: values.name.trim() })
    } catch (error) {
      // Errors that do not belong to a field are reported by the caller.
      for (const [field, message] of Object.entries(profileFieldErrors(error))) {
        setError(field as keyof ProfileFormValues, { type: 'server', message })
      }
    }
  })

  const fieldMessage = (name: keyof ProfileFormValues) => errors[name]?.message

  return (
    <>
    {unsavedGuard}
    <form className={styles.form} onSubmit={submit} noValidate aria-label="Datos personales">
      <FieldRow label="Nombre" required error={fieldMessage('name')}>
        {(props) => (
          <Controller
            name="name"
            control={control}
            render={({ field }) => <Input {...field} {...props} maxLength={120} />}
          />
        )}
      </FieldRow>
      <FieldRow label="Nombre para mostrar" helper="Así aparecerás en listas y menciones. Si lo dejas vacío se usa tu nombre." error={fieldMessage('displayName')}>
        {(props) => (
          <Controller
            name="displayName"
            control={control}
            render={({ field }) => <Input {...field} {...props} maxLength={120} />}
          />
        )}
      </FieldRow>
      <FieldRow label="Cargo" error={fieldMessage('jobTitle')}>
        {(props) => (
          <Controller
            name="jobTitle"
            control={control}
            render={({ field }) => <Input {...field} {...props} maxLength={120} />}
          />
        )}
      </FieldRow>
      <FieldRow label="Departamento" error={fieldMessage('department')}>
        {(props) => (
          <Controller
            name="department"
            control={control}
            render={({ field }) => <Input {...field} {...props} maxLength={120} />}
          />
        )}
      </FieldRow>
      <FieldRow label="Zona horaria" required error={fieldMessage('timezone')}>
        {(props) => (
          <Controller
            name="timezone"
            control={control}
            render={({ field }) => (
              <Select
                id={props.id}
                aria-invalid={props['aria-invalid']}
                aria-describedby={props['aria-describedby']}
                value={field.value}
                onChange={field.onChange}
                onBlur={field.onBlur}
                showSearch
                options={timezones.map((zone) => ({ value: zone, label: zone }))}
              />
            )}
          />
        )}
      </FieldRow>
      <FieldRow label="Correo electrónico" helper="Tu correo de acceso no se puede cambiar desde aquí.">
        {(props) => <Input {...props} value={user.email} disabled readOnly />}
      </FieldRow>
      <div className={styles.actions}>
        <Button type="primary" htmlType="submit" loading={isSubmitting} disabled={!isDirty}>
          Guardar cambios
        </Button>
      </div>
    </form>
    </>
  )
}
