import { zodResolver } from '@hookform/resolvers/zod'
import { App, Button, Card, Form, Input, Typography } from 'antd'
import { Controller, useForm } from 'react-hook-form'
import { Link, useNavigate } from 'react-router-dom'
import { z } from 'zod'

import { useAuthStore } from '@/adapters/in/state/auth.store'
import styles from './AuthPage.module.css'

const schema = z.object({
  name: z.string().min(2, 'El nombre es demasiado corto'),
  email: z.string().email('Introduce un correo valido'),
  password: z.string().min(8, 'La contrasena debe tener al menos 8 caracteres'),
})

type FormValues = z.infer<typeof schema>

export function RegisterPage() {
  const register = useAuthStore((s) => s.register)
  const navigate = useNavigate()
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
      navigate('/issues')
    } catch (error) {
      message.error(error instanceof Error ? error.message : 'No se pudo crear la cuenta')
    }
  })

  return (
    <div className={styles.wrapper}>
      <Card className={styles.card}>
        <Typography.Title level={3} className={styles.title}>
          Crear cuenta en CoreLog
        </Typography.Title>
        <Form layout="vertical" onFinish={onSubmit}>
          <Form.Item label="Nombre" validateStatus={errors.name ? 'error' : ''} help={errors.name?.message}>
            <Controller name="name" control={control} render={({ field }) => <Input {...field} />} />
          </Form.Item>
          <Form.Item label="Correo electronico" validateStatus={errors.email ? 'error' : ''} help={errors.email?.message}>
            <Controller name="email" control={control} render={({ field }) => <Input {...field} type="email" />} />
          </Form.Item>
          <Form.Item label="Contrasena" validateStatus={errors.password ? 'error' : ''} help={errors.password?.message}>
            <Controller
              name="password"
              control={control}
              render={({ field }) => <Input.Password {...field} />}
            />
          </Form.Item>
          <Button type="primary" htmlType="submit" block loading={isSubmitting}>
            Crear cuenta
          </Button>
        </Form>
        <Typography.Paragraph className={styles.footer}>
          ¿Ya tienes cuenta? <Link to="/login">Iniciar sesion</Link>
        </Typography.Paragraph>
      </Card>
    </div>
  )
}
