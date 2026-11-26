import { Button, Segmented, Skeleton } from 'antd'
import { LeftOutlined, RightOutlined } from '@ant-design/icons'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { Link, useParams } from 'react-router-dom'

import { CalendarHttpRepository } from '@/adapters/out/http/calendar-http-repository'
import type { CalendarEvent, CalendarEventKind } from '@/core/projects/domain/calendar'
import { EmptyState } from '@/shared/ui/EmptyState'
import { ErrorPage } from '@/shared/ui/ErrorPage'
import { ErrorState } from '@/shared/ui/ErrorState'
import { Lozenge, type LozengeTone } from '@/shared/ui/Lozenge'
import { PageContainer } from '@/shared/ui/PageContainer'
import { PageHeader } from '@/shared/ui/PageHeader'
import { accessVariantOf, type AccessVariant } from './projectLoadError'
import styles from './ProjectCalendarPage.module.css'

type View = 'month' | 'list'

const kindLabels: Record<CalendarEventKind, string> = {
  sprint_start: 'Inicio de sprint',
  sprint_end: 'Fin de sprint',
  issue_due: 'Fecha límite',
}
const kindTones: Record<CalendarEventKind, LozengeTone> = {
  sprint_start: 'info',
  sprint_end: 'neutral',
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

type LoadState = { key: string; events?: CalendarEvent[]; error?: { variant: AccessVariant | null; message: string } }

export function ProjectCalendarPage() {
  const { id = '' } = useParams()
  const [view, setView] = useState<View>('month')
  const [monthStart, setMonthStart] = useState(() => {
    const now = new Date()
    return new Date(now.getFullYear(), now.getMonth(), 1)
  })
  const [attempt, setAttempt] = useState(0)
  // Resultado de la carga etiquetado con el mes al que pertenece: asi no hace falta limpiar estado en el efecto.
  const [loaded, setLoaded] = useState<LoadState | null>(null)

  const monthEnd = useMemo(
    () => new Date(monthStart.getFullYear(), monthStart.getMonth() + 1, 0),
    [monthStart],
  )
  const monthKey = `${id}/${isoDate(monthStart)}/${attempt}`

  useEffect(() => {
    let active = true
    const key = monthKey
    new CalendarHttpRepository().events(id, isoDate(monthStart), isoDate(monthEnd))
      .then((result) => { if (active) setLoaded({ key, events: result }) })
      .catch((failure: unknown) => {
        if (active) setLoaded({
          key,
          error: {
            variant: accessVariantOf(failure),
            message: 'No se pudo cargar el calendario. Verifica que tengas acceso al proyecto e intenta de nuevo.',
          },
        })
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
  const retry = useCallback(() => setAttempt((value) => value + 1), [])

  if (error?.variant) {
    return (
      <PageContainer variant="fluid" pageName="calendar">
        <div className={styles.frame}>
          <ErrorPage variant={error.variant} />
        </div>
      </PageContainer>
    )
  }

  const renderEvent = (event: CalendarEvent) => (
    <li key={`${event.kind}-${event.sourceId}-${event.date}`} className={styles.event}>
      <Lozenge tone={kindTones[event.kind]}>{kindLabels[event.kind]}</Lozenge>
      <Link to={event.link} className={styles.eventLink} title={event.title}>{event.title}</Link>
    </li>
  )

  return (
    <PageContainer variant="fluid" pageName="calendar" ready={!!events && !error} fill>
      <div className={styles.frame}>
        <PageHeader title="Calendario" description="Inicios y fines de sprint y fechas límite de incidencias." />

        <div className={styles.toolbar}>
          <div className={styles.monthNav}>
            <Button icon={<LeftOutlined aria-hidden="true" />} aria-label="Mes anterior" onClick={() => shift(-1)} />
            <h2 className={styles.monthTitle}>{monthLabel}</h2>
            <Button icon={<RightOutlined aria-hidden="true" />} aria-label="Mes siguiente" onClick={() => shift(1)} />
          </div>
          <Segmented
            aria-label="Vista del calendario"
            value={view}
            onChange={(value) => setView(value as View)}
            options={[{ label: 'Mes', value: 'month' }, { label: 'Lista', value: 'list' }]}
          />
        </div>

        {error && <ErrorState message={error.message} onRetry={retry} />}
        {!error && !events && <Skeleton active paragraph={{ rows: 5 }} />}

        {events && !error && (events.length === 0 ? (
          <div className={styles.panel}>
            <EmptyState title="No hay hitos en este mes" description="Los sprints y las fechas límite aparecerán aquí." />
          </div>
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
              <section key={date} aria-label={date} className={styles.listDay}>
                <h3 className={styles.listDate}>{date}</h3>
                <ul className={styles.dayEvents}>{dayEvents.map(renderEvent)}</ul>
              </section>
            ))}
          </div>
        ))}
      </div>
    </PageContainer>
  )
}
