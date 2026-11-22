const THEME_KEY = 'corelog.theme'

export type ThemeMode = 'light' | 'dark'

export function getStoredTheme(): ThemeMode {
  if (typeof window === 'undefined' || !window.localStorage) {
    return 'light'
  }
  const stored = localStorage.getItem(THEME_KEY)
  if (stored === 'light' || stored === 'dark') {
    return stored
  }
  return 'light'
}

export function setStoredTheme(theme: ThemeMode): void {
  if (typeof window === 'undefined' || !window.localStorage) {
    return
  }
  localStorage.setItem(THEME_KEY, theme)
  document.documentElement.setAttribute('data-theme', theme)
  document.documentElement.style.colorScheme = theme
}
