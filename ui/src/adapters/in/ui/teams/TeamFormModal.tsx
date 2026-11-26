import { zodResolver } from '@hookform/resolvers/zod'
import { Alert, Form, Input, Modal } from 'antd'
import { useState } from 'react'
import { Controller, useForm } from 'react-hook-form'
import { z } from 'zod'

import { isAppError } from '@/core/shared/app-error'
import { useUnsavedChangesGuard } from '@/shared/hooks/useUnsavedChangesGuard'
import { UNSAVED_DESCRIPTION, UNSAVED_TITLE } from '@/shared/hooks/unsavedChangesCopy'
import { ColorPicker } from '@/shared/ui/ColorPicker'
import { ConfirmModal } from '@/shared/ui/ConfirmModal'
import { resolveAvatarColor } from './teamColors'

const schema = z.object({
  name: z
    .string()
    .trim()
    .min(2, 'El nombre debe tener entre 2 y 120 caracteres.')
    .max(120, 'El nombre debe tener entre 2 y 120 caracteres.'),
  description: z.string().max(500, 'La descripción no puede superar 500 caracteres.'),
  color: z.string().nullable(),
})

type FormValues = z.infer<typeof schema>

export interface TeamFormValues {
  name: string
  description: string
  /** Hex color for the API, or null for the derived color. */
  avatarColor: string | null
}

export interface TeamFormInitial {
  name: string
  description: string
  /** Hex color or null. */
  avatarColor: string | null
}

interface TeamFormModalProps {
  open: boolean
  title: string
  initial: TeamFormInitial
  onSubmit: (values: TeamFormValues) => Promise<unknown>
  onCancel: () => void
}

/**
 * Create and edit form of a team. Server errors are shown where they belong: team.name_taken on the name
 * field, any other failure (for example team.creation_not_allowed) as an alert inside the modal.
 */
export function TeamFormModal({ open, title, initial, onSubmit, onCancel }: TeamFormModalProps) {
  const [serverError, setServerError] = useState<string | null>(null)
  const [discardOpen, setDiscardOpen] = useState(false)
  const {
    control,
    handleSubmit,
    setError,
    formState: { errors, isSubmitting, isDirty },
  } = useForm<FormValues>({
    resolver: zodResolver(schema),
    values: { name: initial.name, description: initial.description, color: initial.avatarColor },
  })
  // Only an open modal guards navigation: a closed one may keep a stale dirty flag after a save.
  const unsavedGuard = useUnsavedChangesGuard(open && isDirty)

  // Cancel with unsaved edits asks first; X, Esc and the Cancel button all come through here.
  const requestCancel = () => {
    if (isDirty) setDiscardOpen(true)
    else onCancel()
  }

  const submit = handleSubmit(async (values) => {
    setServerError(null)
    try {
      await onSubmit({
        name: values.name.trim(),
        description: values.description.trim(),
        avatarColor: resolveAvatarColor(values.color),
      })
    } catch (error) {
      if (isAppError(error) && error.code === 'team.name_taken') {
        setError('name', { type: 'server', message: 'Ya existe un equipo con ese nombre en la organización.' })
        return
      }
      setServerError(isAppError(error) ? (error.serverMessage ?? error.message) : 'No se pudo guardar el equipo.')
    }
  })

  return (
    <>
    {unsavedGuard}
    <ConfirmModal
      open={discardOpen}
      title={UNSAVED_TITLE}
      description={UNSAVED_DESCRIPTION}
      confirmText="Salir"
      cancelText="Seguir editando"
      onConfirm={() => {
        setDiscardOpen(false)
        onCancel()
      }}
      onCancel={() => setDiscardOpen(false)}
    />
    <Modal
      open={open}
      title={title}
      onCancel={requestCancel}
      onOk={() => void submit()}
      confirmLoading={isSubmitting}
      okText="Guardar"
      cancelText="Cancelar"
      destroyOnHidden
    >
      <Form layout="vertical">
        {serverError && <Alert type="error" showIcon role="alert" message={serverError} />}
        <Form.Item
          label="Nombre del equipo"
          htmlFor="team-name"
          required
          validateStatus={errors.name ? 'error' : ''}
          help={errors.name?.message ?? 'Entre 2 y 120 caracteres.'}
        >
          <Controller
            name="name"
            control={control}
            render={({ field }) => <Input {...field} id="team-name" autoFocus aria-required="true" />}
          />
        </Form.Item>
        <Form.Item label="Descripción" htmlFor="team-description" validateStatus={errors.description ? 'error' : ''} help={errors.description?.message}>
          <Controller
            name="description"
            control={control}
            render={({ field }) => <Input.TextArea {...field} id="team-description" rows={3} />}
          />
        </Form.Item>
        <Form.Item label="Color del equipo">
          <Controller
            name="color"
            control={control}
            render={({ field }) => (
              <ColorPicker label="Color del equipo" value={field.value} onChange={field.onChange} data-testid="team-color" />
            )}
          />
        </Form.Item>
      </Form>
    </Modal>
    </>
  )
}
