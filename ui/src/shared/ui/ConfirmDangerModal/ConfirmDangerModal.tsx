import { useId, useState, type ReactNode } from 'react'
import { Modal, Typography } from 'antd'

import styles from './ConfirmDangerModal.module.css'
import { isConfirmationMatch } from './confirmation'

type ConfirmDangerModalProps = {
  open: boolean
  title: string
  /** Consecuencia de la acción, visible antes de confirmar. */
  description?: ReactNode
  /** Texto exacto que el usuario debe escribir (nombre del proyecto u organización). */
  confirmation: string
  confirmText?: string
  loading?: boolean
  onConfirm: () => void
  onCancel: () => void
}

/** Confirmación de una acción destructiva: el botón solo se activa al escribir el nombre exacto. */
export function ConfirmDangerModal({
  open,
  title,
  description,
  confirmation,
  confirmText = 'Eliminar',
  loading = false,
  onConfirm,
  onCancel,
}: ConfirmDangerModalProps) {
  const [typed, setTyped] = useState('')
  const inputId = useId()
  const matches = isConfirmationMatch(typed, confirmation)

  return (
    <Modal
      open={open}
      title={title}
      okText={confirmText}
      cancelText="Cancelar"
      okButtonProps={{ danger: true, disabled: !matches, loading }}
      onOk={() => {
        if (matches) onConfirm()
      }}
      onCancel={onCancel}
      afterClose={() => setTyped('')}
      destroyOnHidden
    >
      {description && <Typography.Paragraph>{description}</Typography.Paragraph>}
      <div className={styles.field}>
        <label htmlFor={inputId}>
          Escribe <strong>{confirmation}</strong> para confirmar
        </label>
        <input
          id={inputId}
          type="text"
          autoComplete="off"
          className={styles.input}
          value={typed}
          onChange={(event) => setTyped(event.target.value)}
        />
      </div>
    </Modal>
  )
}
