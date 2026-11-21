import { MoonOutlined, SunOutlined } from '@ant-design/icons'
import { App as AntApp, Button, ConfigProvider, theme } from 'antd'
import type { PropsWithChildren } from 'react'
import { useState } from 'react'

export function AppProviders({ children }: PropsWithChildren) {
  const [isDark, setIsDark] = useState(true)

  return (
    <ConfigProvider
      theme={{
        algorithm: isDark ? theme.darkAlgorithm : theme.defaultAlgorithm,
        token: {
          colorPrimary: '#aa3bff',
          borderRadius: 6,
        },
      }}
    >
      <AntApp>
        {children}
        <Button
          type="default"
          shape="circle"
          size="large"
          icon={isDark ? <SunOutlined /> : <MoonOutlined />}
          onClick={() => setIsDark((prev) => !prev)}
          aria-label="Toggle theme"
          style={{
            position: 'fixed',
            bottom: 24,
            left: 24,
            zIndex: 1000,
            boxShadow: '0 2px 8px rgba(0, 0, 0, 0.15)',
          }}
        />
      </AntApp>
    </ConfigProvider>
  )
}
