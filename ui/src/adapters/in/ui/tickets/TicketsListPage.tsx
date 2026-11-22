import { PlusOutlined } from '@ant-design/icons'
import { Button, Input, Select, Typography } from 'antd'
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

  const { tickets, isLoading } = useTickets({ status: statusFilter, priority: priorityFilter })
  const [search, setSearch] = useState('')
  const updateRank = useTicketsStore((state) => state.updateRank)

  const filteredTickets = useMemo(() => {
   const term = search.trim().toLowerCase()

   if (!term) return tickets

   return tickets.filter((ticket) =>
     ticket.title.toLowerCase().includes(term)
   )
 }, [tickets, search])

  const handleSelect = (ticket: Ticket) => navigate(`/issues/${ticket.id}`)

  const orderedTickets = useMemo(
    () => [...filteredTickets].sort((a, b) => a.rank.localeCompare(b.rank)),
    [filteredTickets],
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
      </div>
      <Input.Search
         placeholder="Buscar incidencia por título"
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

      <Typography.Paragraph type="secondary" className={styles.hint}>
        <Link to="/issues/new">Crear la primera incidencia</Link> si la lista esta vacia.
      </Typography.Paragraph>
    </div>
  )
}
