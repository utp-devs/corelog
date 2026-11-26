import { zodResolver } from '@hookform/resolvers/zod'
import { App, Button, Form, Input } from 'antd'
import { Controller, useForm } from 'react-hook-form'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { z } from 'zod'

import { useAuthStore } from '@/adapters/in/state/auth.store'
import { AuthLayout } from './AuthLayout'
import styles from './AuthPage.module.css'
import { authPathWithRedirect, safeRedirect } from './safe-redirect'

const schema = z.object({
  name: z.string().min(2, 'El nombre debe tener al menos 2 caracteres'),
  email: z.string().email('Introduce un correo electrónico válido'),
  password: z.string().min(8, 'La contraseña debe tener al menos 8 caracteres'),
})

type FormValues = z.infer<typeof schema>

export function RegisterPage() {
  const register = useAuthStore((s) => s.register)
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const redirect = searchParams.get('redirect')
  const { message } = App.useApp()

  const {
    control,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: { name: '', email: '', password: '' },
  })

  const onSubmit = handleSubmit(async (values) => {
    try {
      await register(values)
      navigate(safeRedirect(redirect))
    } catch (error) {
      message.error(error instanceof Error ? error.message : 'No se pudo crear la cuenta')
    }
  })

  return (
    <AuthLayout
      title="Crear cuenta"
      subtitle="Regístrate para empezar a trabajar en CoreLog."
      footer={<>¿Ya tienes cuenta? <Link to={authPathWithRedirect('/login', redirect)}>Iniciar sesión</Link></>}
    >
      <Form layout="vertical" onFinish={onSubmit} requiredMark={false} className={styles.form}>
        <Form.Item label="Nombre" validateStatus={errors.name ? 'error' : ''} help={errors.name?.message}>
          <Controller
            name="name"
            control={control}
            render={({ field }) => <Input {...field} size="large" autoComplete="name" aria-label="Nombre" />}
          />
        </Form.Item>
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
              <Input.Password {...field} size="large" autoComplete="new-password" aria-label="Contraseña" />
            )}
          />
        </Form.Item>
        <Button type="primary" htmlType="submit" block size="large" loading={isSubmitting}>
          Crear cuenta
        </Button>
      </Form>
    </AuthLayout>
  )
}
