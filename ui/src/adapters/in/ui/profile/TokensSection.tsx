import { Card, Typography } from 'antd'

import { EmptyState } from '@/shared/ui/EmptyState'
import styles from './ProfilePage.module.css'

/** Placeholder until API tokens are implemented (S-63). */
export function TokensSection() {
  return (
    <section id="perfil-tokens" aria-labelledby="perfil-tokens-title" className={styles.section}>
      <Typography.Title level={4} id="perfil-tokens-title" className={styles.sectionTitle}>Tokens</Typography.Title>
      <Card className={styles.card}>
        <EmptyState title="Próximamente" description="Los tokens de acceso para integraciones estarán disponibles en una próxima versión." />
      </Card>
    </section>
  )
}
