import { useCallback, useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { Descriptions, Skeleton, Typography } from 'antd'
import dayjs from 'dayjs'

import { useCurrentOrg } from '@/adapters/in/state/org.store'
import { PeopleHttpRepository } from '@/adapters/out/http/people-http-repository'
import { TeamHttpRepository } from '@/adapters/out/http/team-http-repository'
import { extractErrorMessage } from '@/adapters/out/http/api-client'
import type { PeopleRepositoryPort } from '@/core/people/application/ports/people-repository.port'
import type { PersonProfile } from '@/core/people/domain/member'
import type { TeamRepositoryPort } from '@/core/teams/application/ports/team-repository.port'
import type { Team } from '@/core/teams/domain/team'
import { httpStatusOf } from '@/core/shared/app-error'
import { Breadcrumbs } from '@/shared/ui/Breadcrumbs'
import { ErrorPage } from '@/shared/ui/ErrorPage'
import { ErrorState } from '@/shared/ui/ErrorState'
import { Lozenge } from '@/shared/ui/Lozenge'
import { PageContainer } from '@/shared/ui/PageContainer'
import { Section } from '@/shared/ui/Section'
import { TeamChip } from '@/shared/ui/TeamChip'
import { ORG_ROLE_LABELS, ORG_ROLE_TONES } from './roles'
import { UserAvatar } from './UserAvatar'
import styles from './PersonPage.module.css'

interface PersonPageProps {
  repository?: PeopleRepositoryPort
  teamRepository?: Pick<TeamRepositoryPort, 'listForMember'>
}

type LoadState =
  | { status: 'loading' }
  | { status: 'forbidden' }
  | { status: 'not-found' }
  | { status: 'error'; message: string }
  /** teams is null when the team list could not be loaded; the profile still shows. */
  | { status: 'ready'; person: PersonProfile; teams: Team[] | null }

interface LoadResult {
  key: string
  state: LoadState
}

const defaultRepository = new PeopleHttpRepository()
const defaultTeamRepository = new TeamHttpRepository()

/** Public card of one member of the active organization (S-27 profile). */
export function PersonPage({ repository = defaultRepository, teamRepository = defaultTeamRepository }: PersonPageProps) {
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
      const person = await repository.getPerson(orgId, id)
      // The team list is secondary: when it fails the profile still renders.
      const teams = await teamRepository.listForMember(orgId, id).catch(() => null)
      return { status: 'ready', person, teams }
    }
    load()
      .catch((failure: unknown): LoadState => {
        const status = httpStatusOf(failure)
        if (status === 403) return { status: 'forbidden' }
        if (status === 404) return { status: 'not-found' }
        return { status: 'error', message: extractErrorMessage(failure, 'No se pudo cargar la persona.') }
      })
      .then((state) => {
        if (active) setResult({ key, state })
      })
    return () => {
      active = false
    }
  }, [id, orgId, attempt, repository, teamRepository])

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

  if (state.status === 'forbidden') {
    return <ErrorPage variant={403} />
  }

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

  const { person, teams } = state
  const displayName = person.displayName || person.name
  const joined = dayjs(person.joinedAt).format('DD/MM/YYYY')

  return (
    <PageContainer variant="reading" pageName="person" ready>
      <Breadcrumbs items={[{ label: 'Personas', to: '/people' }, { label: displayName }]} />
      <Section ariaLabel={`Ficha de ${displayName}`}>
        <div className={styles.header}>
          <UserAvatar id={person.userId} name={displayName} hasAvatar={person.hasAvatar} size={64} data-testid="person-avatar" />
          <div className={styles.identity}>
            <Typography.Title level={3} className={styles.name}>{displayName}</Typography.Title>
            <Lozenge tone={ORG_ROLE_TONES[person.orgRole]}>{ORG_ROLE_LABELS[person.orgRole]}</Lozenge>
          </div>
        </div>
        <Descriptions
          column={1}
          size="small"
          items={[
            { key: 'title', label: 'Cargo', children: person.jobTitle || '—' },
            { key: 'department', label: 'Departamento', children: person.department || '—' },
            { key: 'email', label: 'Correo', children: person.email },
            { key: 'timezone', label: 'Zona horaria', children: person.timezone || '—' },
            { key: 'joined', label: 'Se unió', children: joined },
          ]}
        />
        <div className={styles.teams} aria-label="Equipos de la persona">
          <Typography.Title level={5}>Equipos</Typography.Title>
          {teams === null && <Typography.Text type="secondary">No se pudieron cargar los equipos.</Typography.Text>}
          {teams !== null && teams.length === 0 && <Typography.Text type="secondary">Sin equipos.</Typography.Text>}
          {teams !== null && teams.length > 0 && (
            <ul className={styles.teamList}>
              {teams.map((team) => (
                <li key={team.id}>
                  <TeamChip id={team.id} name={team.name} memberCount={team.memberCount} href={`/teams/${team.id}`} />
                </li>
              ))}
            </ul>
          )}
        </div>
        <Link to="/people" className={styles.back}>Volver al directorio</Link>
      </Section>
    </PageContainer>
  )
}
