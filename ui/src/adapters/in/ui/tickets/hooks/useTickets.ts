import { useEffect } from 'react'

import type { IssueType, TicketPriority, TicketStatus } from '@/core/tickets/domain/ticket'
import { useTicketsStore } from '@/adapters/in/state/tickets.store'

interface UseTicketsFilter {
  status?: TicketStatus
  priority?: TicketPriority
  query?: string
  issueType?: IssueType
  dueFrom?: string
  dueTo?: string
  page?: number
  pageSize?: number
}

export function useTickets(filter: UseTicketsFilter) {
  const {
    status: statusFilter,
    priority: priorityFilter,
    query,
    issueType,
    dueFrom,
    dueTo,
    page,
    pageSize,
  } = filter
  const tickets = useTicketsStore((s) => s.tickets)
  const status = useTicketsStore((s) => s.status)
  const error = useTicketsStore((s) => s.error)
  const fetchTickets = useTicketsStore((s) => s.fetchTickets)

  useEffect(() => {
    fetchTickets({
      status: statusFilter,
      priority: priorityFilter,
      query,
      issueType,
      dueFrom,
      dueTo,
      page,
      pageSize,
    })
  }, [fetchTickets, statusFilter, priorityFilter, query, issueType, dueFrom, dueTo, page, pageSize])

  return { tickets, isLoading: status === 'loading', error }
}
