import { PlusOutlined } from '@ant-design/icons'
import { App, Button } from 'antd'
import { useEffect, useState } from 'react'

import type { Team } from '@/core/teams/domain/team'
import { useTeamsStore } from '@/adapters/in/state/teams.store'
import { extractErrorMessage } from '@/adapters/out/http/api-client'
import { TeamFormModal } from './TeamFormModal'
import { TeamList } from './TeamList'
import { TeamMembersPanel } from './TeamMembersPanel'
import styles from './TeamsPage.module.css'
import { ConfirmModal } from '@/shared/ui/ConfirmModal'
import { EmptyState } from '@/shared/ui/EmptyState'
import { PageHeader } from '@/shared/ui/PageHeader'

type ModalState = { kind: 'create' } | { kind: 'rename'; team: Team } | { kind: 'closed' }

export function TeamsPage() {
  const { message } = App.useApp()
  const teams = useTeamsStore((s) => s.teams)
  const status = useTeamsStore((s) => s.status)
  const fetchTeams = useTeamsStore((s) => s.fetchTeams)
  const createTeam = useTeamsStore((s) => s.createTeam)
  const renameTeam = useTeamsStore((s) => s.renameTeam)
  const removeTeam = useTeamsStore((s) => s.removeTeam)

  const [selectedTeam, setSelectedTeam] = useState<Team | null>(null)
  const [modal, setModal] = useState<ModalState>({ kind: 'closed' })
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [pendingDelete, setPendingDelete] = useState<Team | null>(null)

  useEffect(() => {
    fetchTeams()
  }, [fetchTeams])

  const handleSubmitModal = async (name: string) => {
    setIsSubmitting(true)
    try {
      if (modal.kind === 'create') {
        await createTeam(name)
        message.success('Equipo creado')
      } else if (modal.kind === 'rename') {
        await renameTeam(modal.team.id, name)
        message.success('Equipo renombrado')
      }
      setModal({ kind: 'closed' })
    } catch (error) {
      message.error(extractErrorMessage(error, 'No se pudo guardar el equipo'))
      throw error
    } finally {
      setIsSubmitting(false)
    }
  }

  const handleDelete = async () => {
    if (!pendingDelete) return
    const team = pendingDelete
    setPendingDelete(null)
    try {
      await removeTeam(team.id)
      message.success('Equipo eliminado')
      if (selectedTeam?.id === team.id) {
        setSelectedTeam(null)
      }
    } catch (error) {
      message.error(extractErrorMessage(error, 'No se pudo eliminar el equipo'))
    }
  }

  return (
    <div>
      <PageHeader
        title="Equipos"
        description="Organiza los miembros y administra los equipos."
        actions={
          <Button type="primary" icon={<PlusOutlined />} onClick={() => setModal({ kind: 'create' })}>
            Nuevo equipo
          </Button>
        }
      />

      <TeamList
        teams={teams}
        isLoading={status === 'loading'}
        selectedTeamId={selectedTeam?.id ?? null}
        onSelect={setSelectedTeam}
        onRename={(team) => setModal({ kind: 'rename', team })}
        onDelete={setPendingDelete}
      />

      <div className={styles.membersSection}>
        {selectedTeam ? (
          <TeamMembersPanel key={selectedTeam.id} team={selectedTeam} />
        ) : (
          <EmptyState title="Selecciona un equipo" description="Elige un equipo de la lista para administrar sus miembros." />
        )}
      </div>

      <ConfirmModal
        open={pendingDelete !== null}
        title={pendingDelete ? `Eliminar el equipo ${pendingDelete.name}` : ''}
        description="Sus miembros dejarán de pertenecer a este equipo. Esta acción no se puede deshacer."
        confirmText="Eliminar equipo"
        danger
        onConfirm={() => void handleDelete()}
        onCancel={() => setPendingDelete(null)}
      />

      <TeamFormModal
        open={modal.kind !== 'closed'}
        title={modal.kind === 'rename' ? 'Renombrar equipo' : 'Nuevo equipo'}
        initialName={modal.kind === 'rename' ? modal.team.name : ''}
        isSubmitting={isSubmitting}
        onSubmit={handleSubmitModal}
        onCancel={() => setModal({ kind: 'closed' })}
      />
    </div>
  )
}
