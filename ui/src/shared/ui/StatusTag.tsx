import type { TicketStatus } from '@/core/tickets/domain/ticket'

import { ToneTag, type Tone } from './ToneTag'

const statusMap: Record<TicketStatus, { label: string; tone: Tone }> = {
  open: { label: 'Abierto', tone: 'blue' },
  in_progress: { label: 'En progreso', tone: 'gold' },
  resolved: { label: 'Resuelto', tone: 'green' },
  closed: { label: 'Cerrado', tone: 'neutral' },
}

type StatusTagProps = {
  status: TicketStatus
}

/** Estado de una incidencia con su color semántico. */
export function StatusTag({ status }: StatusTagProps) {
  const { label, tone } = statusMap[status]
  return <ToneTag label={label} tone={tone} />
}
