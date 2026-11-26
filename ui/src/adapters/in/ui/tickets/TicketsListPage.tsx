import { PlusOutlined } from '@ant-design/icons'
import { Alert, Button, DatePicker, Input, Modal, Select, message as antdMessage } from 'antd'
import dayjs from 'dayjs'
import { useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'

import { SavedFilterHttpRepository } from '@/adapters/out/http/saved-filter-http-repository'
import { useTicketsStore } from '@/adapters/in/state/tickets.store'
import type { IssueType, Ticket, TicketPriority, TicketStatus } from '@/core/tickets/domain/ticket'
import type { SavedFilter, SavedFilterCriteria } from '@/core/tickets/domain/saved-filter'
import { EmptyState } from '@/shared/ui/EmptyState'
import { FilterBar } from '@/shared/ui/FilterBar'
import { PageContainer } from '@/shared/ui/PageContainer'
import { PageHeader } from '@/shared/ui/PageHeader'
import { Section } from '@/shared/ui/Section'
import { Toolbar } from '@/shared/ui/Toolbar'
import { useTickets } from './hooks/useTickets'
import { TicketList } from './TicketList'
import styles from './TicketsListPage.module.css'

const DEFAULT_PAGE_SIZE = 20

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
  { value: 'critical', label: 'Crítica' },
]

const issueTypeOptions: { value: IssueType; label: string }[] = [
  { value: 'task', label: 'Tarea' },
  { value: 'bug', label: 'Error' },
  { value: 'story', label: 'Historia' },
  { value: 'epic', label: 'Épica' },
  { value: 'subtask', label: 'Subtarea' },
]

const savedFilterRepository = new SavedFilterHttpRepository()

export function TicketsListPage() {
  const [advancedOpen, setAdvancedOpen] = useState(false)
  const navigate = useNavigate()
  const [statusFilter, setStatusFilter] = useState<TicketStatus | undefined>()
  const [priorityFilter, setPriorityFilter] = useState<TicketPriority | undefined>()
  const [issueTypeFilter, setIssueTypeFilter] = useState<IssueType | undefined>()
  const [dueFrom, setDueFrom] = useState<string | undefined>()
  const [dueTo, setDueTo] = useState<string | undefined>()
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(DEFAULT_PAGE_SIZE)
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

  const { tickets, total, isLoading, error, refetch } = useTickets({
    status: statusFilter,
    priority: priorityFilter,
    issueType: issueTypeFilter,
    query: search.trim() || undefined,
    dueFrom,
    dueTo,
    page,
    pageSize,
  })
  const updateRank = useTicketsStore((state) => state.updateRank)

  const hasActiveFilters = Boolean(
    search.trim() || statusFilter || priorityFilter || issueTypeFilter || dueFrom || dueTo,
  )

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

  const clearFilters = () => {
    setSearch('')
    setStatusFilter(undefined)
    setPriorityFilter(undefined)
    setIssueTypeFilter(undefined)
    setDueFrom(undefined)
    setDueTo(undefined)
    setSelectedSavedFilter(undefined)
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

  // Los filtros avanzados (filtro guardado y fechas) se muestran plegados; se abren solos si hay alguno activo.
  const advancedVisible = advancedOpen || Boolean(dueFrom || dueTo || selectedSavedFilter)
  const emptyState = hasActiveFilters ? (
    <EmptyState
      title="Ninguna incidencia coincide con los filtros"
      description="Cambia o limpia los filtros para ver más resultados."
      action={<Button onClick={clearFilters}>Limpiar filtros</Button>}
    />
  ) : (
    <EmptyState
      title="Aún no hay incidencias"
      description="Crea la primera para empezar."
      action={<Button type="primary" icon={<PlusOutlined />} onClick={() => navigate('/issues/new')}>Nueva incidencia</Button>}
    />
  )

  return (
    <PageContainer variant="wide" pageName="issues" ready={!isLoading}>
      {contextHolder}
      <PageHeader
        title="Incidencias"
        description="Consulta y administra los incidentes registrados."
      />

      <Toolbar
        left={
          <FilterBar
            search={{ value: search, onChange: (value) => { setSearch(value); setPage(1) }, placeholder: 'Buscar incidencia por título o descripción' }}
            filters={[
              { key: 'status', label: 'Filtrar por estado', options: statusOptions, value: statusFilter, onChange: (value) => { setStatusFilter(value as TicketStatus | undefined); setPage(1) } },
              { key: 'priority', label: 'Filtrar por prioridad', options: priorityOptions, value: priorityFilter, onChange: (value) => { setPriorityFilter(value as TicketPriority | undefined); setPage(1) } },
              { key: 'issueType', label: 'Filtrar por tipo', options: issueTypeOptions, value: issueTypeFilter, onChange: (value) => { setIssueTypeFilter(value as IssueType | undefined); setPage(1) } },
            ]}
            onClear={hasActiveFilters ? clearFilters : undefined}
          />
        }
        right={
          <Button type="primary" icon={<PlusOutlined />} onClick={() => navigate('/issues/new')}>
            Nueva incidencia
          </Button>
        }
        secondary={
          <Button aria-expanded={advancedVisible} onClick={() => setAdvancedOpen((value) => !value)}>
            {advancedVisible ? 'Ocultar filtros avanzados' : 'Filtros avanzados'}
          </Button>
        }
      />

      {advancedVisible && (
        <div className={styles.savedRow}>
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
          <DatePicker
            placeholder="Fecha desde"
            value={dueFrom ? dayjs(dueFrom) : null}
            onChange={(value) => { setDueFrom(value?.startOf('day').toISOString()); setPage(1) }}
          />
          <DatePicker
            placeholder="Fecha hasta"
            value={dueTo ? dayjs(dueTo) : null}
            onChange={(value) => { setDueTo(value?.endOf('day').toISOString()); setPage(1) }}
          />
        </div>
      )}

      {savedFilterError && <Alert type="error" showIcon message={savedFilterError} closable onClose={() => setSavedFilterError(undefined)} />}

      <Section flush ariaLabel="Lista de incidencias">
        <TicketList
          tickets={orderedTickets}
          loading={isLoading}
          error={error}
          onRetry={() => void refetch()}
          onSelect={handleSelect}
          onMoveUp={(ticket) => void handleMove(ticket, -1)}
          onMoveDown={(ticket) => void handleMove(ticket, 1)}
          emptyState={emptyState}
          pagination={total > 0 ? {
            page,
            pageSize,
            total,
            onChange: (nextPage, nextSize) => {
              setPage(nextPage)
              setPageSize(nextSize)
            },
          } : undefined}
        />
      </Section>

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
    </PageContainer>
  )
}
