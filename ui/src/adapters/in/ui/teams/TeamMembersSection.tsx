import { UserDeleteOutlined } from '@ant-design/icons'
import { Button, Select } from 'antd'
import { useEffect, useMemo, useRef, useState } from 'react'

import { useTeamsStore, type TeamMemberView } from '@/adapters/in/state/teams.store'
import { UserDirectoryHttpRepository } from '@/adapters/out/http/user-directory-http-repository'
import { extractErrorMessage } from '@/adapters/out/http/api-client'
import { UserDirectoryService } from '@/core/users/application/user-directory.service'
import type { PickableUser } from '@/shared/ui/UserPicker'
import type { AppError } from '@/core/shared/app-error'
import type { TeamMemberRole } from '@/core/teams/domain/team'
import { DataTable, type DataTableColumn } from '@/shared/ui/DataTable'
import { EmptyState } from '@/shared/ui/EmptyState'
import { Lozenge } from '@/shared/ui/Lozenge'
import { Section } from '@/shared/ui/Section'
import { toast } from '@/shared/ui/Toaster/toast'
import { UserChip } from '@/shared/ui/UserChip'
import { UserPicker } from '@/shared/ui/UserPicker'
import styles from './TeamMembersSection.module.css'

const directoryService = new UserDirectoryService(new UserDirectoryHttpRepository())
const SEARCH_DEBOUNCE_MS = 300

const ROLE_OPTIONS: { value: TeamMemberRole; label: string }[] = [
  { value: 'maintainer', label: 'Responsable' },
  { value: 'member', label: 'Miembro' },
]

function memberErrorMessage(error: unknown, fallback: string): string {
  const code = (error as AppError).code
  if (code === 'team.member_exists') return 'Esa persona ya es miembro del equipo.'
  if (code === 'team.user_not_in_org') return 'Esa persona no pertenece a la organización.'
  if (code === 'team.forbidden') return 'No tienes permiso para gestionar los miembros de este equipo.'
  return extractErrorMessage(error, fallback)
}

interface TeamMembersSectionProps {
  teamId: string
  members: TeamMemberView[]
  status: 'idle' | 'loading' | 'error'
  error: AppError | null
  /** Owners, admins and team maintainers manage members; everybody else reads the table. */
  canManage: boolean
  onRetry: () => void
}

/** Members tab of the team page: the table of memberships with roles, and adding people from the org directory. */
export function TeamMembersSection({ teamId, members, status, error, canManage, onRetry }: TeamMembersSectionProps) {
  const addMember = useTeamsStore((s) => s.addMember)
  const updateMemberRole = useTeamsStore((s) => s.updateMemberRole)
  const removeMember = useTeamsStore((s) => s.removeMember)

  const [pickedId, setPickedId] = useState<string | undefined>(undefined)
  const [newRole, setNewRole] = useState<TeamMemberRole>('member')
  const [candidates, setCandidates] = useState<PickableUser[]>([])
  const [searching, setSearching] = useState(false)
  const [adding, setAdding] = useState(false)
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)

  useEffect(() => () => clearTimeout(timer.current), [])

  const memberIds = useMemo(() => new Set(members.map((m) => m.userId)), [members])
  const pickable = candidates.filter((user) => !memberIds.has(user.id))

  const handleSearch = (term: string) => {
    clearTimeout(timer.current)
    setSearching(true)
    timer.current = setTimeout(() => {
      directoryService
        .searchTeammates(term.trim())
        .then((users) => setCandidates(users.map((u) => ({ id: u.id, name: u.name, email: u.email }))))
        .catch(() => setCandidates([]))
        .finally(() => setSearching(false))
    }, SEARCH_DEBOUNCE_MS)
  }

  const handleAdd = async () => {
    if (!pickedId) return
    setAdding(true)
    try {
      await addMember(teamId, pickedId, newRole)
      toast.success('Miembro agregado')
      setPickedId(undefined)
    } catch (err) {
      toast.error(memberErrorMessage(err, 'No se pudo agregar el miembro'))
    } finally {
      setAdding(false)
    }
  }

  const handleRoleChange = async (userId: string, role: TeamMemberRole) => {
    try {
      await updateMemberRole(teamId, userId, role)
      toast.success('Rol actualizado')
    } catch (err) {
      toast.error(memberErrorMessage(err, 'No se pudo cambiar el rol'))
    }
  }

  const handleRemove = async (member: TeamMemberView) => {
    try {
      await removeMember(teamId, member.userId)
      toast.success(`${member.name} ya no pertenece al equipo`)
    } catch (err) {
      toast.error(memberErrorMessage(err, 'No se pudo quitar el miembro'))
    }
  }

  const roleControl = (member: TeamMemberView) =>
    canManage ? (
      <Select<TeamMemberRole>
        aria-label={`Rol de ${member.name}`}
        className={styles.roleSelect}
        value={member.role}
        options={ROLE_OPTIONS}
        onChange={(role) => void handleRoleChange(member.userId, role)}
      />
    ) : (
      <Lozenge tone={member.role === 'maintainer' ? 'info' : 'neutral'}>
        {ROLE_OPTIONS.find((option) => option.value === member.role)?.label}
      </Lozenge>
    )

  const removeButton = (member: TeamMemberView) =>
    canManage ? (
      <Button
        danger
        size="small"
        icon={<UserDeleteOutlined aria-hidden="true" />}
        aria-label={`Quitar a ${member.name} del equipo`}
        onClick={() => void handleRemove(member)}
      >
        Quitar
      </Button>
    ) : null

  const columns: DataTableColumn<TeamMemberView>[] = [
    {
      key: 'person',
      title: 'Persona',
      mobilePrimary: true,
      render: (_value, member) => <UserChip user={{ id: member.userId, name: member.name, role: member.email || null }} />,
    },
    {
      key: 'role',
      title: 'Rol',
      mobilePrimary: true,
      render: (_value, member) => roleControl(member),
    },
    {
      key: 'actions',
      title: '',
      align: 'right',
      render: (_value, member) => removeButton(member),
    },
  ]

  const errorMessage =
    status === 'error'
      ? error?.code === 'team.forbidden'
        ? 'No tienes permiso para ver los miembros de este equipo.'
        : (error?.message ?? 'No se pudieron cargar los miembros.')
      : null

  return (
    <Section title="Miembros" description={canManage ? undefined : 'Solo lectura: los responsables del equipo gestionan a sus miembros.'}>
      {canManage && (
        <div className={styles.addForm} role="group" aria-label="Agregar miembro">
          <div className={styles.picker}>
            <UserPicker
              users={pickable}
              value={pickedId}
              onChange={setPickedId}
              onSearch={handleSearch}
              loading={searching}
              placeholder="Buscar una persona de la organización"
            />
          </div>
          <Select<TeamMemberRole>
            aria-label="Rol del nuevo miembro"
            className={styles.roleSelect}
            value={newRole}
            options={ROLE_OPTIONS}
            onChange={setNewRole}
          />
          <Button type="primary" disabled={!pickedId} loading={adding} onClick={() => void handleAdd()}>
            Agregar miembro
          </Button>
        </div>
      )}
      <DataTable<TeamMemberView>
        tableKey="team-members"
        columns={columns}
        data={members}
        rowKey="userId"
        loading={status === 'loading'}
        error={errorMessage}
        onRetry={onRetry}
        emptyState={
          <EmptyState
            title="Este equipo aún no tiene miembros"
            description={canManage ? 'Agrega personas con el formulario de arriba.' : undefined}
          />
        }
        mobileRender={(member) => (
          <div className={styles.mobileCard}>
            <UserChip user={{ id: member.userId, name: member.name, role: member.email || null }} />
            <div className={styles.mobileActions}>
              {roleControl(member)}
              {removeButton(member)}
            </div>
          </div>
        )}
      />
    </Section>
  )
}
