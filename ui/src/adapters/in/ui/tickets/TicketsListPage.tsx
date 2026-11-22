import { PlusOutlined } from '@ant-design/icons'
import { Alert, Button, DatePicker, Input, Modal, Pagination, Select, Typography, message as antdMessage } from 'antd'
import type { IssueType } from '@/core/tickets/domain/ticket'
import dayjs from 'dayjs'
import { useEffect, useMemo, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'

import type { Ticket, TicketPriority, TicketStatus } from '@/core/tickets/domain/ticket'
import type { SavedFilter, SavedFilterCriteria } from '@/core/tickets/domain/saved-filter'
import { SavedFilterHttpRepository } from '@/adapters/out/http/saved-filter-http-repository'
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

const savedFilterRepository = new SavedFilterHttpRepository()

export function TicketsListPage() {
  const navigate = useNavigate()
  const [statusFilter, setStatusFilter] = useState<TicketStatus | undefined>()
  const [priorityFilter, setPriorityFilter] = useState<TicketPriority | undefined>()
  const [issueTypeFilter, setIssueTypeFilter] = useState<IssueType | undefined>()
  const [dueFrom, setDueFrom] = useState<string | undefined>()
  const [dueTo, setDueTo] = useState<string | undefined>()
  const [page, setPage] = useState(1)
  const [search, setSearch] = useState('')
  const [savedFilters, setSavedFilters] = useState<SavedFilter[]>([])
  const [selectedSavedFilter, setSelectedSavedFilter] = useState<string>()
  const [savedFilterName, setSavedFilterName] = useState('')
  const [saveModalOpen, setSaveModalOpen] = useState(false)
  const [savedFilterError, setSavedFilterError] = useState<string>()
  const [messageApi, contextHolder] = antdMessage.useMessage()

  useEffect(() => {
    let active = true
    void savedFilterRepository.list().then((filters) => {
      if (active) setSavedFilters(filters)
    }).catch(() => {
      if (active) setSavedFilterError('No se pudieron cargar los filtros guardados.')
    })
    return () => { active = false }
  }, [])

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

  const currentCriteria: SavedFilterCriteria = {
    status: statusFilter,
    priority: priorityFilter,
    issueType: issueTypeFilter,
    query: search.trim() || undefined,
    dueFrom,
    dueTo,
  }

  const applySavedFilter = (filterID?: string) => {
    setSelectedSavedFilter(filterID)
    const criteria = savedFilters.find((filter) => filter.id === filterID)?.criteria
    setStatusFilter(criteria?.status)
    setPriorityFilter(criteria?.priority)
    setIssueTypeFilter(criteria?.issueType)
    setSearch(criteria?.query ?? '')
    setDueFrom(criteria?.dueFrom)
    setDueTo(criteria?.dueTo)
    setPage(1)
  }

  const saveCurrentFilter = async () => {
    try {
      const filter = await savedFilterRepository.create(savedFilterName, currentCriteria)
      setSavedFilters((current) => [...current, filter].sort((a, b) => a.name.localeCompare(b.name)))
      setSelectedSavedFilter(filter.id)
      setSavedFilterName('')
      setSaveModalOpen(false)
      setSavedFilterError(undefined)
      messageApi.success('Filtro guardado')
    } catch {
      setSavedFilterError('No se pudo guardar el filtro. Comprueba que el nombre no esté repetido.')
    }
  }

  const deleteSelectedFilter = async () => {
    const filter = savedFilters.find((item) => item.id === selectedSavedFilter)
    if (!filter) return
    try {
      await savedFilterRepository.delete(filter.id)
      setSavedFilters((current) => current.filter((item) => item.id !== filter.id))
      setSelectedSavedFilter(undefined)
      setSavedFilterError(undefined)
      messageApi.success('Filtro eliminado')
    } catch {
      setSavedFilterError('No se pudo eliminar el filtro guardado.')
    }
  }

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
      {contextHolder}
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
          placeholder="Cargar filtro guardado"
          className={styles.filterSelect}
          value={selectedSavedFilter}
          options={savedFilters.map((filter) => ({ value: filter.id, label: filter.name }))}
          onChange={applySavedFilter}
        />
        <Button onClick={() => setSaveModalOpen(true)}>Guardar filtro actual</Button>
        <Button danger disabled={!selectedSavedFilter} onClick={() => void deleteSelectedFilter()}>Eliminar filtro</Button>
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
          value={dueFrom ? dayjs(dueFrom) : null}
          onChange={(value) => setDueFrom(value?.startOf('day').toISOString())}
        />
        <DatePicker
          placeholder="Fecha hasta"
          value={dueTo ? dayjs(dueTo) : null}
          onChange={(value) => setDueTo(value?.endOf('day').toISOString())}
        />
      </div>
      {savedFilterError && <Alert type="error" showIcon message={savedFilterError} closable onClose={() => setSavedFilterError(undefined)} />}
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
      <Modal
        title="Guardar filtro"
        open={saveModalOpen}
        okText="Guardar"
        cancelText="Cancelar"
        onCancel={() => setSaveModalOpen(false)}
        onOk={() => void saveCurrentFilter()}
        okButtonProps={{ disabled: !savedFilterName.trim() || savedFilterName.length > 80 }}
      >
        <Input
          autoFocus
          maxLength={80}
          value={savedFilterName}
          onChange={(event) => setSavedFilterName(event.target.value)}
          onPressEnter={() => { if (savedFilterName.trim()) void saveCurrentFilter() }}
          placeholder="Nombre del filtro"
        />
      </Modal>
    </div>
  )
}
