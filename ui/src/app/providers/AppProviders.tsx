import { App as AntApp, ConfigProvider, theme } from 'antd'
import type { PropsWithChildren } from 'react'
import { useEffect, useState } from 'react'

import { getStoredTheme, setStoredTheme, type ThemeMode } from '@/adapters/out/storage/theme-storage'
import { coreLogTheme } from '@/shared/theme/tokens'
import { ThemeContext } from '@/app/theme-context'

export function AppProviders({ children }: PropsWithChildren) {
  const [themeMode, setThemeMode] = useState<ThemeMode>(() => getStoredTheme())

  useEffect(() => {
    setStoredTheme(themeMode)
  }, [themeMode])

  const isDark = themeMode === 'dark'
  const toggleTheme = () => setThemeMode((prev) => (prev === 'dark' ? 'light' : 'dark'))

  return (
    <ConfigProvider
      theme={{
        algorithm: isDark ? theme.darkAlgorithm : theme.defaultAlgorithm,
        token: {
          colorPrimary: isDark ? coreLogTheme.brandDark : coreLogTheme.brand,
          colorInfo: isDark ? coreLogTheme.infoDark : coreLogTheme.info,
          colorSuccess: isDark ? coreLogTheme.successDark : coreLogTheme.success,
          colorWarning: isDark ? coreLogTheme.warningDark : coreLogTheme.warning,
          colorError: isDark ? coreLogTheme.dangerDark : coreLogTheme.danger,
          colorBgLayout: isDark ? coreLogTheme.bgPageDark : coreLogTheme.bgPageLight,
          colorBgContainer: isDark ? coreLogTheme.bgSurfaceDark : coreLogTheme.bgSurfaceLight,
          colorText: isDark ? coreLogTheme.textDark : coreLogTheme.textLight,
          colorTextDescription: isDark ? coreLogTheme.textSubtleDark : coreLogTheme.textSubtleLight,
          colorBorder: isDark ? coreLogTheme.borderDark : coreLogTheme.borderLight,
          borderRadius: 6,
          fontSize: 14,
          fontFamily: coreLogTheme.fontFamily,
          boxShadow: coreLogTheme.shadow1,
        },
        components: {
          Layout: {
            headerBg: 'transparent',
            siderBg: isDark ? coreLogTheme.bgSurfaceDark : coreLogTheme.bgSurfaceLight,
            bodyBg: isDark ? coreLogTheme.bgPageDark : coreLogTheme.bgPageLight,
            triggerBg: isDark ? coreLogTheme.bgSunkenDark : coreLogTheme.bgSunkenLight,
            triggerColor: isDark ? coreLogTheme.textSubtleDark : coreLogTheme.textSubtleLight,
          },
          Table: {
            headerBg: isDark ? coreLogTheme.bgSunkenDark : coreLogTheme.bgSunkenLight,
          },
          Menu: {
            itemBorderRadius: 6,
            itemHeight: 40,
            itemMarginInline: 8,
            itemSelectedBg: isDark ? coreLogTheme.brandSubtleDark : coreLogTheme.brandSubtleLight,
            itemSelectedColor: isDark ? coreLogTheme.brandDark : coreLogTheme.brand,
            groupTitleColor: isDark ? coreLogTheme.textSubtleDark : coreLogTheme.textSubtleLight,
            groupTitleFontSize: 11,
          },
          Card: { boxShadow: 'none' },
          Button: { primaryShadow: 'none', defaultShadow: 'none' },
        },
      }}
    >
      <ThemeContext.Provider value={{ themeMode, setThemeMode, toggleTheme }}>
        <AntApp>{children}</AntApp>
      </ThemeContext.Provider>
    </ConfigProvider>
  )
}
