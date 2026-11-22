import { DeleteOutlined, EditOutlined } from '@ant-design/icons'
import { Button, Space, Table } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import type { MouseEvent } from 'react'

import type { Team } from '@/core/teams/domain/team'
import styles from './TeamList.module.css'

interface TeamListProps {
  teams: Team[]
  isLoading: boolean
  selectedTeamId: string | null
  onSelect: (team: Team) => void
  onRename: (team: Team) => void
  onDelete: (team: Team) => void
}

export function TeamList({ teams, isLoading, selectedTeamId, onSelect, onRename, onDelete }: TeamListProps) {
  const stop = (event: MouseEvent) => event.stopPropagation()

  const columns: ColumnsType<Team> = [
    { title: 'Nombre', dataIndex: 'name', key: 'name' },
    {
      title: 'Creado',
      dataIndex: 'createdAt',
      key: 'createdAt',
      render: (value: string) => new Date(value).toLocaleString(),
    },
    {
      title: 'Acciones',
      key: 'actions',
      render: (_, team) => (
        <Space onClick={stop}>
          <Button icon={<EditOutlined />} onClick={() => onRename(team)}>
            Renombrar
          </Button>
          <Button danger icon={<DeleteOutlined />} onClick={() => onDelete(team)}>
            Eliminar
          </Button>
        </Space>
      ),
    },
  ]

  return (
    <Table<Team>
      rowKey="id"
      loading={isLoading}
      dataSource={teams}
      columns={columns}
      onRow={(team) => ({ onClick: () => onSelect(team) })}
      rowClassName={(team) => (team.id === selectedTeamId ? styles.selectedRow : styles.row)}
    />
  )
}
