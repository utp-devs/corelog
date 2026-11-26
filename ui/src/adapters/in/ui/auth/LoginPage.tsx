import { zodResolver } from '@hookform/resolvers/zod'
import { Alert, Button, Input } from 'antd'
import { useState } from 'react'
import { Controller, useForm } from 'react-hook-form'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { z } from 'zod'

import { useAuthStore } from '@/adapters/in/state/auth.store'
import { fieldErrors } from '@/core/shared/app-error'
import { FieldRow } from '@/shared/ui/FieldRow'
import { AuthLayout } from './AuthLayout'
import styles from './AuthPage.module.css'
import { PasswordInput } from './PasswordInput'
import { authPathWithRedirect, safeRedirect } from './safe-redirect'

const schema = z.object({
  email: z.string().email('Introduce un correo electrónico válido'),
  password: z.string().min(1, 'La contraseña es obligatoria'),
})

type FormValues = z.infer<typeof schema>

const SERVER_FIELDS = ['email', 'password'] as const
const GENERIC_ERROR = 'No se pudo iniciar sesión. Revisa tus datos e inténtalo de nuevo.'

export function LoginPage() {
  const login = useAuthStore((s) => s.login)
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
    defaultValues: { email: '', password: '' },
  })

  const onSubmit = handleSubmit(async (values) => {
    setFormError(null)
    try {
      await login(values)
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
      title="Iniciar sesión"
      subtitle="Accede con tu cuenta corporativa para continuar con tu trabajo."
      footer={
        <>
          ¿No tienes cuenta? <Link to={authPathWithRedirect('/register', redirect)}>Crear una</Link>
        </>
      }
    >
      <form className={styles.form} onSubmit={onSubmit} noValidate aria-busy={isSubmitting}>
        {formError && <Alert type="error" showIcon className={styles.alert} message={formError} />}
        <fieldset className={styles.fieldset} disabled={isSubmitting}>
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
              <FieldRow label="Contraseña" error={errors.password?.message}>
                {(aria) => <PasswordInput {...field} {...aria} aria-required autoComplete="current-password" />}
              </FieldRow>
            )}
          />
          <Button type="primary" htmlType="submit" size="large" loading={isSubmitting} className={styles.submit}>
            Iniciar sesión
          </Button>
        </fieldset>
      </form>
    </AuthLayout>
  )
}
