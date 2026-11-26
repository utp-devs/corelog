import { zodResolver } from '@hookform/resolvers/zod'
import { App, Button, Form, Input } from 'antd'
import { Controller, useForm } from 'react-hook-form'
import { Link, useNavigate } from 'react-router-dom'
import { z } from 'zod'

import { useAuthStore } from '@/adapters/in/state/auth.store'
import { fieldErrors } from '@/core/shared/app-error'
import { AuthLayout } from './AuthLayout'
import styles from './AuthPage.module.css'

const schema = z.object({
  email: z.string().email('Introduce un correo electrónico válido'),
  password: z.string().min(1, 'La contraseña es obligatoria'),
})

type FormValues = z.infer<typeof schema>

const SERVER_FIELDS = ['email', 'password'] as const

export function LoginPage() {
  const login = useAuthStore((s) => s.login)
  const navigate = useNavigate()
  const { message } = App.useApp()

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
    try {
      await login(values)
      navigate('/issues')
    } catch (error) {
      const fields = fieldErrors(error)
      const invalid = SERVER_FIELDS.filter((name) => fields[name])
      if (invalid.length > 0) {
        invalid.forEach((name) => setError(name, { type: 'server', message: fields[name] }))
        return
      }
      message.error(error instanceof Error ? error.message : 'No se pudo iniciar sesión')
    }
  })

  return (
    <AuthLayout
      title="Iniciar sesión"
      subtitle="Accede con tu cuenta corporativa para continuar con tu trabajo."
      footer={<>¿No tienes cuenta? <Link to="/register">Crear una</Link></>}
    >
      <Form layout="vertical" onFinish={onSubmit} requiredMark={false} className={styles.form}>
        <Form.Item label="Correo electrónico" validateStatus={errors.email ? 'error' : ''} help={errors.email?.message}>
          <Controller
            name="email"
            control={control}
            render={({ field }) => (
              <Input {...field} size="large" type="email" autoComplete="email" aria-label="Correo electrónico" />
            )}
          />
        </Form.Item>
        <Form.Item label="Contraseña" validateStatus={errors.password ? 'error' : ''} help={errors.password?.message}>
          <Controller
            name="password"
            control={control}
            render={({ field }) => (
              <Input.Password {...field} size="large" autoComplete="current-password" aria-label="Contraseña" />
            )}
          />
        </Form.Item>
        <Button type="primary" htmlType="submit" block size="large" loading={isSubmitting}>
          Iniciar sesión
        </Button>
      </Form>
    </AuthLayout>
  )
}
