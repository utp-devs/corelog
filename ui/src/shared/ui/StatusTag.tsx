import type { IssueStatusKind } from '@/shared/types/issue'

import { ToneTag, type Tone } from './ToneTag'

const statusMap: Record<IssueStatusKind, { label: string; tone: Tone }> = {
  open: { label: 'Abierto', tone: 'blue' },
  in_progress: { label: 'En progreso', tone: 'gold' },
  resolved: { label: 'Resuelto', tone: 'green' },
  closed: { label: 'Cerrado', tone: 'neutral' },
}

type StatusTagProps = {
  status: IssueStatusKind
}

/** Estado de una incidencia con su color semántico. */
export function StatusTag({ status }: StatusTagProps) {
  const { label, tone } = statusMap[status]
  return <ToneTag label={label} tone={tone} />
}
