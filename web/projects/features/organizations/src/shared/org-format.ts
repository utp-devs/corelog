import { OrgRole } from '@corelog/core';
import type { LozengeTone } from '@corelog/ui/lozenge';

/** Tone of the role lozenge. */
export const ORG_ROLE_TONES: Readonly<Record<OrgRole, LozengeTone>> = {
  owner: 'info',
  admin: 'success',
  member: 'neutral',
  guest: 'warning',
};

const dateFormatter = new Intl.DateTimeFormat('es', {
  day: '2-digit',
  month: 'short',
  year: 'numeric',
});

/** Date-only label in Spanish; missing or invalid input shows a dash instead of "Invalid Date". */
export function formatOrgDate(iso: string | null | undefined): string {
  if (!iso) return '—';
  const date = new Date(iso);
  return Number.isNaN(date.getTime()) ? '—' : dateFormatter.format(date);
}
