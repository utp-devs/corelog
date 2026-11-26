import { PlusOutlined } from '@ant-design/icons'
import { Button, Skeleton } from 'antd'
import { useEffect, useMemo, useState, type ReactNode } from 'react'

import { useCurrentOrg } from '@/adapters/in/state/org.store'
import { useTeamsStore } from '@/adapters/in/state/teams.store'
import { canCreateTeam, filterTeams } from '@/core/teams/domain/team-policy'
import { EmptyState } from '@/shared/ui/EmptyState'
import { ErrorState } from '@/shared/ui/ErrorState'
import { CardGrid } from '@/shared/ui/CardGrid'
import { PageContainer } from '@/shared/ui/PageContainer'
import { PageHeader } from '@/shared/ui/PageHeader'
import { Section } from '@/shared/ui/Section'
import { SearchInput } from '@/shared/ui/SearchInput'
import { toast } from '@/shared/ui/Toaster/toast'
import { Toolbar } from '@/shared/ui/Toolbar'
import { TeamCard } from './TeamCard'
import { TeamFormModal, type TeamFormValues } from './TeamFormModal'
import styles from './TeamsPage.module.css'

const SKELETON_CARDS = 6

/** Teams of the active organization. Anyone the org policy allows can create one; guests only browse. */
export function TeamsPage() {
  const teams = useTeamsStore((s) => s.teams)
  const status = useTeamsStore((s) => s.status)
  const error = useTeamsStore((s) => s.error)
  const fetchTeams = useTeamsStore((s) => s.fetchTeams)
  const createTeam = useTeamsStore((s) => s.createTeam)
  const { org, role } = useCurrentOrg()
  const canCreate = canCreateTeam(role, org?.settings.whoCanCreateTeams)

  const [query, setQuery] = useState('')
  const [createOpen, setCreateOpen] = useState(false)

  useEffect(() => {
    void fetchTeams()
  }, [fetchTeams])

  const visible = useMemo(() => filterTeams(teams, query), [teams, query])
  const loading = status === 'loading' && teams.length === 0
  const failed = status === 'error' && teams.length === 0

  const createButton = canCreate ? (
    <Button type="primary" icon={<PlusOutlined aria-hidden="true" />} onClick={() => setCreateOpen(true)}>
      Crear equipo
    </Button>
  ) : undefined

  const handleCreate = async (values: TeamFormValues) => {
    await createTeam(values)
    toast.success('Equipo creado')
    setCreateOpen(false)
  }

  let body: ReactNode
  if (loading) {
    body = (
      <div role="status" aria-busy="true" aria-label="Cargando equipos">
        <CardGrid>
          {Array.from({ length: SKELETON_CARDS }, (_, index) => (
            <div key={index} className={styles.skeleton} aria-hidden="true">
              <Skeleton active avatar={{ shape: 'square' }} paragraph={{ rows: 2 }} title={{ width: '60%' }} />
            </div>
          ))}
        </CardGrid>
      </div>
    )
  } else if (failed) {
    body = (
      <Section fill ariaLabel="Equipos">
        <ErrorState
          message={error?.message ?? 'No se pudieron cargar los equipos.'}
          onRetry={() => void fetchTeams()}
        />
      </Section>
    )
  } else if (teams.length === 0) {
    body = (
      <Section fill ariaLabel="Equipos">
        <EmptyState
          title="Aún no hay equipos"
          description={
            canCreate
              ? 'Crea el primer equipo para agrupar a las personas de la organización.'
              : 'Pide a un administrador de la organización que cree un equipo.'
          }
          action={canCreate ? createButton : undefined}
        />
      </Section>
    )
  } else if (visible.length === 0) {
    body = (
      <Section fill ariaLabel="Equipos">
        <EmptyState title="Ningún equipo coincide con la búsqueda" description="Prueba con otro nombre o descripción." />
      </Section>
    )
  } else {
    body = (
      <CardGrid ariaLabel="Equipos">
        {visible.map((team) => (
          <TeamCard key={team.id} team={team} />
        ))}
      </CardGrid>
    )
  }

  return (
    <PageContainer variant="wide" ready={!loading} pageName="teams" fill>
      <PageHeader title="Equipos" description="Agrupa a las personas de la organización y administra sus equipos." />
      <Toolbar
        left={<SearchInput value={query} onChange={setQuery} placeholder="Buscar equipos" ariaLabel="Buscar equipos" />}
        right={teams.length > 0 ? createButton : undefined}
      />
      {body}
      <TeamFormModal
        open={createOpen}
        title="Nuevo equipo"
        initial={{ name: '', description: '', avatarColor: null }}
        onSubmit={handleCreate}
        onCancel={() => setCreateOpen(false)}
      />
    </PageContainer>
  )
}
