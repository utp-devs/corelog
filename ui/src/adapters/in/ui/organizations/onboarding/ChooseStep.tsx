import { Button, Form, Input, Typography } from 'antd'
import { useState } from 'react'
import { useNavigate } from 'react-router-dom'

import { parseInvitationToken } from '@/core/organizations/domain/organization'
import styles from './onboarding.module.css'

export type ChooseStepProps = {
  onCreate: () => void
}

/** Wizard step 1: create an organization, or join one with an invitation link or token. */
export function ChooseStep({ onCreate }: ChooseStepProps) {
  const navigate = useNavigate()
  const [link, setLink] = useState('')
  const [error, setError] = useState<string | null>(null)

  const join = () => {
    const token = parseInvitationToken(link)
    if (!token) {
      setError('Introduce un enlace de invitación válido.')
      return
    }
    navigate(`/invite/${encodeURIComponent(token)}`)
  }

  return (
    <div className={styles.choices}>
      <section className={styles.choice} aria-labelledby="onboarding-create-title">
        <h3 id="onboarding-create-title">Crear organización</h3>
        <Typography.Paragraph>
          Crea un espacio nuevo para tus proyectos, equipos e incidencias. Serás su propietario.
        </Typography.Paragraph>
        <div>
          <Button type="primary" size="large" onClick={onCreate}>
            Crear organización
          </Button>
        </div>
      </section>

      <section className={styles.choice} aria-labelledby="onboarding-join-title">
        <h3 id="onboarding-join-title">Unirme con invitación</h3>
        <Typography.Paragraph>
          Pega el enlace que te han enviado para unirte a una organización existente.
        </Typography.Paragraph>
        <Form layout="vertical" requiredMark={false} onFinish={join} className={styles.joinForm}>
          <Form.Item className={styles.joinField} validateStatus={error ? 'error' : ''} help={error}>
            <Input
              size="large"
              value={link}
              placeholder="https://.../invite/..."
              aria-label="Enlace o código de invitación"
              onChange={(event) => {
                setLink(event.target.value)
                setError(null)
              }}
            />
          </Form.Item>
          <Button htmlType="submit" size="large" className={styles.joinButton}>
            Continuar
          </Button>
        </Form>
      </section>
    </div>
  )
}
