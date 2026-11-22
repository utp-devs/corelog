import { zodResolver } from '@hookform/resolvers/zod'
import { App, Button, Card, Form, Input, Typography } from 'antd'
import { Controller, useForm } from 'react-hook-form'
import { Link, useNavigate } from 'react-router-dom'
import { z } from 'zod'

import { useAuthStore } from '@/adapters/in/state/auth.store'
import styles from './AuthPage.module.css'

const schema = z.object({
  email: z.string().email('Introduce un correo electrónico válido'),
  password: z.string().min(1, 'La contraseña es obligatoria'),
})

type FormValues = z.infer<typeof schema>

export function LoginPage() {
  const login = useAuthStore((s) => s.login)
  const navigate = useNavigate()
  const { message } = App.useApp()

  const {
    control,
    handleSubmit,
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
      message.error(error instanceof Error ? error.message : 'No se pudo iniciar sesión')
    }
  })

  return (
    <div className={styles.wrapper}>
      <Card className={styles.card}>
        <img
          src="/brand/corelog-banner-480.png"
          srcSet="/brand/corelog-banner-480.png 480w, /brand/corelog-banner-960.png 960w"
          sizes="280px"
          alt="CoreLog"
          width={280}
          className={styles.banner}
        />
        <Typography.Title level={3} className={styles.title}>
          Iniciar sesión en CoreLog
        </Typography.Title>
        <Form layout="vertical" onFinish={onSubmit}>
          <Form.Item label="Correo electrónico" validateStatus={errors.email ? 'error' : ''} help={errors.email?.message}>
            <Controller
              name="email"
              control={control}
              render={({ field }) => <Input {...field} type="email" autoComplete="email" aria-label="Correo electrónico" />}
            />
          </Form.Item>
          <Form.Item label="Contraseña" validateStatus={errors.password ? 'error' : ''} help={errors.password?.message}>
            <Controller
              name="password"
              control={control}
              render={({ field }) => <Input.Password {...field} autoComplete="current-password" aria-label="Contraseña" />}
            />
          </Form.Item>
          <Button type="primary" htmlType="submit" block loading={isSubmitting}>
            Iniciar sesión
          </Button>
        </Form>
        <Typography.Paragraph className={styles.footer}>
          ¿No tienes cuenta? <Link to="/register">Crear una</Link>
        </Typography.Paragraph>
      </Card>
    </div>
  )
}
