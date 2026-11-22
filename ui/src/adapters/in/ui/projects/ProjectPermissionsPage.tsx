import { CheckOutlined, MinusOutlined } from '@ant-design/icons'
import { Alert, Card, Skeleton, Table, Tag, Typography } from 'antd'
import type { ColumnsType } from 'antd/es/table'
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
import styles from './ProjectPermissionsPage.module.css'

// Columnas de la matriz, en el orden en que se lee de mayor a menor alcance.
const columns: { key: EffectiveProjectRole; label: string; description: string }[] = [
  { key: 'admin', label: 'Administrador', description: 'Global: acceso a todos los proyectos' },
  { key: 'lead', label: 'Líder', description: 'Dirige el proyecto y su configuración' },
  { key: 'contributor', label: 'Colaborador', description: 'Trabaja en las incidencias' },
  { key: 'viewer', label: 'Observador', description: 'Solo consulta y sigue incidencias' },
]

const roleNames: Record<Exclude<EffectiveProjectRole, null>, string> = {
  admin: 'Administrador',
  lead: 'Líder',
  contributor: 'Colaborador',
  viewer: 'Observador',
}

export function ProjectPermissionsPage() {
  const { id = '' } = useParams()
  const members = useProjectsStore((s) => s.members)
  const status = useProjectsStore((s) => s.status)
  const fetchMembers = useProjectsStore((s) => s.fetchMembers)
  const user = useAuthStore((s) => s.user)

  useEffect(() => {
    void fetchMembers(id)
  }, [id, fetchMembers])

  // La tabla se muestra aunque la carga de miembros falle: la matriz no depende de los datos.
  const myRole = effectiveProjectRole(user?.role, user?.id, members as { userId: string; role: ProjectMemberRole }[])
  const roleCounts = members.reduce<Record<string, number>>((acc, member) => {
    acc[member.role] = (acc[member.role] ?? 0) + 1
    return acc
  }, {})

  const tableColumns: ColumnsType<PermissionDefinition> = [
    {
      title: 'Permiso',
      dataIndex: 'label',
      key: 'label',
      render: (_value, permission) => (
        <div>
          <Typography.Text strong>{permission.label}</Typography.Text>
          <Typography.Paragraph type="secondary" className={styles.description}>{permission.description}</Typography.Paragraph>
        </div>
      ),
    },
    ...columns.map((column) => ({
      title: (
        <span className={column.key === myRole ? styles.currentHeader : undefined}>
          {column.label}
          {column.key === myRole && <Tag color="blue" className={styles.youTag}>Tú</Tag>}
        </span>
      ),
      key: column.key ?? 'none',
      align: 'center' as const,
      width: 140,
      onCell: () => ({ className: column.key === myRole ? styles.currentColumn : undefined }),
      render: (_value: unknown, permission: PermissionDefinition) => {
        const granted = column.key !== null && hasProjectPermission(column.key, permission.key)
        return granted
          ? <CheckOutlined className={styles.granted} aria-label={`${column.label}: permitido`} />
          : <MinusOutlined className={styles.denied} aria-label={`${column.label}: no permitido`} />
      },
    })),
  ]

  return (
    <div className={styles.page}>
      <div className={styles.summary}>
        <Card className={styles.yourRole}>
          <Typography.Text type="secondary">Tu rol en este proyecto</Typography.Text>
          <Typography.Title level={4} className={styles.roleTitle}>
            {myRole ? roleNames[myRole] : 'Sin acceso como miembro'}
          </Typography.Title>
          <Typography.Text type="secondary">
            {myRole === 'admin' ? 'Tienes todos los permisos por ser administrador global.' : 'Los permisos que no tienes aparecen con un guion.'}
          </Typography.Text>
        </Card>
        <Card className={styles.counts} title="Miembros por rol">
          {status === 'loading' && members.length === 0 ? (
            <Skeleton active paragraph={{ rows: 2 }} />
          ) : (
            <div className={styles.countList}>
              {(['lead', 'contributor', 'viewer'] as ProjectMemberRole[]).map((role) => (
                <div key={role} className={styles.countItem}>
                  <Typography.Text>{roleNames[role]}</Typography.Text>
                  <Tag>{roleCounts[role] ?? 0}</Tag>
                </div>
              ))}
            </div>
          )}
        </Card>
      </div>

      {status === 'error' && (
        <Alert type="warning" showIcon message="No se pudieron cargar los miembros. La matriz de permisos sigue vigente." />
      )}

      <Card title="Matriz de permisos" className={styles.matrix}>
        <Table<PermissionDefinition>
          rowKey="key"
          columns={tableColumns}
          dataSource={projectPermissions}
          pagination={false}
          size="middle"
          scroll={{ x: 760 }}
        />
      </Card>
    </div>
  )
}
