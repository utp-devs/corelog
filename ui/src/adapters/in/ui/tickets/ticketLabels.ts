import type { LozengeCategory } from '@/shared/ui/Lozenge'
import type { IssueType, TicketPriority, TicketStatus } from '@/core/tickets/domain/ticket'

export const statusMeta: Record<TicketStatus, { label: string; category: LozengeCategory }> = {
  open: { label: 'Abierto', category: 'todo' },
  in_progress: { label: 'En progreso', category: 'progress' },
  resolved: { label: 'Resuelto', category: 'done' },
  closed: { label: 'Cerrado', category: 'done' },
}

export const priorityLabels: Record<TicketPriority, string> = {
  low: 'Baja',
  medium: 'Media',
  high: 'Alta',
  critical: 'Crítica',
}

export const issueTypeLabels: Record<IssueType, string> = {
  task: 'Tarea',
  bug: 'Error',
  story: 'Historia',
  epic: 'Épica',
  subtask: 'Subtarea',
}

/** Clave visible de la incidencia: el modelo no tiene clave propia, se deriva del identificador. */
export function issueKeyOf(id: string): string {
  return `INC-${id.slice(0, 8).toUpperCase()}`
}
