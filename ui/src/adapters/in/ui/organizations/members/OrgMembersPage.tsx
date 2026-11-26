import { Alert, Button, Select, Tooltip, Typography } from 'antd'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'

import { useAuthStore } from '@/adapters/in/state/auth.store'
import { useCurrentOrg } from '@/adapters/in/state/org.store'
import { useOrgAdminStore, type OrgAdminStore } from '@/adapters/in/state/org-admin.store'
import {
  assignableRoles,
  canRemoveMember,
  canViewMembers,
} from '@/core/organizations/domain/org-permissions'
import { canManageOrg, ORG_ROLES, type OrganizationMember, type OrgRole } from '@/core/organizations/domain/organization'
import { UserAvatar } from '@/adapters/in/ui/people/UserAvatar'
import { ConfirmDangerModal } from '@/shared/ui/ConfirmDangerModal'
import { ConfirmModal } from '@/shared/ui/ConfirmModal'
import { DataTable, type DataTableColumn } from '@/shared/ui/DataTable'
import { EmptyState } from '@/shared/ui/EmptyState'
import { Lozenge } from '@/shared/ui/Lozenge'
import { PageContainer } from '@/shared/ui/PageContainer'
import { PageHeader } from '@/shared/ui/PageHeader'
import { SearchInput } from '@/shared/ui/SearchInput'
import { Toolbar } from '@/shared/ui/Toolbar'
import { toast } from '@/shared/ui/Toaster'
import { adminErrorMessage } from './member-messages'
import { formatOrgDate, ORG_ROLE_LABELS, ORG_ROLE_TONES } from './role-labels'
import styles from './OrgMembersPage.module.css'

export interface OrgMembersPageProps {
  /** Injected for tests; the app uses the application singleton. */
  store?: OrgAdminStore
}

type RoleFilter = OrgRole | 'all'

const ROLE_FILTER_OPTIONS = [
  { value: 'all', label: 'Todos los roles' },
  ...ORG_ROLES.map((role) => ({ value: role, label: ORG_ROLE_LABELS[role] })),
]

interface PendingRoleChange {
  member: OrganizationMember
  role: OrgRole
}

function matchesQuery(member: OrganizationMember, query: string): boolean {
  const needle = query.trim().toLocaleLowerCase()
  if (!needle) return true
  return member.name.toLocaleLowerCase().includes(needle) || member.email.toLocaleLowerCase().includes(needle)
}

/** Members of the active organization (S-21). Role changes and removals are confirmed and enforced by the API. */
export function OrgMembersPage({ store = useOrgAdminStore }: OrgMembersPageProps) {
  const navigate = useNavigate()
  const { org, role } = useCurrentOrg()
  const currentUserId = useAuthStore((s) => s.user?.id ?? null)
  const members = store((s) => s.members)
  const status = store((s) => s.membersStatus)
  const loadError = store((s) => s.membersError)
  const orgId = org?.id ?? null
  const canManage = canManageOrg(role)
  const canView = canViewMembers(role)

  const [query, setQuery] = useState('')
  const [roleFilter, setRoleFilter] = useState<RoleFilter>('all')
  const [selected, setSelected] = useState<string[]>([])
  const [pendingRole, setPendingRole] = useState<PendingRoleChange | null>(null)
  const [roleError, setRoleError] = useState<string | null>(null)
  const [removing, setRemoving] = useState<OrganizationMember | null>(null)
  const [bulkOpen, setBulkOpen] = useState(false)
  const [busy, setBusy] = useState(false)

  const load = useCallback(() => {
    if (orgId) void store.getState().fetchMembers(orgId)
  }, [store, orgId])

  useEffect(() => {
    if (orgId && canView) load()
  }, [orgId, canView, load])

  const isSelf = useCallback((member: OrganizationMember) => member.userId === currentUserId, [currentUserId])

  const removable = useMemo(
    () => members.filter((m) => canRemoveMember(role, m.role, isSelf(m))),
    [members, role, isSelf],
  )
  const removableIds = useMemo(() => new Set(removable.map((m) => m.userId)), [removable])

  const visible = useMemo(
    () => members.filter((m) => (roleFilter === 'all' || m.role === roleFilter) && matchesQuery(m, query)),
    [members, roleFilter, query],
  )

  const selectedRemovable = selected.filter((id) => removableIds.has(id))

  const changeRole = async () => {
    if (!pendingRole || !orgId) return
    setBusy(true)
    setRoleError(null)
    try {
      await store.getState().changeMemberRole(orgId, pendingRole.member.userId, pendingRole.role)
      toast.success(`Rol de ${pendingRole.member.name} actualizado`)
    } catch (error) {
      setRoleError(adminErrorMessage(error, 'No se pudo cambiar el rol.'))
    } finally {
      setBusy(false)
      setPendingRole(null)
    }
  }

  const removeOne = async () => {
    if (!removing || !orgId) return
    setBusy(true)
    try {
      await store.getState().removeMember(orgId, removing.userId)
      toast.success(`${removing.name} ya no es miembro de la organización`)
    } catch (error) {
      toast.error(adminErrorMessage(error, 'No se pudo quitar al miembro.'))
    } finally {
      setBusy(false)
      setRemoving(null)
    }
  }

  const removeSelected = async () => {
    if (!orgId) return
    setBusy(true)
    const targets = members.filter((m) => selectedRemovable.includes(m.userId))
    let failures = 0
    for (const target of targets) {
      try {
        await store.getState().removeMember(orgId, target.userId)
      } catch {
        failures += 1
      }
    }
    setBusy(false)
    setBulkOpen(false)
    setSelected([])
    if (failures === 0) toast.success(`${targets.length} miembros quitados`)
    else toast.error(`No se pudieron quitar ${failures} de ${targets.length} miembros.`)
  }

  /** Role control: a select where the actor may change the role, otherwise the role with the reason. */
  const roleCell = (member: OrganizationMember) => {
    const self = isSelf(member)
    const options = assignableRoles(role, member.role, self)
    if (!canManage || options.length === 0) {
      const reason = self
        ? 'No puedes cambiar tu propio rol.'
        : role === 'admin'
          ? 'Solo los propietarios pueden cambiar este rol.'
          : 'No tienes permiso para cambiar este rol.'
      return (
        <Tooltip title={canManage || self ? reason : undefined}>
          <span className={styles.roleStatic}>
            <Lozenge tone={ORG_ROLE_TONES[member.role]}>{ORG_ROLE_LABELS[member.role]}</Lozenge>
          </span>
        </Tooltip>
      )
    }
    return (
      <Select
        aria-label={`Rol de ${member.name}`}
        size="small"
        className={styles.roleSelect}
        value={member.role}
        onChange={(next: OrgRole) => {
          if (next !== member.role) setPendingRole({ member, role: next })
        }}
        options={options.map((item) => ({ value: item, label: ORG_ROLE_LABELS[item] }))}
      />
    )
  }

  const columns: DataTableColumn<OrganizationMember>[] = [
    {
      key: 'name',
      title: 'Persona',
      mobilePrimary: true,
      render: (_value, member) => (
        <span className={styles.person}>
          <UserAvatar id={member.userId} name={member.name} hasAvatar={member.hasAvatar} size={32} />
          <span className={styles.personName}>
            {member.name}
            {isSelf(member) && <span className={styles.youTag}> (tú)</span>}
          </span>
        </span>
      ),
    },
    {
      key: 'jobTitle',
      title: 'Cargo',
      render: (_value, member) => member.jobTitle || <Typography.Text type="secondary">—</Typography.Text>,
    },
    {
      key: 'email',
      title: 'Correo',
      render: (_value, member) => <Typography.Text type="secondary">{member.email}</Typography.Text>,
    },
    {
      key: 'role',
      title: 'Rol',
      mobilePrimary: true,
      render: (_value, member) => roleCell(member),
    },
    {
      key: 'joinedAt',
      title: 'Se unió',
      render: (_value, member) => formatOrgDate(member.joinedAt),
    },
  ]

  const emptyState = query.trim() || roleFilter !== 'all' ? (
    <EmptyState title="Sin resultados" description="No hay miembros que coincidan con los filtros." />
  ) : (
    <EmptyState
      title="Aún no hay miembros"
      description="Invita a tus compañeros para empezar a colaborar."
      action={
        canManage ? (
          <Button type="primary" onClick={() => navigate('/organization/invitations')}>
            Invitar personas
          </Button>
        ) : undefined
      }
    />
  )

  if (org && !canView) {
    return (
      <PageContainer variant="wide" pageName="org-members" ready>
        <PageHeader title="Miembros" description={org.name} />
        <EmptyState
          title="No tienes permiso para ver esto"
          description="Pide acceso a un administrador de la organización."
        />
      </PageContainer>
    )
  }

  const loading = orgId !== null && (status === 'idle' || status === 'loading')
  const error = orgId && status === 'error' ? (loadError?.message ?? 'No se pudo cargar el directorio.') : null

  return (
    <PageContainer variant="wide" pageName="org-members" ready={!loading} fill>
      <PageHeader
        title="Miembros"
        description={org ? `Personas de ${org.name}` : undefined}
        actions={
          canManage ? (
            <Button type="primary" onClick={() => navigate('/organization/invitations')}>
              Invitar personas
            </Button>
          ) : undefined
        }
      />

      {role === 'member' && (
        <Alert
          type="info"
          showIcon
          className={styles.banner}
          message="Solo lectura"
          description="Puedes ver a los miembros de la organización. Solo los administradores pueden cambiar roles o invitar personas."
        />
      )}

      {roleError && (
        <Alert type="error" showIcon role="alert" className={styles.banner} message={roleError} closable onClose={() => setRoleError(null)} />
      )}

      <Toolbar
        left={
          <>
            <SearchInput
              value={query}
              onChange={setQuery}
              placeholder="Buscar por nombre o correo"
              ariaLabel="Buscar miembros"
            />
            <Select
              aria-label="Filtrar por rol"
              value={roleFilter}
              onChange={(value: RoleFilter) => setRoleFilter(value)}
              options={ROLE_FILTER_OPTIONS}
              className={styles.filterSelect}
            />
          </>
        }
      />

      <DataTable<OrganizationMember>
        tableKey="org-members"
        fill
        columns={columns}
        data={orgId ? visible : []}
        rowKey="userId"
        loading={loading}
        error={error}
        onRetry={load}
        emptyState={emptyState}
        selection={
          canManage && removable.length > 0
            ? {
                selectedKeys: selectedRemovable,
                onChange: (keys) => setSelected(keys.filter((key) => removableIds.has(key))),
                actions: [
                  {
                    key: 'remove',
                    label: 'Quitar miembros',
                    danger: true,
                    onClick: () => setBulkOpen(true),
                  },
                ],
              }
            : undefined
        }
        rowActions={
          canManage
            ? (member) =>
                canRemoveMember(role, member.role, isSelf(member))
                  ? [{ key: 'remove', label: 'Quitar de la organización', danger: true, onClick: () => setRemoving(member) }]
                  : []
            : undefined
        }
        mobileRender={(member) => (
          <div className={styles.mobileCard}>
            <span className={styles.personName}>{member.name}</span>
            <Typography.Text type="secondary">{member.email}</Typography.Text>
            {roleCell(member)}
          </div>
        )}
      />

      <ConfirmModal
        open={pendingRole !== null}
        title="Cambiar rol"
        description={
          pendingRole
            ? `¿Cambiar el rol de ${pendingRole.member.name} de ${ORG_ROLE_LABELS[pendingRole.member.role]} a ${ORG_ROLE_LABELS[pendingRole.role]}?${
                pendingRole.role === 'owner' ? ' Un propietario controla toda la organización.' : ''
              }`
            : undefined
        }
        confirmText="Cambiar rol"
        loading={busy}
        onConfirm={changeRole}
        onCancel={() => setPendingRole(null)}
      />

      <ConfirmDangerModal
        open={removing !== null}
        title="Quitar de la organización"
        description={removing ? `${removing.name} perderá el acceso a los proyectos y equipos de la organización.` : undefined}
        confirmation={removing?.email ?? ''}
        confirmText="Quitar"
        loading={busy}
        onConfirm={removeOne}
        onCancel={() => setRemoving(null)}
      />

      <ConfirmModal
        open={bulkOpen}
        title={`¿Quitar ${selectedRemovable.length} miembros?`}
        description="Perderán el acceso a los proyectos y equipos de la organización."
        confirmText="Quitar miembros"
        danger
        loading={busy}
        onConfirm={removeSelected}
        onCancel={() => setBulkOpen(false)}
      />
    </PageContainer>
  )
}
