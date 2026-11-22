import { create } from 'zustand'

import { TicketService } from '@/core/tickets/application/ticket.service'
import type {
  CreateTicketInput,
  ListTicketsFilter,
  UpdateIssueMetadataInput,
  IssueComment,
  IssueActivity,
} from '@/core/tickets/application/ports/ticket-repository.port'
import type { Ticket, TicketStatus } from '@/core/tickets/domain/ticket'
import { TicketHttpRepository } from '@/adapters/out/http/ticket-http-repository'
import { extractErrorMessage } from '@/adapters/out/http/api-client'

const ticketService = new TicketService(new TicketHttpRepository())

type Status = 'idle' | 'loading' | 'error'

interface TicketsState {
  tickets: Ticket[]
  selected: Ticket | null
  comments: IssueComment[]
  activities: IssueActivity[]
  status: Status
  error: string | null
  fetchTickets: (filter?: ListTicketsFilter) => Promise<void>
  fetchTicket: (id: string) => Promise<void>
  createTicket: (input: CreateTicketInput) => Promise<Ticket>
  updateStatus: (id: string, status: TicketStatus) => Promise<void>
  assign: (id: string, assigneeId: string) => Promise<void>
  updateMetadata: (id: string, input: UpdateIssueMetadataInput) => Promise<void>
  fetchComments: (id: string) => Promise<void>
  createComment: (id: string, body: string) => Promise<void>
  deleteComment: (issueId: string, commentId: string) => Promise<void>
  fetchActivities: (id: string) => Promise<void>
}

export const useTicketsStore = create<TicketsState>((set, get) => ({
  tickets: [],
  selected: null,
  comments: [],
  activities: [],
  status: 'idle',
  error: null,

  fetchTickets: async (filter = {}) => {
    set({ status: 'loading', error: null })
    try {
      const tickets = await ticketService.list(filter)
      set({ tickets, status: 'idle' })
    } catch (error) {
      set({ status: 'error', error: extractErrorMessage(error, 'No se pudieron cargar las incidencias') })
    }
  },

  fetchTicket: async (id) => {
    set({ status: 'loading', error: null })
    try {
      const selected = await ticketService.getById(id)
      set({ selected, status: 'idle' })
    } catch (error) {
      set({ status: 'error', error: extractErrorMessage(error, 'No se pudo cargar la incidencia') })
    }
  },

  createTicket: async (input) => {
    const ticket = await ticketService.create(input)
    set({ tickets: [ticket, ...get().tickets] })
    return ticket
  },

  updateStatus: async (id, status) => {
    const updated = await ticketService.updateStatus(id, status)
    applyUpdate(set, get, updated)
  },

  assign: async (id, assigneeId) => {
    const updated = await ticketService.assign(id, assigneeId)
    applyUpdate(set, get, updated)
  },

  updateMetadata: async (id, input) => {
    const updated = await ticketService.updateMetadata(id, input)
    applyUpdate(set, get, updated)
  },

  fetchComments: async (id) => {
    set({ comments: await ticketService.listComments(id) })
  },

  createComment: async (id, body) => {
    const comment = await ticketService.createComment(id, body)
    set({ comments: [...get().comments, comment] })
  },

  deleteComment: async (issueId, commentId) => {
    await ticketService.deleteComment(issueId, commentId)
    set({ comments: get().comments.filter((comment) => comment.id !== commentId) })
  },

  fetchActivities: async (id) => {
    set({ activities: await ticketService.listActivities(id) })
  },
}))

function applyUpdate(
  set: (partial: Partial<TicketsState>) => void,
  get: () => TicketsState,
  updated: Ticket,
) {
  set({
    tickets: get().tickets.map((t) => (t.id === updated.id ? updated : t)),
    selected: get().selected?.id === updated.id ? updated : get().selected,
  })
}
