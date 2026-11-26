import { zodResolver } from '@hookform/resolvers/zod'
import { Button, Input, Select } from 'antd'
import { Controller, useForm } from 'react-hook-form'
import { z } from 'zod'

import type { CreateTicketInput } from '@/core/tickets/application/ports/ticket-repository.port'
import type { IssueType, TicketPriority } from '@/core/tickets/domain/ticket'
import { FieldRow } from '@/shared/ui/FieldRow'
import { RichTextEditor, type UserLite } from '@/shared/ui/RichTextEditor'
import { Section } from '@/shared/ui/Section'
import { issueTypeLabels } from './ticketLabels'
import styles from './TicketForm.module.css'

const schema = z.object({
  title: z.string().min(3, 'El título es demasiado corto'),
  description: z.string().min(3, 'La descripción es demasiado corta'),
  priority: z.enum(['low', 'medium', 'high', 'critical']),
  issueType: z.enum(['task', 'bug', 'story', 'epic', 'subtask']),
})

type FormValues = z.infer<typeof schema>

const issueTypeOptions = (Object.keys(issueTypeLabels) as IssueType[]).map((value) => ({
  value,
  label: issueTypeLabels[value],
}))

const priorityOptions: { value: TicketPriority; label: string }[] = [
  { value: 'low', label: 'Baja' },
  { value: 'medium', label: 'Media' },
  { value: 'high', label: 'Alta' },
  { value: 'critical', label: 'Crítica' },
]

interface TicketFormProps {
  onSubmit: (input: CreateTicketInput) => Promise<unknown>
  onCancel: () => void
  isSubmitting: boolean
  searchUsers?: (query: string) => Promise<UserLite[]>
}

/** Formulario de alta de incidencia: datos principales y descripción en Markdown. */
export function TicketForm({ onSubmit, onCancel, isSubmitting, searchUsers }: TicketFormProps) {
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
    <form className={styles.form} onSubmit={(event) => void submit(event)} noValidate aria-label="Nueva incidencia">
      <Section title="Datos principales">
        <div className={styles.grid}>
          <div className={styles.wide}>
            <Controller
              name="title"
              control={control}
              render={({ field }) => (
                <FieldRow label="Título" required error={errors.title?.message} helper="Describe el problema en una frase.">
                  {(props) => (
                    <Input
                      {...props}
                      value={field.value}
                      onChange={field.onChange}
                      onBlur={field.onBlur}
                      name={field.name}
                      ref={field.ref}
                      status={errors.title ? 'error' : undefined}
                      maxLength={200}
                    />
                  )}
                </FieldRow>
              )}
            />
          </div>
          <Controller
            name="issueType"
            control={control}
            render={({ field }) => (
              <FieldRow label="Tipo" required error={errors.issueType?.message}>
                {(props) => (
                  <Select<IssueType>
                    {...props}
                    value={field.value}
                    onChange={field.onChange}
                    onBlur={field.onBlur}
                    options={issueTypeOptions}
                    status={errors.issueType ? 'error' : undefined}
                  />
                )}
              </FieldRow>
            )}
          />
          <Controller
            name="priority"
            control={control}
            render={({ field }) => (
              <FieldRow label="Prioridad" required error={errors.priority?.message}>
                {(props) => (
                  <Select<TicketPriority>
                    {...props}
                    value={field.value}
                    onChange={field.onChange}
                    onBlur={field.onBlur}
                    options={priorityOptions}
                    status={errors.priority ? 'error' : undefined}
                  />
                )}
              </FieldRow>
            )}
          />
        </div>
      </Section>

      <Section title="Descripción">
        <Controller
          name="description"
          control={control}
          render={({ field }) => (
            <div className={styles.editor}>
              <RichTextEditor
                ariaLabel="Descripción"
                value={field.value}
                onChange={field.onChange}
                onSubmit={() => void submit()}
                placeholder="Describe el problema, los pasos para reproducirlo y el resultado esperado. Usa @ para mencionar."
                minRows={8}
                searchUsers={searchUsers}
                disabled={isSubmitting}
              />
              {errors.description && (
                <p className={styles.error} role="alert">{errors.description.message}</p>
              )}
            </div>
          )}
        />
      </Section>

      <div className={styles.actions}>
        <Button onClick={onCancel}>Cancelar</Button>
        <Button type="primary" htmlType="submit" loading={isSubmitting}>
          Crear incidencia
        </Button>
      </div>
    </form>
  )
}
