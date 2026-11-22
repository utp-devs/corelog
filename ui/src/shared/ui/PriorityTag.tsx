import type { TicketPriority } from '@/core/tickets/domain/ticket'

import { ToneTag, type Tone } from './ToneTag'

const priorityMap: Record<TicketPriority, { label: string; tone: Tone }> = {
  low: { label: 'Baja', tone: 'neutral' },
  medium: { label: 'Media', tone: 'blue' },
  high: { label: 'Alta', tone: 'orange' },
  critical: { label: 'Crítica', tone: 'red' },
}

type PriorityTagProps = {
  priority: TicketPriority
}

/** Prioridad de una incidencia con su color semántico. */
export function PriorityTag({ priority }: PriorityTagProps) {
  const { label, tone } = priorityMap[priority]
  return <ToneTag label={label} tone={tone} />
}
