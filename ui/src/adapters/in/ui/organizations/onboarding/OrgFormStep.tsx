import { zodResolver } from '@hookform/resolvers/zod'
import { Alert, Button, Form, Input, Space } from 'antd'
import { useState } from 'react'
import { Controller, useForm } from 'react-hook-form'
import { z } from 'zod'

import { useOrgStore } from '@/adapters/in/state/org.store'
import { isValidOrgSlug, slugFromName } from '@/core/organizations/domain/organization'
import { fieldErrors, isAppError } from '@/core/shared/app-error'
import { orgCreateErrorMessage } from '../org-copy'
import styles from './onboarding.module.css'

const SLUG_RULE = 'Usa 3 a 40 caracteres: minúsculas, números y guiones, sin guion al principio ni al final.'

const schema = z.object({
  name: z.string().trim().min(1, 'El nombre es obligatorio').max(120, 'Usa como máximo 120 caracteres'),
  slug: z.string().refine(isValidOrgSlug, SLUG_RULE),
})

type FormValues = z.infer<typeof schema>

const SERVER_FIELDS = ['name', 'slug'] as const

export type OrgFormStepProps = {
  onBack?: () => void
  onCreated: () => void
}

/** Wizard step 2: name and slug of the new organization, with live slug validation and server errors. */
export function OrgFormStep({ onBack, onCreated }: OrgFormStepProps) {
  const createOrg = useOrgStore((s) => s.createOrg)
  const [slugEdited, setSlugEdited] = useState(false)
  const [formError, setFormError] = useState<string | null>(null)

  const {
    control,
    handleSubmit,
    setError,
    setValue,
    formState: { errors, isSubmitting },
  } = useForm<FormValues>({
    resolver: zodResolver(schema),
    mode: 'onChange',
    defaultValues: { name: '', slug: '' },
  })

  const onSubmit = handleSubmit(async (values) => {
    setFormError(null)
    try {
      await createOrg({ name: values.name.trim(), slug: values.slug })
      onCreated()
    } catch (error) {
      const fields = fieldErrors(error)
      const invalid = SERVER_FIELDS.filter((name) => fields[name])
      if (invalid.length > 0) {
        invalid.forEach((name) => setError(name, { type: 'server', message: fields[name] }))
        return
      }
      if (isAppError(error) && error.code === 'org.slug_taken') {
        setError('slug', { type: 'server', message: 'Ese identificador ya está en uso. Elige otro.' })
        return
      }
      if (isAppError(error) && error.code === 'org.slug_reserved') {
        setError('slug', { type: 'server', message: 'Ese identificador está reservado. Elige otro.' })
        return
      }
      setFormError(orgCreateErrorMessage(error))
    }
  })

  return (
    <Form layout="vertical" onFinish={onSubmit} requiredMark={false} aria-label="Datos de la organización" className={styles.formGrid}>
      <Form.Item className={styles.half} label="Nombre de la organización" validateStatus={errors.name ? 'error' : ''} help={errors.name?.message}>
        <Controller
          name="name"
          control={control}
          render={({ field }) => (
            <Input
              {...field}
              size="large"
              autoComplete="organization"
              aria-label="Nombre de la organización"
              onChange={(event) => {
                field.onChange(event)
                if (!slugEdited) {
                  setValue('slug', slugFromName(event.target.value), { shouldValidate: true })
                }
              }}
            />
          )}
        />
      </Form.Item>

      <Form.Item
        className={styles.half}
        label="Identificador (slug)"
        validateStatus={errors.slug ? 'error' : ''}
        help={errors.slug?.message ?? SLUG_RULE}
      >
        <Controller
          name="slug"
          control={control}
          render={({ field }) => (
            <Input
              {...field}
              size="large"
              aria-label="Identificador de la organización"
              onChange={(event) => {
                setSlugEdited(true)
                field.onChange(event.target.value.toLowerCase())
              }}
            />
          )}
        />
      </Form.Item>

      {formError && <Alert type="error" showIcon message={formError} className={styles.full} />}

      <Space className={styles.full}>
        {onBack && (
          <Button size="large" onClick={onBack}>
            Atrás
          </Button>
        )}
        <Button type="primary" size="large" htmlType="submit" loading={isSubmitting}>
          Crear organización
        </Button>
      </Space>
    </Form>
  )
}
