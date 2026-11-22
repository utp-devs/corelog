import { Button, Space, Table } from 'antd'
import { DownOutlined, UpOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import dayjs from 'dayjs'

import type { Ticket } from '@/core/tickets/domain/ticket'
import { EmptyState } from '@/shared/ui/EmptyState'
import { IssueTypeIcon } from '@/shared/ui/IssueTypeIcon'
import { PriorityTag } from '@/shared/ui/PriorityTag'
import { StatusTag } from '@/shared/ui/StatusTag'
import styles from './TicketList.module.css'

interface TicketListProps {
  tickets: Ticket[]
  isLoading: boolean
  onSelect: (ticket: Ticket) => void
  onMoveUp?: (ticket: Ticket) => void
  onMoveDown?: (ticket: Ticket) => void
  emptyText?: string
}

export function TicketList({ tickets, isLoading, onSelect, onMoveUp, onMoveDown, emptyText = 'Aún no hay incidencias' }: TicketListProps) {
  const columns: ColumnsType<Ticket> = [
    {
      title: 'Tipo',
      key: 'issueType',
      width: 72,
      render: (_, ticket) => <IssueTypeIcon type={ticket.issueType} />,
    },
    { title: 'Título', dataIndex: 'title', key: 'title' },
    {
      title: 'Estado',
      key: 'status',
      render: (_, ticket) => <StatusTag status={ticket.status} />,
    },
    {
      title: 'Prioridad',
      key: 'priority',
      render: (_, ticket) => <PriorityTag priority={ticket.priority} />,
    },
    {
      title: 'Creado',
      dataIndex: 'createdAt',
      key: 'createdAt',
      render: (value: string) => dayjs(value).format('DD/MM/YYYY HH:mm'),
    },
    {
      title: 'Orden',
      key: 'rank',
      render: (_, ticket) => (
        <Space>
          <Button size="small" icon={<UpOutlined />} onClick={(event) => { event.stopPropagation(); onMoveUp?.(ticket) }} aria-label="Mover arriba" />
          <Button size="small" icon={<DownOutlined />} onClick={(event) => { event.stopPropagation(); onMoveDown?.(ticket) }} aria-label="Mover abajo" />
        </Space>
      ),
    },
  ]

  return (
    <Table<Ticket>
      rowKey="id"
      loading={isLoading}
      dataSource={tickets}
      columns={columns}
      onRow={(record) => ({ onClick: () => onSelect(record) })}
      rowClassName={() => styles.row}
      pagination={false}
      locale={{ emptyText: <EmptyState title={emptyText} /> }}
    />
  )
}
