import { InvitationItemInput, OrgRole } from '@corelog/core';

import { EMAIL_PATTERN } from '../invitations/invite-form';

export interface InviteRow {
  readonly id: number;
  readonly email: string;
  readonly role: OrgRole;
}

export type InviteRowErrors = Readonly<Record<number, string>>;

/**
 * Validates the invitation rows. Empty rows are ignored. Returns the valid items in order and the
 * message for each invalid row (by row id). Duplicated emails are reported on the repeated row.
 */
export function validateInviteRows(rows: readonly InviteRow[]): {
  items: InvitationItemInput[];
  errors: InviteRowErrors;
} {
  const items: InvitationItemInput[] = [];
  const errors: Record<number, string> = {};
  const seen = new Set<string>();
  for (const row of rows) {
    const email = row.email.trim();
    if (email === '') continue;
    if (!EMAIL_PATTERN.test(email) || email.length > 255) {
      errors[row.id] = 'Introduce un correo electrónico válido';
      continue;
    }
    const key = email.toLowerCase();
    if (seen.has(key)) {
      errors[row.id] = 'Este correo ya está en la lista';
      continue;
    }
    seen.add(key);
    items.push({ email, role: row.role });
  }
  return { items, errors };
}
