import { BellOutlined } from '@ant-design/icons'
import { Card, Space, Switch, Tag, Typography } from 'antd'
import { useEffect, useState } from 'react'

import { extractErrorMessage } from '@/adapters/out/http/api-client'
import { NotificationHttpRepository } from '@/adapters/out/http/notification-http-repository'
import styles from './ProfilePage.module.css'

const notificationPrefs = new NotificationHttpRepository()

/** In-app notification preference (live). Email channel is not available yet. */
export function NotificationsSection() {
  // null = not loaded yet (or not available): the switch stays disabled.
  const [inAppEnabled, setInAppEnabled] = useState<boolean | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [status, setStatus] = useState<string | null>(null)

  useEffect(() => {
    let active = true
    notificationPrefs
      .getPreferences()
      .then((prefs) => { if (active) setInAppEnabled(prefs.inApp) })
      .catch(() => undefined)
    return () => { active = false }
  }, [])

  const handleInAppToggle = async (checked: boolean) => {
    const previous = inAppEnabled
    setInAppEnabled(checked)
    setError(null)
    try {
      const saved = await notificationPrefs.updatePreferences(checked)
      setInAppEnabled(saved.inApp)
      setStatus(saved.inApp ? 'Avisos en la aplicación activados' : 'Avisos en la aplicación desactivados')
    } catch (failure) {
      setInAppEnabled(previous)
      setError(extractErrorMessage(failure, 'No se pudo guardar la preferencia'))
    }
  }

  return (
    <section id="perfil-notificaciones" aria-labelledby="perfil-notificaciones-title" className={styles.section}>
      <Typography.Title level={4} id="perfil-notificaciones-title" className={styles.sectionTitle}>Notificaciones</Typography.Title>
      <Card title={<><BellOutlined /> Avisos</>} className={styles.card}>
        <Space orientation="vertical" className={styles.fullWidth}>
          <div className={styles.rowBetween}>
            <Typography.Text>Avisos en la aplicación</Typography.Text>
            <Switch
              checked={inAppEnabled ?? false}
              disabled={inAppEnabled === null}
              loading={inAppEnabled === null}
              onChange={(checked) => void handleInAppToggle(checked)}
              aria-label="Avisos en la aplicación"
            />
          </div>
          <div className={styles.rowBetween}>
            <Typography.Text>Correo electrónico</Typography.Text>
            <Space>
              <Tag color="default">Próximamente</Tag>
              <Switch disabled aria-label="Correo electrónico" />
            </Space>
          </div>
          {status && <Typography.Text type="secondary" role="status">{status}</Typography.Text>}
          {error && <Typography.Text type="danger" role="alert">{error}</Typography.Text>}
        </Space>
      </Card>
    </section>
  )
}
