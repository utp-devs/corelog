import { DeleteOutlined, EditOutlined } from '@ant-design/icons'
import { Button, Tabs } from 'antd'
import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'

import { useAuthStore } from '@/adapters/in/state/auth.store'
import { useCurrentOrg } from '@/adapters/in/state/org.store'
import { useTeamsStore } from '@/adapters/in/state/teams.store'
import { extractErrorMessage } from '@/adapters/out/http/api-client'
import { canManageTeam } from '@/core/teams/domain/team-policy'
import { ConfirmDangerModal } from '@/shared/ui/ConfirmDangerModal'
import { ErrorState } from '@/shared/ui/ErrorState'
import { LoadingSkeleton } from '@/shared/ui/LoadingSkeleton'
import { PageContainer } from '@/shared/ui/PageContainer'
import { Section } from '@/shared/ui/Section'
import { toast } from '@/shared/ui/Toaster/toast'
import { TeamFormModal, type TeamFormValues } from './TeamFormModal'
import { TeamMembersSection } from './TeamMembersSection'
import { TeamTile } from './TeamTile'
import styles from './TeamPage.module.css'

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString('es-ES', { day: 'numeric', month: 'long', year: 'numeric' })
}

/** Team detail: header with actions for owners, admins and maintainers, the members tab and a summary panel. */
export function TeamPage() {
  const { id = '' } = useParams()
  const navigate = useNavigate()

  const team = useTeamsStore((s) => s.currentTeam)
  const teamStatus = useTeamsStore((s) => s.currentTeamStatus)
  const error = useTeamsStore((s) => s.error)
  const members = useTeamsStore((s) => s.members)
  const membersStatus = useTeamsStore((s) => s.membersStatus)
  const membersError = useTeamsStore((s) => s.membersError)
  const fetchTeam = useTeamsStore((s) => s.fetchTeam)
  const fetchMembers = useTeamsStore((s) => s.fetchMembers)
  const updateTeam = useTeamsStore((s) => s.updateTeam)
  const removeTeam = useTeamsStore((s) => s.removeTeam)

  const { role: orgRole } = useCurrentOrg()
  const viewerId = useAuthStore((s) => s.user?.id ?? null)

  const [editOpen, setEditOpen] = useState(false)
  const [deleteOpen, setDeleteOpen] = useState(false)
  const [deleting, setDeleting] = useState(false)

  useEffect(() => {
    void fetchTeam(id)
    void fetchMembers(id)
  }, [id, fetchTeam, fetchMembers])

  const teamReady = team !== null && team.id === id
  const viewerTeamRole = members.find((m) => m.userId === viewerId)?.role ?? null
  const canManage = canManageTeam(orgRole, viewerTeamRole)

  const handleEdit = async (values: TeamFormValues) => {
    if (!team) return
    await updateTeam(team.id, values)
    toast.success('Equipo actualizado')
    setEditOpen(false)
  }

  const handleDelete = async () => {
    if (!team) return
    setDeleting(true)
    try {
      await removeTeam(team.id)
      toast.success('Equipo eliminado')
      navigate('/teams', { replace: true })
    } catch (err) {
      toast.error(extractErrorMessage(err, 'No se pudo eliminar el equipo'))
    } finally {
      setDeleting(false)
      setDeleteOpen(false)
    }
  }

  if (!teamReady) {
    const failed = teamStatus === 'error'
    const message =
      error?.code === 'team.not_found'
        ? 'Este equipo no existe o no tienes acceso.'
        : (error?.message ?? 'No se pudo cargar el equipo.')
    return (
      <PageContainer variant="wide" ready={false} pageName="team">
        {failed ? (
          <ErrorState message={message} onRetry={() => void fetchTeam(id)} />
        ) : (
          <LoadingSkeleton variant="detail" />
        )}
      </PageContainer>
    )
  }

  const creatorName = !team.createdBy
    ? 'No disponible'
    : (members.find((m) => m.userId === team.createdBy)?.name ?? 'Persona de la organización')

  return (
    <PageContainer
      variant="wide"
      ready
      pageName="team"
      sidebar={
        <Section title="Detalles" headingLevel={3}>
          <dl className={styles.details}>
            <dt>Creado por</dt>
            <dd>{creatorName}</dd>
            <dt>Fecha de creación</dt>
            <dd>
              <time dateTime={team.createdAt}>{formatDate(team.createdAt)}</time>
            </dd>
            <dt>Miembros</dt>
            <dd>{membersStatus === 'error' ? 'No disponible' : members.length}</dd>
          </dl>
        </Section>
      }
    >
      <header className={styles.header}>
        <div className={styles.identity}>
          <TeamTile id={team.id} name={team.name} color={team.avatarColor} size="lg" />
          <div className={styles.titleBlock}>
            <h1 className={styles.title}>{team.name}</h1>
            <p className={styles.description}>{team.description || 'Sin descripción'}</p>
          </div>
        </div>
        {canManage && (
          <div className={styles.actions}>
            <Button icon={<EditOutlined aria-hidden="true" />} onClick={() => setEditOpen(true)}>
              Editar equipo
            </Button>
            <Button danger icon={<DeleteOutlined aria-hidden="true" />} onClick={() => setDeleteOpen(true)}>
              Eliminar equipo
            </Button>
          </div>
        )}
      </header>

      <Tabs
        items={[
          {
            key: 'members',
            label: 'Miembros',
            children: (
              <TeamMembersSection
                teamId={team.id}
                members={members}
                status={membersStatus}
                error={membersError}
                canManage={canManage}
                onRetry={() => void fetchMembers(team.id)}
              />
            ),
          },
        ]}
      />

      <TeamFormModal
        open={editOpen}
        title="Editar equipo"
        initial={{ name: team.name, description: team.description, avatarColor: team.avatarColor }}
        onSubmit={handleEdit}
        onCancel={() => setEditOpen(false)}
      />

      <ConfirmDangerModal
        open={deleteOpen}
        title="Eliminar equipo"
        description="Los miembros dejarán de pertenecer a este equipo. Esta acción no se puede deshacer."
        confirmation={team.name}
        confirmText="Eliminar equipo"
        loading={deleting}
        onConfirm={() => void handleDelete()}
        onCancel={() => setDeleteOpen(false)}
      />

    </PageContainer>
  )
}
