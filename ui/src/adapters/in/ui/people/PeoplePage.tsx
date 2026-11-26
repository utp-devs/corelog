import { useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Typography } from 'antd'

import { useCurrentOrg } from '@/adapters/in/state/org.store'
import { PeopleHttpRepository } from '@/adapters/out/http/people-http-repository'
import type { PeopleRepositoryPort } from '@/core/people/application/ports/people-repository.port'
import type { OrganizationMember } from '@/core/people/domain/member'
import { DataTable, type DataTableColumn } from '@/shared/ui/DataTable'
import { EmptyState } from '@/shared/ui/EmptyState'
import { Lozenge } from '@/shared/ui/Lozenge'
import { PageContainer } from '@/shared/ui/PageContainer'
import { PageHeader } from '@/shared/ui/PageHeader'
import { SearchInput } from '@/shared/ui/SearchInput'
import { Toolbar } from '@/shared/ui/Toolbar'
import { describeDirectoryError } from './errors'
import { filterMembers, ORG_ROLE_LABELS, ORG_ROLE_TONES } from './roles'
import { UserAvatar } from './UserAvatar'
import styles from './PeoplePage.module.css'

interface PeoplePageProps {
  /** Injected for tests; the app uses the HTTP repository. */
  repository?: PeopleRepositoryPort
}

interface DirectoryResult {
  key: string
  members: OrganizationMember[]
  error: string | null
}

const defaultRepository = new PeopleHttpRepository()

/** Directory of the members of the active organization (S-27). */
export function PeoplePage({ repository = defaultRepository }: PeoplePageProps) {
  const navigate = useNavigate()
  const { org } = useCurrentOrg()
  const orgId = org?.id ?? null
  const [query, setQuery] = useState('')
  const [attempt, setAttempt] = useState(0)
  const [result, setResult] = useState<DirectoryResult | null>(null)

  // The result is tied to the request key, so a stale answer never shows for another org or attempt.
  const requestKey = orgId ? `${orgId}|${attempt}` : null

  useEffect(() => {
    if (!orgId) return
    const key = `${orgId}|${attempt}`
    let active = true
    repository
      .listMembers(orgId)
      .then((list) => {
        if (active) setResult({ key, members: list, error: null })
      })
      .catch((failure: unknown) => {
        if (active) setResult({ key, members: [], error: describeDirectoryError(failure) })
      })
    return () => {
      active = false
    }
  }, [orgId, attempt, repository])

  const current = result !== null && result.key === requestKey ? result : null
  const loading = requestKey !== null && current === null
  const error = current?.error ?? null
  const members = useMemo(() => current?.members ?? [], [current])
  const visible = useMemo(() => filterMembers(members, query), [members, query])
  const retry = useCallback(() => setAttempt((value) => value + 1), [])

  const columns: DataTableColumn<OrganizationMember>[] = [
    {
      key: 'name',
      title: 'Persona',
      mobilePrimary: true,
      render: (_value, member) => (
        <span className={styles.person}>
          <UserAvatar id={member.userId} name={member.name} hasAvatar={member.hasAvatar} size={32} />
          <span className={styles.personName}>{member.name}</span>
        </span>
      ),
    },
    {
      key: 'jobTitle',
      title: 'Cargo',
      render: (_value, member) => member.jobTitle || <Typography.Text type="secondary">—</Typography.Text>,
    },
    {
      key: 'role',
      title: 'Rol en la organización',
      render: (_value, member) => (
        <Lozenge tone={ORG_ROLE_TONES[member.role]}>{ORG_ROLE_LABELS[member.role]}</Lozenge>
      ),
    },
    {
      key: 'email',
      title: 'Correo',
      render: (_value, member) => <Typography.Text type="secondary">{member.email}</Typography.Text>,
    },
  ]

  const emptyState = !orgId ? (
    <EmptyState title="Elige una organización" description="El directorio muestra las personas de la organización activa." />
  ) : query.trim() ? (
    <EmptyState title="Sin resultados" description={`No hay personas que coincidan con "${query.trim()}".`} />
  ) : (
    <EmptyState title="Aún no hay personas" description="Invita a tus compañeros para verlos aquí." />
  )

  return (
    <PageContainer variant="wide" pageName="people" ready={!loading && error === null}>
      <PageHeader title="Personas" description={org ? `Miembros de ${org.name}` : undefined} />
      <Toolbar
        left={
          <SearchInput
            value={query}
            onChange={setQuery}
            placeholder="Buscar por nombre o correo"
            ariaLabel="Buscar personas"
          />
        }
      />
      <DataTable<OrganizationMember>
        tableKey="people-directory"
        columns={columns}
        data={orgId ? visible : []}
        rowKey="userId"
        loading={loading}
        error={orgId ? error : null}
        onRetry={retry}
        emptyState={emptyState}
        onRowOpen={(member) => navigate(`/people/${member.userId}`)}
        mobileRender={(member) => (
          <div className={styles.mobileCard}>
            <span className={styles.person}>
              <UserAvatar id={member.userId} name={member.name} hasAvatar={member.hasAvatar} size={32} />
              <span className={styles.personName}>{member.name}</span>
            </span>
            {member.jobTitle && <Typography.Text>{member.jobTitle}</Typography.Text>}
            <Lozenge tone={ORG_ROLE_TONES[member.role]}>{ORG_ROLE_LABELS[member.role]}</Lozenge>
            <Typography.Text type="secondary">{member.email}</Typography.Text>
          </div>
        )}
      />
    </PageContainer>
  )
}
