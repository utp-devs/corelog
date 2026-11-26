import { DownOutlined, UpOutlined } from '@ant-design/icons'
import dayjs from 'dayjs'
import type { ReactNode } from 'react'

import type { Ticket } from '@/core/tickets/domain/ticket'
import { DataTable, type DataTableColumn, type DataTablePagination } from '@/shared/ui/DataTable'
import { IssueKey } from '@/shared/ui/IssueKey'
import { Lozenge } from '@/shared/ui/Lozenge'
import { PriorityTag } from '@/shared/ui/PriorityTag'
import { UserChip } from '@/shared/ui/UserChip'
import { issueKeyOf, statusMeta } from './ticketLabels'
import styles from './TicketList.module.css'

interface TicketListProps {
  tickets: Ticket[]
  loading: boolean
  error?: string | null
  onRetry?: () => void
  onSelect: (ticket: Ticket) => void
  onMoveUp?: (ticket: Ticket) => void
  onMoveDown?: (ticket: Ticket) => void
  emptyState: ReactNode
  pagination?: DataTablePagination
  /** Table grows to the remaining height of its Section (PageContainer fill). */
  fill?: boolean
}

function Assignee({ ticket }: { ticket: Ticket }) {
  if (!ticket.assigneeId || !ticket.assigneeName) {
    return <span className={styles.muted}>Sin asignar</span>
  }
  return <UserChip user={{ id: ticket.assigneeId, name: ticket.assigneeName }} />
}

function StatusLozenge({ ticket }: { ticket: Ticket }) {
  const meta = statusMeta[ticket.status]
  return <Lozenge category={meta.category}>{meta.label}</Lozenge>
}

/** Lista de incidencias sobre DataTable: en escritorio tabla, por debajo de 768 px tarjetas. */
export function TicketList({ tickets, loading, error, onRetry, onSelect, onMoveUp, onMoveDown, emptyState, pagination, fill = false }: TicketListProps) {
  const columns: DataTableColumn<Ticket>[] = [
    {
      key: 'key',
      title: 'Clave',
      width: 150,
      render: (_, ticket) => (
        <IssueKey
          issueKey={issueKeyOf(ticket.id)}
          href={`/issues/${ticket.id}`}
          summary={ticket.title}
          type={ticket.issueType}
          resolved={ticket.status === 'resolved' || ticket.status === 'closed'}
        />
      ),
    },
    {
      key: 'summary',
      title: 'Resumen',
      render: (_, ticket) => <span className={styles.summary} title={ticket.title}>{ticket.title}</span>,
    },
    {
      key: 'status',
      title: 'Estado',
      width: 140,
      render: (_, ticket) => <StatusLozenge ticket={ticket} />,
    },
    {
      key: 'priority',
      title: 'Prioridad',
      width: 120,
      hideable: true,
      render: (_, ticket) => <PriorityTag priority={ticket.priority} />,
    },
    {
      key: 'assignee',
      title: 'Responsable',
      width: 200,
      hideable: true,
      render: (_, ticket) => <Assignee ticket={ticket} />,
    },
    {
      key: 'updated',
      title: 'Actualizado',
      width: 150,
      hideable: true,
      render: (_, ticket) => (
        <time dateTime={ticket.updatedAt} title={dayjs(ticket.updatedAt).format('DD/MM/YYYY HH:mm')}>
          {dayjs(ticket.updatedAt).format('DD/MM/YYYY HH:mm')}
        </time>
      ),
    },
  ]

  const rowActions = onMoveUp && onMoveDown
    ? (ticket: Ticket) => [
        { key: 'up', label: 'Mover arriba', icon: <UpOutlined />, onClick: () => onMoveUp(ticket) },
        { key: 'down', label: 'Mover abajo', icon: <DownOutlined />, onClick: () => onMoveDown(ticket) },
      ]
    : undefined

  return (
    <DataTable<Ticket>
      tableKey="issues"
      rowKey="id"
      columns={columns}
      data={tickets}
      loading={loading}
      error={error}
      onRetry={onRetry}
      emptyState={emptyState}
      pagination={pagination}
      fill={fill}
      onRowOpen={onSelect}
      rowActions={rowActions}
      mobileRender={(ticket) => (
        <div className={styles.card}>
          <span className={styles.cardTitle}>{ticket.title}</span>
          <div className={styles.cardMeta}>
            <StatusLozenge ticket={ticket} />
            <PriorityTag priority={ticket.priority} />
            <Assignee ticket={ticket} />
          </div>
        </div>
      )}
    />
  )
}
