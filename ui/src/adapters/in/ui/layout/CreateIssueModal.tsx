import { Button, Modal, Typography } from 'antd'
import { useNavigate } from 'react-router-dom'

export type CreateIssueModalProps = {
  open: boolean
  onClose: () => void
}

/** Placeholder until P4 ships the create-issue dialog: it points to the existing form. */
export function CreateIssueModal({ open, onClose }: CreateIssueModalProps) {
  const navigate = useNavigate()

  const goToForm = () => {
    onClose()
    navigate('/issues/new')
  }

  return (
    <Modal
      open={open}
      title="Crear incidencia"
      onCancel={onClose}
      width={520}
      footer={[
        <Button key="cancel" onClick={onClose}>Cancelar</Button>,
        <Button key="form" type="primary" onClick={goToForm}>Ir al formulario de incidencia</Button>,
      ]}
    >
      <Typography.Paragraph>
        El asistente rápido para crear incidencias llegará en una próxima versión. Por ahora puedes usar el formulario completo.
      </Typography.Paragraph>
    </Modal>
  )
}
