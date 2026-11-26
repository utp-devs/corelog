import { Button, Typography } from 'antd'
import { useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'

import { useCurrentOrg } from '@/adapters/in/state/org.store'
import { InviteStep } from '@/adapters/in/ui/organizations/onboarding/InviteStep'
import { ChooseStep } from '@/adapters/in/ui/organizations/onboarding/ChooseStep'
import { OrgFormStep } from '@/adapters/in/ui/organizations/onboarding/OrgFormStep'
import { PageContainer } from '@/shared/ui/PageContainer'
import styles from './OnboardingPage.module.css'

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
    <main className={styles.page}>
      <PageContainer variant="reading" pageName="onboarding" ready fill>
        <div className={styles.wizard}>
          <header className={styles.header}>
            <img src="/brand/corelog-banner-480.png" alt="CoreLog" width={128} height={32} className={styles.logo} />
          </header>

          <nav aria-label="Pasos del asistente" className={styles.stepsNav}>
            <ol className={styles.steps}>
              {STEP_TITLES.map((title, index) => {
                const state = index < step ? 'done' : index === step ? 'current' : 'upcoming'
                return (
                  <li
                    key={title}
                    className={styles.step}
                    data-state={state}
                    aria-current={index === step ? 'step' : undefined}
                  >
                    <span className={styles.stepNumber} aria-hidden="true">
                      {index + 1}
                    </span>
                    <span className={styles.stepTitle}>
                      {title}
                      {state === 'done' && <span className={styles.srOnly}> (completado)</span>}
                    </span>
                  </li>
                )
              })}
            </ol>
          </nav>

          <div className={styles.stepBody}>
          {step === 0 && <ChooseStep onCreate={() => setStep(1)} />}

          {step === 1 && (
            <section className={styles.panel} aria-labelledby="onboarding-org-title">
              <Typography.Title level={2} id="onboarding-org-title" className={styles.panelTitle}>
                Datos de la organización
              </Typography.Title>
              <OrgFormStep onBack={() => setStep(0)} onCreated={() => setStep(2)} />
            </section>
          )}

          {step === 2 && (
            <section className={styles.panel} aria-labelledby="onboarding-invite-title">
              <Typography.Title level={2} id="onboarding-invite-title" className={styles.panelTitle}>
                Invita a tu equipo
              </Typography.Title>
              {org ? (
                <InviteStep orgId={org.id} onContinue={() => setStep(3)} />
              ) : (
                <Typography.Paragraph>No hay una organización activa. Vuelve al paso anterior.</Typography.Paragraph>
              )}
            </section>
          )}

          {step === 3 && (
            <section className={styles.panel} aria-labelledby="onboarding-finish-title">
              <Typography.Title level={2} id="onboarding-finish-title" className={styles.panelTitle}>
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
          </div>
        </div>
      </PageContainer>
    </main>
  )
}
