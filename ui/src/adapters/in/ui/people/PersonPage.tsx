import { useCallback, useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { Descriptions, Skeleton, Typography } from 'antd'
import dayjs from 'dayjs'

import { useCurrentOrg } from '@/adapters/in/state/org.store'
import { PeopleHttpRepository } from '@/adapters/out/http/people-http-repository'
import type { PeopleRepositoryPort } from '@/core/people/application/ports/people-repository.port'
import type { User } from '@/core/auth/domain/user'
import type { OrganizationMember } from '@/core/people/domain/member'
import { Breadcrumbs } from '@/shared/ui/Breadcrumbs'
import { ErrorPage } from '@/shared/ui/ErrorPage'
import { ErrorState } from '@/shared/ui/ErrorState'
import { Lozenge } from '@/shared/ui/Lozenge'
import { PageContainer } from '@/shared/ui/PageContainer'
import { Section } from '@/shared/ui/Section'
import { describeDirectoryError } from './errors'
import { ORG_ROLE_LABELS, ORG_ROLE_TONES } from './roles'
import { UserAvatar } from './UserAvatar'
import styles from './PersonPage.module.css'

interface PersonPageProps {
  repository?: PeopleRepositoryPort
}

type LoadState =
  | { status: 'loading' }
  | { status: 'error'; message: string }
  | { status: 'not-found' }
  | { status: 'ready'; member: OrganizationMember; user: User | null }

interface LoadResult {
  key: string
  state: LoadState
}

const defaultRepository = new PeopleHttpRepository()

/** Public card of one member of the active organization (S-27 profile). */
export function PersonPage({ repository = defaultRepository }: PersonPageProps) {
  const { id = '' } = useParams<{ id: string }>()
  const { org } = useCurrentOrg()
  const orgId = org?.id ?? null
  const [attempt, setAttempt] = useState(0)
  const [result, setResult] = useState<LoadResult | null>(null)

  const requestKey = `${orgId ?? ''}|${id}|${attempt}`

  useEffect(() => {
    if (!orgId) return
    const key = `${orgId}|${id}|${attempt}`
    let active = true
    const load = async (): Promise<LoadState> => {
      const members = await repository.listMembers(orgId)
      const member = members.find((item) => item.userId === id)
      if (!member) return { status: 'not-found' }
      // The members list has no job title or photo: the search result (same org) provides them.
      // If the search fails the card still shows the member data.
      const user = await repository
        .searchUsers(orgId, member.email)
        .then((found) => found.find((item) => item.id === id) ?? null)
        .catch(() => null)
      return { status: 'ready', member, user }
    }
    load()
      .catch((failure: unknown): LoadState => ({ status: 'error', message: describeDirectoryError(failure) }))
      .then((state) => {
        if (active) setResult({ key, state })
      })
    return () => {
      active = false
    }
  }, [id, orgId, attempt, repository])

  const retry = useCallback(() => setAttempt((value) => value + 1), [])

  if (!orgId) {
    return (
      <ErrorPage
        variant={404}
        title="Persona no encontrada"
        description="Esta persona no forma parte de la organización activa o ya no existe."
      />
    )
  }

  const state: LoadState = result !== null && result.key === requestKey ? result.state : { status: 'loading' }

  if (state.status === 'not-found') {
    return (
      <ErrorPage
        variant={404}
        title="Persona no encontrada"
        description="Esta persona no forma parte de la organización activa o ya no existe."
      />
    )
  }

  if (state.status === 'loading') {
    return (
      <PageContainer variant="reading" pageName="person" ready={false}>
        <Section ariaLabel="Cargando persona">
          <Skeleton active avatar paragraph={{ rows: 4 }} />
        </Section>
      </PageContainer>
    )
  }

  if (state.status === 'error') {
    return (
      <PageContainer variant="reading" pageName="person" ready>
        <ErrorState message={state.message} onRetry={retry} />
      </PageContainer>
    )
  }

  const { member, user } = state
  const displayName = user?.displayName || member.name
  const joined = dayjs(member.joinedAt).format('DD/MM/YYYY')

  return (
    <PageContainer variant="reading" pageName="person" ready>
      <Breadcrumbs items={[{ label: 'Personas', to: '/people' }, { label: displayName }]} />
      <Section ariaLabel={`Ficha de ${displayName}`}>
        <div className={styles.header}>
          <UserAvatar id={member.userId} name={displayName} avatarKey={user?.avatarKey} size={64} data-testid="person-avatar" />
          <div className={styles.identity}>
            <Typography.Title level={3} className={styles.name}>{displayName}</Typography.Title>
            <Lozenge tone={ORG_ROLE_TONES[member.role]}>{ORG_ROLE_LABELS[member.role]}</Lozenge>
          </div>
        </div>
        <Descriptions
          column={1}
          size="small"
          items={[
            { key: 'title', label: 'Cargo', children: user?.jobTitle || '—' },
            { key: 'department', label: 'Departamento', children: user?.department || '—' },
            { key: 'email', label: 'Correo', children: member.email },
            { key: 'timezone', label: 'Zona horaria', children: user?.timezone || '—' },
            { key: 'joined', label: 'Se unió', children: joined },
          ]}
        />
        <Link to="/people" className={styles.back}>Volver al directorio</Link>
      </Section>
    </PageContainer>
  )
}
