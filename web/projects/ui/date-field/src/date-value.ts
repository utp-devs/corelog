const ISO_DATE = /^(\d{4})-(\d{2})-(\d{2})/;

/** Local date for an ISO `yyyy-MM-dd` string (a time part is ignored); null when empty or invalid. */
export function parseIsoDate(value: string | null | undefined): Date | null {
  const match = ISO_DATE.exec(value ?? '');
  if (!match) {
    return null;
  }
  const [year, month, day] = [Number(match[1]), Number(match[2]) - 1, Number(match[3])];
  const date = new Date(year, month, day);
  const valid = date.getFullYear() === year && date.getMonth() === month && date.getDate() === day;
  return valid ? date : null;
}

/** ISO `yyyy-MM-dd` string of the local calendar day of a date. */
export function toIsoDate(date: Date): string {
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  return `${String(date.getFullYear()).padStart(4, '0')}-${month}-${day}`;
}
