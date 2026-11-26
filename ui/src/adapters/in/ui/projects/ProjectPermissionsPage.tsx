import { CheckOutlined, MinusOutlined } from '@ant-design/icons'
import { Alert, Skeleton, Tag, Typography } from 'antd'
import { useEffect } from 'react'
import { useParams } from 'react-router-dom'

import { useAuthStore } from '@/adapters/in/state/auth.store'
import { useProjectsStore } from '@/adapters/in/state/projects.store'
import type { ProjectMemberRole } from '@/core/projects/domain/project'
import {
  effectiveProjectRole,
  hasProjectPermission,
  projectPermissions,
  type EffectiveProjectRole,
  type PermissionDefinition,
} from '@/core/projects/domain/permissions'
import { DataTable, type DataTableColumn } from '@/shared/ui/DataTable'
import { EmptyState } from '@/shared/ui/EmptyState'
import { ErrorPage } from '@/shared/ui/ErrorPage'
import { Lozenge } from '@/shared/ui/Lozenge'
import { PageContainer } from '@/shared/ui/PageContainer'
import { PageHeader } from '@/shared/ui/PageHeader'
import { Section } from '@/shared/ui/Section'
import { StatTile } from '@/shared/ui/StatTile'
import { accessVariantOf } from './projectLoadError'
import styles from './ProjectPermissionsPage.module.css'

// Columnas de la matriz, en el orden en que se lee de mayor a menor alcance.
const columns: { key: EffectiveProjectRole; label: string }[] = [
  { key: 'admin', label: 'Administrador' },
  { key: 'lead', label: 'Líder' },
  { key: 'contributor', label: 'Colaborador' },
  { key: 'viewer', label: 'Observador' },
]

const roleNames: Record<Exclude<EffectiveProjectRole, null>, string> = {
  admin: 'Administrador',
  lead: 'Líder',
  contributor: 'Colaborador',
  viewer: 'Observador',
}

const roleDescriptions: Record<Exclude<EffectiveProjectRole, null>, string> = {
  admin: 'Global: acceso a todos los proyectos.',
  lead: 'Dirige el proyecto y su configuración.',
  contributor: 'Trabaja en las incidencias.',
  viewer: 'Solo consulta y sigue incidencias.',
}

function grantedOf(role: EffectiveProjectRole, permission: PermissionDefinition): boolean {
  return role !== null && hasProjectPermission(role, permission.key)
}

export function ProjectPermissionsPage() {
  const { id = '' } = useParams()
  const members = useProjectsStore((s) => s.members)
  const status = useProjectsStore((s) => s.status)
  const error = useProjectsStore((s) => s.error)
  const fetchMembers = useProjectsStore((s) => s.fetchMembers)
  const user = useAuthStore((s) => s.user)

  useEffect(() => {
    void fetchMembers(id)
  }, [id, fetchMembers])

  const accessVariant = status === 'error' ? accessVariantOf(error) : null
  if (accessVariant !== null) {
    return (
      <PageContainer variant="reading" pageName="permissions">
        <ErrorPage variant={accessVariant} />
      </PageContainer>
    )
  }

  const loading = status === 'loading' && members.length === 0
  // La matriz no depende de los datos: se muestra aunque la carga de miembros falle.
  const myRole = effectiveProjectRole(user?.role, user?.id, members as { userId: string; role: ProjectMemberRole }[])
  const roleCounts = members.reduce<Record<string, number>>((acc, member) => {
    acc[member.role] = (acc[member.role] ?? 0) + 1
    return acc
  }, {})

  const tableColumns: DataTableColumn<PermissionDefinition>[] = [
    {
      key: 'permission',
      title: 'Permiso',
      mobilePrimary: true,
      render: (_value, permission) => (
        <div className={styles.permission}>
          <Typography.Text strong>{permission.label}</Typography.Text>
          <Typography.Text type="secondary" className={styles.description}>{permission.description}</Typography.Text>
        </div>
      ),
    },
    ...columns.map((column): DataTableColumn<PermissionDefinition> => ({
      key: column.key ?? 'none',
      title: (
        <span className={column.key === myRole ? styles.currentHeader : undefined}>
          {column.label}
          {column.key === myRole && <Tag color="blue" className={styles.youTag}>Tú</Tag>}
        </span>
      ),
      align: 'center',
      width: 140,
      onCell: () => ({ className: column.key === myRole ? styles.currentColumn : undefined }),
      render: (_value, permission) => {
        const granted = column.key !== null && grantedOf(column.key, permission)
        return granted
          ? <CheckOutlined className={styles.granted} aria-label={`${column.label}: permitido`} />
          : <MinusOutlined className={styles.denied} aria-label={`${column.label}: no permitido`} />
      },
    })),
  ]

  return (
    <PageContainer variant="reading" pageName="permissions" ready={!loading} fill>
      <PageHeader title="Permisos" description="Qué puede hacer cada rol en este proyecto." />

      {status === 'error' && (
        <Alert type="warning" showIcon message="No se pudieron cargar los miembros. La matriz de permisos sigue vigente." />
      )}

      <Section title="Tu rol en este proyecto">
        {loading ? (
          <Skeleton active paragraph={{ rows: 2 }} />
        ) : (
          <div className={styles.yourRole}>
            <Lozenge tone="info">{myRole ? roleNames[myRole] : 'Sin acceso como miembro'}</Lozenge>
            <Typography.Text type="secondary">
              {myRole === 'admin'
                ? 'Tienes todos los permisos por ser administrador global.'
                : myRole
                  ? `${roleDescriptions[myRole]} Los permisos que no tienes aparecen con un guion.`
                  : 'No apareces como miembro del proyecto. Pide acceso a un líder.'}
            </Typography.Text>
          </div>
        )}
      </Section>

      <Section title="Miembros por rol">
        {loading ? (
          <Skeleton active paragraph={{ rows: 2 }} />
        ) : (
          <div className={styles.countGrid}>
            {(['lead', 'contributor', 'viewer'] as ProjectMemberRole[]).map((role) => (
              <StatTile key={role} label={roleNames[role]} value={roleCounts[role] ?? 0} />
            ))}
          </div>
        )}
      </Section>

      <Section
        title="Matriz de permisos"
        description="Cada fila es un permiso y cada columna un rol. La columna resaltada es la tuya."
        flush
        fill
      >
        <DataTable<PermissionDefinition>
          tableKey="project-permissions"
          columns={tableColumns}
          data={[...projectPermissions]}
          fill
          rowKey="key"
          loading={loading}
          emptyState={<EmptyState title="Sin permisos definidos" />}
          mobileRender={(permission) => (
            <div className={styles.mobileCard}>
              <Typography.Text strong>{permission.label}</Typography.Text>
              <Typography.Text type="secondary">{permission.description}</Typography.Text>
              <ul className={styles.mobileRoles} aria-label={`Roles con ${permission.label}`}>
                {columns.map((column) => {
                  const granted = column.key !== null && grantedOf(column.key, permission)
                  return (
                    <li key={column.key ?? 'none'} className={styles.mobileRole}>
                      <span>{column.label}{column.key === myRole ? ' (tú)' : ''}</span>
                      <Lozenge tone={granted ? 'success' : 'neutral'}>{granted ? 'Permitido' : 'No permitido'}</Lozenge>
                    </li>
                  )
                })}
              </ul>
            </div>
          )}
        />
      </Section>
    </PageContainer>
  )
}
