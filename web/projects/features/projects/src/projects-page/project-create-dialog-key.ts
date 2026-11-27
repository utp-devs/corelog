/** Longest key the API accepts. */
export const PROJECT_KEY_MAX_LENGTH = 20;

const SINGLE_WORD_LENGTH = 4;
const INITIALS_LENGTH = 5;

/**
 * Suggests a project key from its name: the first letters of a single word (Seguridad becomes SEGU) or the
 * initials of several (Plataforma de Pagos becomes PDP). Accents and symbols are dropped.
 */
export function suggestProjectKey(name: string): string {
  const words = name
    .normalize('NFD')
    .replace(/[̀-ͯ]/g, '')
    .toUpperCase()
    .split(/[^A-Z0-9]+/)
    .filter(Boolean);
  if (words.length === 0) return '';
  if (words.length === 1) return words[0].slice(0, SINGLE_WORD_LENGTH);
  return words
    .map((word) => word[0])
    .join('')
    .slice(0, INITIALS_LENGTH);
}
