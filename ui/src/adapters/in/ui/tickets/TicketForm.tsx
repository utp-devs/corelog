import { zodResolver } from '@hookform/resolvers/zod'
import { Button, Form, Input, Select } from 'antd'
import { Controller, useForm } from 'react-hook-form'
import { z } from 'zod'

import type { CreateTicketInput } from '@/core/tickets/application/ports/ticket-repository.port'
import type { IssueType } from '@/core/tickets/domain/ticket'
import styles from './TicketForm.module.css'

const schema = z.object({
  title: z.string().min(3, 'El titulo es demasiado corto'),
  description: z.string().min(3, 'La descripcion es demasiado corta'),
  priority: z.enum(['low', 'medium', 'high', 'critical']),
  issueType: z.enum(['task', 'bug', 'story', 'epic', 'subtask']),
})

type FormValues = z.infer<typeof schema>

interface TicketFormProps {
  onSubmit: (input: CreateTicketInput) => Promise<unknown>
  isSubmitting: boolean
}

export function TicketForm({ onSubmit, isSubmitting }: TicketFormProps) {
  const {
    control,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: { title: '', description: '', priority: 'medium', issueType: 'task' },
  })

  const submit = handleSubmit(async (values) => {
    try {
      await onSubmit(values)
      reset()
    } catch {
      return
    }
  })

  return (
    <Form className={styles.form} layout="vertical" onFinish={submit}>
      <Form.Item label="Titulo" validateStatus={errors.title ? 'error' : ''} help={errors.title?.message}>
        <Controller name="title" control={control} render={({ field }) => <Input {...field} />} />
      </Form.Item>
      <Form.Item label="Tipo" validateStatus={errors.issueType ? 'error' : ''} help={errors.issueType?.message}>
        <Controller
          name="issueType"
          control={control}
          render={({ field }) => (
            <Select<IssueType>
              {...field}
              options={[
                { value: 'task', label: 'Tarea' },
                { value: 'bug', label: 'Error' },
                { value: 'story', label: 'Historia' },
                { value: 'epic', label: 'Epic' },
                { value: 'subtask', label: 'Subtarea' },
              ]}
            />
          )}
        />
      </Form.Item>
      <Form.Item
        label="Descripcion"
        validateStatus={errors.description ? 'error' : ''}
        help={errors.description?.message}
      >
        <Controller name="description" control={control} render={({ field }) => <Input.TextArea {...field} rows={4} />} />
      </Form.Item>
      <Form.Item label="Prioridad" validateStatus={errors.priority ? 'error' : ''} help={errors.priority?.message}>
        <Controller
          name="priority"
          control={control}
          render={({ field }) => (
            <Select
              {...field}
              options={[
                { value: 'low', label: 'Baja' },
                { value: 'medium', label: 'Media' },
                { value: 'high', label: 'Alta' },
                { value: 'critical', label: 'Critica' },
              ]}
            />
          )}
        />
      </Form.Item>
      <Button type="primary" htmlType="submit" loading={isSubmitting}>
        Crear incidencia
      </Button>
    </Form>
  )
}
