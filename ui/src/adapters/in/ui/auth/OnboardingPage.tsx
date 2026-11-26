import { Button, Steps, Typography } from 'antd'
import { useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'

import { useCurrentOrg } from '@/adapters/in/state/org.store'
import { InviteStep } from '@/adapters/in/ui/organizations/onboarding/InviteStep'
import { ChooseStep } from '@/adapters/in/ui/organizations/onboarding/ChooseStep'
import { OrgFormStep } from '@/adapters/in/ui/organizations/onboarding/OrgFormStep'
import styles from '@/adapters/in/ui/organizations/onboarding/onboarding.module.css'
import { PageContainer } from '@/shared/ui/PageContainer'

const STEP_TITLES = ['Elegir', 'Organización', 'Equipo', 'Finalizar'] as const

/**
 * Onboarding wizard (S-05), authenticated but outside the app shell. Step 1 chooses between creating
 * an organization and joining one; ?new=1 starts at step 2 (used by the org switcher).
 */
export function OnboardingPage() {
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const { org } = useCurrentOrg()
  const [step, setStep] = useState<number>(() => (searchParams.get('new') ? 1 : 0))

  return (
    <PageContainer variant="reading" pageName="onboarding" ready>
      <header className={styles.header}>
        <img src="/brand/corelog-banner-480.png" alt="CoreLog" width={128} className={styles.logo} />
      </header>

      <Steps
        className={styles.steps}
        current={step}
        items={STEP_TITLES.map((title) => ({ title }))}
      />

      {step === 0 && <ChooseStep onCreate={() => setStep(1)} />}

      {step === 1 && (
        <>
          <Typography.Title level={2}>Datos de la organización</Typography.Title>
          <OrgFormStep onBack={() => setStep(0)} onCreated={() => setStep(2)} />
        </>
      )}

      {step === 2 && (
        <>
          <Typography.Title level={2}>Invita a tu equipo</Typography.Title>
          {org ? (
            <InviteStep orgId={org.id} onContinue={() => setStep(3)} />
          ) : (
            <Typography.Paragraph>No hay una organización activa. Vuelve al paso anterior.</Typography.Paragraph>
          )}
        </>
      )}

      {step === 3 && (
        <section className={styles.summary} aria-labelledby="onboarding-finish-title">
          <Typography.Title level={2} id="onboarding-finish-title">
            Todo listo
          </Typography.Title>
          <Typography.Paragraph>
            {org ? `${org.name} está preparada.` : 'Tu cuenta está preparada.'} Empieza creando tu primer proyecto.
          </Typography.Paragraph>
          <Button type="primary" size="large" onClick={() => navigate('/projects', { replace: true })}>
            Ir a proyectos
          </Button>
        </section>
      )}
    </PageContainer>
  )
}
