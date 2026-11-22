import type { TicketPriority, TicketStatus } from '@/core/tickets/domain/ticket'
import styles from './TicketBadge.module.css'

type Tone = 'neutral' | 'blue' | 'gold' | 'green' | 'orange' | 'red'

const statusMap: Record<TicketStatus, { label: string; tone: Tone }> = {
  open: { label: 'Abierto', tone: 'blue' },
  in_progress: { label: 'En progreso', tone: 'gold' },
  resolved: { label: 'Resuelto', tone: 'green' },
  closed: { label: 'Cerrado', tone: 'neutral' },
}

const priorityMap: Record<TicketPriority, { label: string; tone: Tone }> = {
  low: { label: 'Baja', tone: 'neutral' },
  medium: { label: 'Media', tone: 'blue' },
  high: { label: 'Alta', tone: 'orange' },
  critical: { label: 'Critica', tone: 'red' },
}

type TicketBadgeProps = { kind: 'status'; value: TicketStatus } | { kind: 'priority'; value: TicketPriority }

export function TicketBadge(props: TicketBadgeProps) {
  const { label, tone } = props.kind === 'status' ? statusMap[props.value] : priorityMap[props.value]

  return <span className={`${styles.badge} ${styles[tone]}`}>{label}</span>
}
