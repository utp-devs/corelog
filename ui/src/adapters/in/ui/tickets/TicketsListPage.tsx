import { PlusOutlined } from '@ant-design/icons'
import { Button, DatePicker, Input, Pagination, Select, Typography } from 'antd'
import type { IssueType } from '@/core/tickets/domain/ticket'
import { useMemo, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'

import type { Ticket, TicketPriority, TicketStatus } from '@/core/tickets/domain/ticket'
import { useTickets } from './hooks/useTickets'
import { useTicketsStore } from '@/adapters/in/state/tickets.store'
import { TicketList } from './TicketList'
import { PageHeader } from '@/shared/ui/PageHeader'
import styles from './TicketsListPage.module.css'

const statusOptions: { value: TicketStatus; label: string }[] = [
  { value: 'open', label: 'Abierto' },
  { value: 'in_progress', label: 'En progreso' },
  { value: 'resolved', label: 'Resuelto' },
  { value: 'closed', label: 'Cerrado' },
]

const priorityOptions: { value: TicketPriority; label: string }[] = [
  { value: 'low', label: 'Baja' },
  { value: 'medium', label: 'Media' },
  { value: 'high', label: 'Alta' },
  { value: 'critical', label: 'Critica' },
]

export function TicketsListPage() {
  const navigate = useNavigate()
  const [statusFilter, setStatusFilter] = useState<TicketStatus | undefined>()
  const [priorityFilter, setPriorityFilter] = useState<TicketPriority | undefined>()
  const [issueTypeFilter, setIssueTypeFilter] = useState<IssueType | undefined>()
  const [dueFrom, setDueFrom] = useState<string | undefined>()
  const [dueTo, setDueTo] = useState<string | undefined>()
  const [page, setPage] = useState(1)
  const [search, setSearch] = useState('')

  const { tickets, isLoading } = useTickets({
    status: statusFilter,
    priority: priorityFilter,
    issueType: issueTypeFilter,
    query: search.trim() || undefined,
    dueFrom,
    dueTo,
    page,
    pageSize: 20,
  })
  const updateRank = useTicketsStore((state) => state.updateRank)

  const handleSelect = (ticket: Ticket) => navigate(`/issues/${ticket.id}`)

  const orderedTickets = useMemo(
    () => [...tickets].sort((a, b) => a.rank.localeCompare(b.rank)),
    [tickets],
  )

  const handleMove = async (ticket: Ticket, direction: -1 | 1) => {
    const currentIndex = orderedTickets.findIndex((item) => item.id === ticket.id)
    const targetIndex = currentIndex + direction
    if (currentIndex < 0 || targetIndex < 0 || targetIndex >= orderedTickets.length) return

    await Promise.all(
      orderedTickets.map((item, index) =>
        updateRank(item.id, index.toString().padStart(8, '0')),
      ),
    )

    const currentRank = currentIndex.toString().padStart(8, '0')
    const targetRank = targetIndex.toString().padStart(8, '0')

    await Promise.all([
      updateRank(ticket.id, targetRank),
      updateRank(orderedTickets[targetIndex].id, currentRank),
    ])
  }

  return (
    <div>
      <PageHeader
        title="Incidencias"
        description="Consulta y administra los incidentes registrados."
        actions={
          <Button type="primary" icon={<PlusOutlined />} onClick={() => navigate('/issues/new')}>
            Nueva incidencia
          </Button>
        }
      />

      <div className={styles.filters}>
        <Select
          allowClear
          placeholder="Filtrar por estado"
          className={styles.filterSelect}
          options={statusOptions}
          value={statusFilter}
          onChange={setStatusFilter}
        />
        <Select
          allowClear
          placeholder="Filtrar por prioridad"
          className={styles.filterSelect}
          options={priorityOptions}
          value={priorityFilter}
          onChange={setPriorityFilter}
        />
        <Select
          allowClear
          placeholder="Filtrar por tipo"
          className={styles.filterSelect}
          options={[
            { value: 'task', label: 'Tarea' },
            { value: 'bug', label: 'Error' },
            { value: 'story', label: 'Historia' },
            { value: 'epic', label: 'Epic' },
            { value: 'subtask', label: 'Subtarea' },
          ]}
          value={issueTypeFilter}
          onChange={setIssueTypeFilter}
        />
        <DatePicker
          placeholder="Fecha desde"
          onChange={(value) => setDueFrom(value?.startOf('day').toISOString())}
        />
        <DatePicker
          placeholder="Fecha hasta"
          onChange={(value) => setDueTo(value?.endOf('day').toISOString())}
        />
      </div>
      <Input.Search
         placeholder="Buscar incidencia por título o descripción"
         allowClear
         value={search}
         onChange={(e) => setSearch(e.target.value)}
      />

      <TicketList
         tickets={orderedTickets}
         isLoading={isLoading}
         onSelect={handleSelect}
         onMoveUp={(ticket) => void handleMove(ticket, -1)}
         onMoveDown={(ticket) => void handleMove(ticket, 1)}
         emptyText={search.trim() ? `Ninguna incidencia coincide con "${search.trim()}"` : undefined}
      />

      {tickets.length >= 20 && (
        <Pagination
          current={page}
          pageSize={20}
          total={tickets.length + (page - 1) * 20 + 20}
          onChange={setPage}
          style={{ marginTop: 16 }}
        />
      )}

      <Typography.Paragraph type="secondary" className={styles.hint}>
        <Link to="/issues/new">Crear la primera incidencia</Link> si la lista esta vacia.
      </Typography.Paragraph>
    </div>
  )
}
