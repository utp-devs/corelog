import { Alert, Button, Empty, Segmented, Skeleton, Space, Tag, Typography } from 'antd'
import { ArrowLeftOutlined, LeftOutlined, RightOutlined } from '@ant-design/icons'
import { useEffect, useMemo, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'

import { CalendarHttpRepository } from '@/adapters/out/http/calendar-http-repository'
import { ProjectHttpRepository } from '@/adapters/out/http/project-http-repository'
import type { CalendarEvent, CalendarEventKind } from '@/core/projects/domain/calendar'
import type { Project } from '@/core/projects/domain/project'
import { PageHeader } from '@/shared/ui/PageHeader'
import styles from './ProjectCalendarPage.module.css'

type View = 'month' | 'list'

const kindLabels: Record<CalendarEventKind, string> = {
  sprint_start: 'Inicio de sprint',
  sprint_end: 'Fin de sprint',
  issue_due: 'Fecha límite',
}
const kindColors: Record<CalendarEventKind, string> = {
  sprint_start: 'processing',
  sprint_end: 'default',
  issue_due: 'warning',
}
const weekdays = ['Lun', 'Mar', 'Mié', 'Jue', 'Vie', 'Sáb', 'Dom']

// Fechas en hora local, en formato AAAA-MM-DD, para no desplazar un dia por la zona horaria.
function isoDate(date: Date): string {
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${date.getFullYear()}-${month}-${day}`
}

// Celdas del mes en semanas que empiezan en lunes; null rellena los huecos antes y despues.
function monthCells(monthStart: Date): (Date | null)[] {
  const daysInMonth = new Date(monthStart.getFullYear(), monthStart.getMonth() + 1, 0).getDate()
  const offset = (monthStart.getDay() + 6) % 7
  const cells: (Date | null)[] = Array.from({ length: offset }, () => null)
  for (let day = 1; day <= daysInMonth; day += 1) {
    cells.push(new Date(monthStart.getFullYear(), monthStart.getMonth(), day))
  }
  while (cells.length % 7 !== 0) cells.push(null)
  return cells
}

export function ProjectCalendarPage() {
  const { id = '' } = useParams()
  const navigate = useNavigate()
  const [view, setView] = useState<View>('month')
  const [monthStart, setMonthStart] = useState(() => {
    const now = new Date()
    return new Date(now.getFullYear(), now.getMonth(), 1)
  })
  const [project, setProject] = useState<Project | undefined>(undefined)
  // Resultado de la carga etiquetado con el mes al que pertenece: asi no hace falta limpiar estado en el efecto.
  const [loaded, setLoaded] = useState<{ key: string; events?: CalendarEvent[]; error?: string } | null>(null)

  const monthEnd = useMemo(
    () => new Date(monthStart.getFullYear(), monthStart.getMonth() + 1, 0),
    [monthStart],
  )
  const monthKey = `${id}/${isoDate(monthStart)}`

  useEffect(() => {
    let active = true
    new ProjectHttpRepository().get(id)
      .then((result) => { if (active) setProject(result) })
      .catch(() => undefined)
    return () => { active = false }
  }, [id])

  useEffect(() => {
    let active = true
    const key = monthKey
    new CalendarHttpRepository().events(id, isoDate(monthStart), isoDate(monthEnd))
      .then((result) => { if (active) setLoaded({ key, events: result }) })
      .catch(() => {
        if (active) setLoaded({ key, error: 'No se pudo cargar el calendario. Verifica que tengas acceso al proyecto e intenta de nuevo.' })
      })
    return () => { active = false }
  }, [monthKey, id, monthStart, monthEnd])

  const current = loaded?.key === monthKey ? loaded : null
  const events = current?.events
  const error = current?.error

  const eventsByDate = useMemo(() => {
    const map = new Map<string, CalendarEvent[]>()
    for (const event of events ?? []) {
      map.set(event.date, [...(map.get(event.date) ?? []), event])
    }
    return map
  }, [events])

  const monthLabel = monthStart.toLocaleDateString('es-ES', { month: 'long', year: 'numeric' })
  const shift = (months: number) => setMonthStart((previous) => new Date(previous.getFullYear(), previous.getMonth() + months, 1))

  const renderEvent = (event: CalendarEvent) => (
    <li key={`${event.kind}-${event.sourceId}-${event.date}`} className={styles.event}>
      <Tag color={kindColors[event.kind]}>{kindLabels[event.kind]}</Tag>
      <Link to={event.link}>{event.title}</Link>
    </li>
  )

  return (
    <div>
      <PageHeader
        title={project ? `${project.key} · Calendario` : 'Calendario del proyecto'}
        description="Inicios y fines de sprint y fechas límite de incidencias."
        actions={(
          <Space>
            <Button onClick={() => navigate(`/projects/${id}/sprints`)}>Sprints</Button>
            <Button icon={<ArrowLeftOutlined />} onClick={() => navigate(`/projects/${id}/backlog`)}>Backlog</Button>
          </Space>
        )}
      />

      <div className={styles.toolbar}>
        <Space>
          <Button icon={<LeftOutlined />} aria-label="Mes anterior" onClick={() => shift(-1)} />
          <Typography.Title level={5} className={styles.monthTitle}>{monthLabel}</Typography.Title>
          <Button icon={<RightOutlined />} aria-label="Mes siguiente" onClick={() => shift(1)} />
        </Space>
        <Segmented
          value={view}
          onChange={(value) => setView(value as View)}
          options={[{ label: 'Mes', value: 'month' }, { label: 'Lista', value: 'list' }]}
        />
      </div>

      {error && <Alert type="error" showIcon message={error} />}
      {!error && !events && <Skeleton active paragraph={{ rows: 5 }} />}

      {events && !error && (events.length === 0 ? (
        <Empty description="No hay hitos en este mes." />
      ) : view === 'month' ? (
        <div className={styles.grid} role="grid" aria-label={`Calendario de ${monthLabel}`}>
          {weekdays.map((weekday) => (
            <div key={weekday} role="columnheader" className={styles.weekday}>{weekday}</div>
          ))}
          {monthCells(monthStart).map((cell, index) => {
            if (!cell) return <div key={`blank-${index}`} className={styles.blank} role="gridcell" aria-hidden="true" />
            const key = isoDate(cell)
            const dayEvents = eventsByDate.get(key) ?? []
            return (
              <div key={key} role="gridcell" className={styles.day} aria-label={`${cell.getDate()} de ${monthLabel}`}>
                <span className={styles.dayNumber}>{cell.getDate()}</span>
                <ul className={styles.dayEvents}>{dayEvents.map(renderEvent)}</ul>
              </div>
            )
          })}
        </div>
      ) : (
        <div className={styles.list}>
          {Array.from(eventsByDate.entries()).map(([date, dayEvents]) => (
            <section key={date} aria-label={date}>
              <Typography.Title level={5}>{date}</Typography.Title>
              <ul className={styles.dayEvents}>{dayEvents.map(renderEvent)}</ul>
            </section>
          ))}
        </div>
      ))}
    </div>
  )
}
