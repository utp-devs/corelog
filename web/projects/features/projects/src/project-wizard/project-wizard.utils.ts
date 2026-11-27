/** Keys are 2 to 10 characters: an upper-case letter followed by upper-case letters or digits. */
export const PROJECT_KEY_PATTERN = /^[A-Z][A-Z0-9]{1,9}$/;
export const PROJECT_KEY_MAX_LENGTH = 10;

const KEY_FALLBACK_LENGTH = 4;

function words(name: string): string[] {
  return name
    .normalize('NFD')
    .replace(/[̀-ͯ]/g, '')
    .toUpperCase()
    .split(/[^A-Z0-9]+/)
    .filter(Boolean);
}

/**
 * Suggests a key from a project name: the initials of several words (Plataforma de Pagos becomes
 * PDP) or the first letters of a single one (Seguridad becomes SEGU). Always starts with a letter.
 */
export function suggestKey(name: string): string {
  const parts = words(name);
  if (parts.length === 0) return '';
  const raw =
    parts.length === 1
      ? parts[0].slice(0, KEY_FALLBACK_LENGTH)
      : parts
          .map((part) => part[0])
          .join('')
          .slice(0, PROJECT_KEY_MAX_LENGTH);
  return raw.replace(/^[0-9]+/, '');
}

export function normalizeKey(value: string): string {
  return value.trim().toUpperCase();
}

export function isValidKey(key: string): boolean {
  return PROJECT_KEY_PATTERN.test(key);
}

export type KeyStatus = 'idle' | 'checking' | 'available' | 'taken' | 'invalid' | 'unknown';

/** CSS color of a stored project color: a palette name maps to its token, a hex value is used as is. */
export function projectColorCss(value: string | null | undefined): string | null {
  if (!value) return null;
  const match = /^palette-(\d+)$/.exec(value);
  if (match) return `var(--cl-palette-${match[1]}-bg)`;
  return /^#[0-9a-fA-F]{6}$/.test(value) ? value : null;
}
