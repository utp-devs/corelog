import { Modal, Typography } from 'antd'
import type { ReactNode } from 'react'

type ConfirmModalProps = {
  open: boolean
  title: string
  /** Consecuencia de la acción, visible antes de confirmar. */
  description?: ReactNode
  confirmText?: string
  cancelText?: string
  /** Marca la acción como destructiva: el botón de confirmar usa el estilo de peligro. */
  danger?: boolean
  loading?: boolean
  onConfirm: () => void
  onCancel: () => void
}

/** Confirmación de una acción, con la consecuencia escrita en el cuerpo del modal. */
export function ConfirmModal({
  open,
  title,
  description,
  confirmText = 'Confirmar',
  cancelText = 'Cancelar',
  danger = false,
  loading = false,
  onConfirm,
  onCancel,
}: ConfirmModalProps) {
  return (
    <Modal
      open={open}
      title={title}
      okText={confirmText}
      cancelText={cancelText}
      okButtonProps={{ danger, loading }}
      onOk={onConfirm}
      onCancel={onCancel}
      destroyOnHidden
    >
      {description && <Typography.Paragraph>{description}</Typography.Paragraph>}
    </Modal>
  )
}
