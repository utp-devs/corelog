import { createContext, useContext } from 'react'

import type { ThemeMode } from '@/adapters/out/storage/theme-storage'

export type ThemeContextValue = {
  themeMode: ThemeMode
  setThemeMode: (mode: ThemeMode) => void
  toggleTheme: () => void
}

export const ThemeContext = createContext<ThemeContextValue | null>(null)

export function useThemeMode(): ThemeContextValue {
  const context = useContext(ThemeContext)
  if (!context) throw new Error('useThemeMode must be used inside AppProviders')
  return context
}
