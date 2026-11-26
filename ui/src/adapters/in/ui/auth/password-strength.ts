/** 0 = empty, 1 = weak, 2 = fair, 3 = good, 4 = strong. */
export type PasswordLevel = 0 | 1 | 2 | 3 | 4

const LEVEL_LABELS: Record<PasswordLevel, string> = {
  0: '',
  1: 'Débil',
  2: 'Aceptable',
  3: 'Buena',
  4: 'Fuerte',
}

/** Scores a password from length, letter case, digits and symbols. Passwords under 8 characters are never above weak. */
export function scorePassword(password: string): PasswordLevel {
  if (password.length === 0) return 0
  if (password.length < 8) return 1
  let points = 1
  if (password.length >= 12) points += 1
  if (/[a-z]/.test(password) && /[A-Z]/.test(password)) points += 1
  if (/\d/.test(password) || /[^A-Za-z0-9]/.test(password)) points += 1
  return Math.min(4, points) as PasswordLevel
}

export function passwordLevelLabel(level: PasswordLevel): string {
  return LEVEL_LABELS[level]
}
