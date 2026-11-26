import { zodResolver } from '@hookform/resolvers/zod'
import { Alert, Button, Input } from 'antd'
import { useState } from 'react'
import { Controller, useForm, useWatch } from 'react-hook-form'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { z } from 'zod'

import { useAuthStore } from '@/adapters/in/state/auth.store'
import { fieldErrors } from '@/core/shared/app-error'
import { FieldRow } from '@/shared/ui/FieldRow'
import { AuthLayout } from './AuthLayout'
import styles from './AuthPage.module.css'
import { PasswordInput } from './PasswordInput'
import { PasswordStrength } from './PasswordStrength'
import { authPathWithRedirect, safeRedirect } from './safe-redirect'

const schema = z.object({
  name: z.string().min(2, 'El nombre debe tener al menos 2 caracteres'),
  email: z.string().email('Introduce un correo electrónico válido'),
  password: z.string().min(8, 'La contraseña debe tener al menos 8 caracteres'),
})

type FormValues = z.infer<typeof schema>

const SERVER_FIELDS = ['name', 'email', 'password'] as const
const GENERIC_ERROR = 'No se pudo crear la cuenta. Inténtalo de nuevo en unos minutos.'

export function RegisterPage() {
  const register = useAuthStore((s) => s.register)
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const redirect = searchParams.get('redirect')
  const [formError, setFormError] = useState<string | null>(null)

  const {
    control,
    handleSubmit,
    setError,
    formState: { errors, isSubmitting },
  } = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: { name: '', email: '', password: '' },
  })
  const password = useWatch({ control, name: 'password', defaultValue: '' })

  const onSubmit = handleSubmit(async (values) => {
    setFormError(null)
    try {
      await register(values)
      navigate(safeRedirect(redirect))
    } catch (error) {
      const fields = fieldErrors(error)
      const invalid = SERVER_FIELDS.filter((name) => fields[name])
      if (invalid.length > 0) {
        invalid.forEach((name) => setError(name, { type: 'server', message: fields[name] }))
        return
      }
      setFormError(error instanceof Error && error.message ? error.message : GENERIC_ERROR)
    }
  })

  return (
    <AuthLayout
      title="Crear cuenta"
      subtitle="Regístrate para empezar a trabajar en CoreLog."
      footer={
        <>
          ¿Ya tienes cuenta? <Link to={authPathWithRedirect('/login', redirect)}>Iniciar sesión</Link>
        </>
      }
    >
      <form className={styles.form} onSubmit={onSubmit} noValidate aria-busy={isSubmitting}>
        {formError && <Alert type="error" showIcon className={styles.alert} message={formError} />}
        <fieldset className={styles.fieldset} disabled={isSubmitting}>
          <Controller
            name="name"
            control={control}
            render={({ field }) => (
              <FieldRow label="Nombre" error={errors.name?.message}>
                {(aria) => <Input {...field} {...aria} aria-required size="large" autoComplete="name" />}
              </FieldRow>
            )}
          />
          <Controller
            name="email"
            control={control}
            render={({ field }) => (
              <FieldRow label="Correo electrónico" error={errors.email?.message}>
                {(aria) => (
                  <Input
                    {...field}
                    {...aria} aria-required
                    size="large"
                    type="email"
                    autoComplete="email"
                    inputMode="email"
                    autoCapitalize="none"
                    spellCheck={false}
                  />
                )}
              </FieldRow>
            )}
          />
          <Controller
            name="password"
            control={control}
            render={({ field }) => (
              <FieldRow
                label="Contraseña"
                error={errors.password?.message}
                helper={errors.password ? undefined : 'Mínimo 8 caracteres.'}
              >
                {(aria) => (
                  <>
                    <PasswordInput {...field} {...aria} aria-required autoComplete="new-password" />
                    <PasswordStrength password={password} />
                  </>
                )}
              </FieldRow>
            )}
          />
          <p className={styles.terms}>
            Al crear tu cuenta aceptas las condiciones de uso y la política de privacidad de CoreLog.
          </p>
          <Button type="primary" htmlType="submit" size="large" loading={isSubmitting} className={styles.submit}>
            Crear cuenta
          </Button>
        </fieldset>
      </form>
    </AuthLayout>
  )
}
