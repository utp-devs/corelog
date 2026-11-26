import type { OrgRole } from '@/core/organizations/domain/organization'
import type { InvitationItemInput } from '@/core/organizations/application/ports/organization-repository.port'
import { z } from 'zod'

export interface InviteRow {
  id: number
  email: string
  role: OrgRole
}

export type InviteRowErrors = Record<number, string>

const emailSchema = z.string().trim().email('Introduce un correo electrónico válido').max(255)

/**
 * Validates the invitation rows. Empty rows are ignored. Returns the valid items in order and the
 * message for each invalid row (by row id). Duplicated emails are reported on the repeated row.
 */
export function validateInviteRows(rows: InviteRow[]): { items: InvitationItemInput[]; errors: InviteRowErrors } {
  const items: InvitationItemInput[] = []
  const errors: InviteRowErrors = {}
  const seen = new Set<string>()
  for (const row of rows) {
    const email = row.email.trim()
    if (email === '') continue
    const parsed = emailSchema.safeParse(email)
    if (!parsed.success) {
      errors[row.id] = 'Introduce un correo electrónico válido'
      continue
    }
    const key = email.toLowerCase()
    if (seen.has(key)) {
      errors[row.id] = 'Este correo ya está en la lista'
      continue
    }
    seen.add(key)
    items.push({ email, role: row.role })
  }
  return { items, errors }
}
