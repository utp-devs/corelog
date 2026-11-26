import type { IssuePriorityKind } from '@/shared/types/issue'

import { ToneTag, type Tone } from './ToneTag'

const priorityMap: Record<IssuePriorityKind, { label: string; tone: Tone }> = {
  low: { label: 'Baja', tone: 'neutral' },
  medium: { label: 'Media', tone: 'blue' },
  high: { label: 'Alta', tone: 'orange' },
  critical: { label: 'Crítica', tone: 'red' },
}

type PriorityTagProps = {
  priority: IssuePriorityKind
}

/** Prioridad de una incidencia con su color semántico. */
export function PriorityTag({ priority }: PriorityTagProps) {
  const { label, tone } = priorityMap[priority]
  return <ToneTag label={label} tone={tone} />
}
