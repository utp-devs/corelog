import { BellOutlined } from '@ant-design/icons'
import { Badge, Button, Empty, Popover, Typography } from 'antd'
import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'

import { NotificationHttpRepository } from '@/adapters/out/http/notification-http-repository'
import type { Notification } from '@/core/notifications/domain/notification'
import styles from './NotificationsBell.module.css'

// Sondeo del contador de no leidas (notificaciones in-app con sondeo).
const POLL_INTERVAL_MS = 30_000

export function NotificationsBell() {
  const navigate = useNavigate()
  const [repo] = useState(() => new NotificationHttpRepository())
  const [count, setCount] = useState(0)
  const [items, setItems] = useState<Notification[]>([])
  const [open, setOpen] = useState(false)

  useEffect(() => {
    let active = true
    const refresh = () => {
      repo.unreadCount()
        .then((value) => { if (active) setCount(value) })
        .catch(() => undefined)
    }
    refresh()
    const timer = window.setInterval(refresh, POLL_INTERVAL_MS)
    return () => {
      active = false
      window.clearInterval(timer)
    }
  }, [repo])

  useEffect(() => {
    if (!open) return
    let active = true
    repo.list()
      .then((list) => { if (active) setItems(list) })
      .catch(() => undefined)
    return () => { active = false }
  }, [open, repo])

  const markAll = () => {
    repo.markAllRead()
      .then(() => {
        setCount(0)
        setItems((previous) => previous.map((item) => ({ ...item, read: true })))
      })
      .catch(() => undefined)
  }

  // Abre el aviso: lo marca como leido (si no lo estaba) y lleva a su pantalla.
  const openItem = (item: Notification) => {
    if (!item.read) {
      repo.markRead(item.id)
        .then(() => {
          setCount((value) => Math.max(0, value - 1))
          setItems((previous) => previous.map((entry) => (entry.id === item.id ? { ...entry, read: true } : entry)))
        })
        .catch(() => undefined)
    }
    setOpen(false)
    if (item.link) navigate(item.link)
  }

  const panel = (
    <div className={styles.panel}>
      <div className={styles.header}>
        <Typography.Text strong>Notificaciones</Typography.Text>
        <Button type="link" size="small" onClick={markAll} disabled={count === 0}>Marcar todas</Button>
      </div>
      {items.length === 0 ? (
        <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="Sin notificaciones" />
      ) : (
        <ul className={styles.list}>
          {items.map((item) => (
            <li key={item.id}>
              <button type="button" className={item.read ? styles.item : styles.itemUnread} onClick={() => openItem(item)}>
                {item.message}
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  )

  return (
    <Popover open={open} onOpenChange={setOpen} trigger="click" placement="bottomRight" content={panel}>
      <Badge count={count} overflowCount={99} size="small">
        <Button
          type="text"
          icon={<BellOutlined />}
          aria-label={count > 0 ? `Notificaciones, ${count} sin leer` : 'Notificaciones'}
        />
      </Badge>
    </Popover>
  )
}
