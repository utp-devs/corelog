import { z } from 'zod'

import type { InvitationItemInput } from '@/core/organizations/application/ports/organization-repository.port'
import type { OrgRole } from '@/core/organizations/domain/organization'

/** Maximum invitations per request, as enforced by the API (invitations maxItems). */
export const MAX_INVITATIONS_PER_REQUEST = 50

const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]+$/

export interface ParsedEmails {
  /** Valid, unique emails in the order typed (case-insensitive duplicates removed). */
  emails: string[]
  /** Entries that are not valid email addresses. */
  invalid: string[]
  /** Repeated entries (kept once in `emails`). */
  duplicates: string[]
}

/** Splits a free text of emails by commas, semicolons, spaces or new lines and classifies each entry. */
export function parseInviteEmails(text: string): ParsedEmails {
  const emails: string[] = []
  const invalid: string[] = []
  const duplicates: string[] = []
  const seen = new Set<string>()
  for (const raw of text.split(/[\s,;]+/)) {
    const candidate = raw.trim()
    if (candidate === '') continue
    if (!EMAIL_PATTERN.test(candidate) || candidate.length > 255) {
      invalid.push(candidate)
      continue
    }
    const key = candidate.toLowerCase()
    if (seen.has(key)) {
      duplicates.push(candidate)
      continue
    }
    seen.add(key)
    emails.push(candidate)
  }
  return { emails, invalid, duplicates }
}

export const inviteFormSchema = z.object({
  emails: z
    .string()
    .refine((text) => parseInviteEmails(text).emails.length > 0, 'Introduce al menos un correo electrónico')
    .refine((text) => parseInviteEmails(text).invalid.length === 0, 'Revisa los correos: alguno no tiene un formato válido')
    .refine(
      (text) => parseInviteEmails(text).emails.length <= MAX_INVITATIONS_PER_REQUEST,
      `Puedes invitar a ${MAX_INVITATIONS_PER_REQUEST} personas como máximo por envío`,
    ),
  role: z.enum(['admin', 'member', 'guest']),
})

export type InviteFormValues = { emails: string; role: Exclude<OrgRole, 'owner'> }

/** Items for the API call: one per valid email with the chosen role. */
export function inviteItemsOf(values: InviteFormValues): InvitationItemInput[] {
  return parseInviteEmails(values.emails).emails.map((email) => ({ email, role: values.role }))
}
