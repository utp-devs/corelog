import type { LozengeTone } from '@/shared/ui/Lozenge'
import type { OrgRole } from '@/core/organizations/domain/organization'

export const ORG_ROLE_LABELS: Record<OrgRole, string> = {
  owner: 'Propietario',
  admin: 'Administrador',
  member: 'Miembro',
  guest: 'Invitado',
}

export const ORG_ROLE_TONES: Record<OrgRole, LozengeTone> = {
  owner: 'info',
  admin: 'success',
  member: 'neutral',
  guest: 'warning',
}

const dateFormatter = new Intl.DateTimeFormat('es', { day: '2-digit', month: 'short', year: 'numeric' })

/** Date-only label in Spanish; invalid input shows a dash instead of "Invalid Date". */
export function formatOrgDate(iso: string | null | undefined): string {
  if (!iso) return '—'
  const date = new Date(iso)
  return Number.isNaN(date.getTime()) ? '—' : dateFormatter.format(date)
}
