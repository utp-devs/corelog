import { AbstractControl, ValidationErrors, ValidatorFn } from '@angular/forms';
import { InvitationItemInput, OrgRole } from '@corelog/core';

/** Maximum invitations per request, as enforced by the API (invitations maxItems). */
export const MAX_INVITATIONS_PER_REQUEST = 50;

export const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

export interface ParsedEmails {
  /** Valid, unique emails in the order typed (case-insensitive duplicates removed). */
  readonly emails: string[];
  /** Entries that are not valid email addresses. */
  readonly invalid: string[];
  /** Repeated entries (kept once in `emails`). */
  readonly duplicates: string[];
}

/** Splits a free text of emails by commas, semicolons, spaces or new lines and classifies each entry. */
export function parseInviteEmails(text: string): ParsedEmails {
  const emails: string[] = [];
  const invalid: string[] = [];
  const duplicates: string[] = [];
  const seen = new Set<string>();
  for (const raw of text.split(/[\s,;]+/)) {
    const candidate = raw.trim();
    if (candidate === '') continue;
    if (!EMAIL_PATTERN.test(candidate) || candidate.length > 255) {
      invalid.push(candidate);
      continue;
    }
    const key = candidate.toLowerCase();
    if (seen.has(key)) {
      duplicates.push(candidate);
      continue;
    }
    seen.add(key);
    emails.push(candidate);
  }
  return { emails, invalid, duplicates };
}

/** Roles that an invitation may grant. Owners are granted by transfer, not by invitation. */
export type InvitableRole = Exclude<OrgRole, 'owner'>;

/**
 * Rules for the emails field, checked in order: at least one email, no invalid entry, at most the API
 * limit. Each failure carries its Spanish message under the `message` key.
 */
export const inviteEmailsValidator: ValidatorFn = (
  control: AbstractControl,
): ValidationErrors | null => {
  const parsed = parseInviteEmails(String(control.value ?? ''));
  if (parsed.emails.length === 0) {
    return { message: 'Introduce al menos un correo electrónico' };
  }
  if (parsed.invalid.length > 0) {
    return { message: 'Revisa los correos: alguno no tiene un formato válido' };
  }
  if (parsed.emails.length > MAX_INVITATIONS_PER_REQUEST) {
    return {
      message: `Puedes invitar a ${MAX_INVITATIONS_PER_REQUEST} personas como máximo por envío`,
    };
  }
  return null;
};

export interface InviteFormValues {
  readonly emails: string;
  readonly role: InvitableRole;
}

/** Items for the API call: one per valid email with the chosen role. */
export function inviteItemsOf(values: InviteFormValues): InvitationItemInput[] {
  return parseInviteEmails(values.emails).emails.map((email) => ({ email, role: values.role }));
}
