import { Button, Space, Table } from 'antd'
import { DownOutlined, UpOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'

import type { Ticket, TicketPriority, TicketStatus } from '@/core/tickets/domain/ticket'
import { TicketBadge } from './TicketBadge'
import styles from './TicketList.module.css'

interface TicketListProps {
  tickets: Ticket[]
  isLoading: boolean
  onSelect: (ticket: Ticket) => void
  onMoveUp?: (ticket: Ticket) => void
  onMoveDown?: (ticket: Ticket) => void
  emptyText?: string
}

export function TicketList({ tickets, isLoading, onSelect, onMoveUp, onMoveDown, emptyText = 'Aun no hay incidencias' }: TicketListProps) {
  const columns: ColumnsType<Ticket> = [
    { title: 'Titulo', dataIndex: 'title', key: 'title' },
    {
      title: 'Estado',
      dataIndex: 'status',
      key: 'status',
      render: (value: TicketStatus) => <TicketBadge kind="status" value={value} />,
    },
    {
      title: 'Prioridad',
      dataIndex: 'priority',
      key: 'priority',
      render: (value: TicketPriority) => <TicketBadge kind="priority" value={value} />,
    },
    {
      title: 'Creado',
      dataIndex: 'createdAt',
      key: 'createdAt',
      render: (value: string) => new Date(value).toLocaleString(),
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
      locale={{ emptyText: <div className={styles.empty}>{emptyText}</div> }}
    />
  )
}
