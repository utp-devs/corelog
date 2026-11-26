import { GlobalOutlined } from '@ant-design/icons'
import { Card, Segmented, Select, Space, Tag, Typography } from 'antd'

import type { ThemeMode } from '@/adapters/out/storage/theme-storage'
import { useThemeMode } from '@/app/theme-context'
import styles from './ProfilePage.module.css'

const themeOptions: { value: ThemeMode; label: string }[] = [
  { value: 'light', label: 'Claro' },
  { value: 'dark', label: 'Oscuro' },
]

/** Theme (saved in this browser) and language (Spanish only for now). */
export function AppearanceSection() {
  const { themeMode, setThemeMode } = useThemeMode()

  return (
    <section id="perfil-apariencia" aria-labelledby="perfil-apariencia-title" className={styles.section}>
      <Typography.Title level={4} id="perfil-apariencia-title" className={styles.sectionTitle}>Apariencia</Typography.Title>
      <Card title="Tema" className={styles.card}>
        <Typography.Paragraph type="secondary" className={styles.hint}>
          El tema se guarda en este navegador y se aplica al abrir la aplicación.
        </Typography.Paragraph>
        <Segmented
          aria-label="Tema de la aplicación"
          options={themeOptions}
          value={themeMode}
          onChange={(value) => setThemeMode(value as ThemeMode)}
        />
      </Card>
      <Card className={styles.card}>
        <div className={styles.rowBetween}>
          <div>
            <Typography.Text strong><GlobalOutlined /> Idioma</Typography.Text>
            <Typography.Paragraph type="secondary" className={styles.rowHint}>
              La interfaz está disponible en español.
            </Typography.Paragraph>
          </div>
          <Space>
            <Tag color="default">Próximamente</Tag>
            <Select
              aria-label="Idioma"
              disabled
              value="es"
              options={[{ value: 'es', label: 'Español' }]}
              className={styles.languageSelect}
            />
          </Space>
        </div>
      </Card>
    </section>
  )
}
