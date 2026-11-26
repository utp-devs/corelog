import { create } from 'zustand'

import { TicketService } from '@/core/tickets/application/ticket.service'
import type {
  CreateTicketInput,
  ListTicketsFilter,
  UpdateIssueMetadataInput,
  UpdateIssueDetailsInput,
  IssueComment,
  IssueActivity,
  IssueLink,
  IssueLinkKind,
} from '@/core/tickets/application/ports/ticket-repository.port'
import type { Ticket, TicketStatus } from '@/core/tickets/domain/ticket'
import { TicketHttpRepository } from '@/adapters/out/http/ticket-http-repository'
import { extractErrorMessage } from '@/adapters/out/http/api-client'

const ticketService = new TicketService(new TicketHttpRepository())

type Status = 'idle' | 'loading' | 'error'

interface TicketsState {
  tickets: Ticket[]
  total: number
  selected: Ticket | null
  comments: IssueComment[]
  commentsIssueId: string | null
  activities: IssueActivity[]
  activitiesIssueId: string | null
  links: IssueLink[]
  linksIssueId: string | null
  status: Status
  error: string | null
  fetchTickets: (filter?: ListTicketsFilter) => Promise<void>
  fetchTicket: (id: string) => Promise<void>
  createTicket: (input: CreateTicketInput) => Promise<Ticket>
  updateStatus: (id: string, status: TicketStatus) => Promise<void>
  assign: (id: string, assigneeId: string | null) => Promise<void>
  unassign: (id: string) => Promise<void>
  updateMetadata: (id: string, input: UpdateIssueMetadataInput) => Promise<void>
  updateDetails: (id: string, input: UpdateIssueDetailsInput) => Promise<void>
  fetchComments: (id: string) => Promise<void>
  createComment: (id: string, body: string) => Promise<void>
  deleteComment: (issueId: string, commentId: string) => Promise<void>
  fetchActivities: (id: string) => Promise<void>
  fetchLinks: (id: string) => Promise<void>
  createLink: (id: string, targetIssueId: string, kind: IssueLinkKind) => Promise<void>
  deleteLink: (id: string, linkId: string) => Promise<void>
  updateRank: (id: string, rank: string) => Promise<void>
}

let commentsRequestId = 0
let activitiesRequestId = 0
let linksRequestId = 0
let ticketRequestId = 0

export const useTicketsStore = create<TicketsState>((set, get) => ({
  tickets: [],
  total: 0,
  selected: null,
  comments: [],
  commentsIssueId: null,
  activities: [],
  activitiesIssueId: null,
  links: [],
  linksIssueId: null,
  status: 'idle',
  error: null,

  fetchTickets: async (filter = {}) => {
    set({ status: 'loading', error: null })
    try {
      const page = await ticketService.list(filter)
      set({ tickets: page.items, total: page.total, status: 'idle' })
    } catch (error) {
      set({ status: 'error', error: extractErrorMessage(error, 'No se pudieron cargar las incidencias') })
    }
  },

  fetchTicket: async (id) => {
    const requestId = ++ticketRequestId
    set({ status: 'loading', error: null })
    try {
      const selected = await ticketService.getById(id)
      if (requestId === ticketRequestId) set({ selected, status: 'idle' })
    } catch (error) {
      if (requestId === ticketRequestId) set({ status: 'error', error: extractErrorMessage(error, 'No se pudo cargar la incidencia') })
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
    await assignWithRollback(set, get, id, assigneeId)
  },

  unassign: async (id) => {
    await assignWithRollback(set, get, id, null)
  },

  updateMetadata: async (id, input) => {
    const updated = await ticketService.updateMetadata(id, input)
    applyUpdate(set, get, updated)
  },

  updateDetails: async (id, input) => {
    const updated = await ticketService.updateDetails(id, input)
    applyUpdate(set, get, updated)
  },

  fetchComments: async (id) => {
    const requestId = ++commentsRequestId
    const comments = await ticketService.listComments(id)
    if (requestId === commentsRequestId) set({ comments, commentsIssueId: id })
  },

  createComment: async (id, body) => {
    const comment = await ticketService.createComment(id, body)
    const state = get()
    if (state.commentsIssueId === id) set({ comments: [...state.comments, comment] })
  },

  deleteComment: async (issueId, commentId) => {
    await ticketService.deleteComment(issueId, commentId)
    const state = get()
    if (state.commentsIssueId === issueId) set({ comments: state.comments.filter((comment) => comment.id !== commentId) })
  },

  fetchActivities: async (id) => {
    const requestId = ++activitiesRequestId
    const activities = await ticketService.listActivities(id)
    if (requestId === activitiesRequestId) set({ activities, activitiesIssueId: id })
  },

  fetchLinks: async (id) => {
    const requestId = ++linksRequestId
    const links = await ticketService.listLinks(id)
    if (requestId === linksRequestId) set({ links, linksIssueId: id })
  },

  createLink: async (id, targetIssueId, kind) => {
    const link = await ticketService.createLink(id, targetIssueId, kind)
    const state = get()
    if (state.linksIssueId === id) set({ links: [...state.links, link] })
  },

  deleteLink: async (id, linkId) => {
    await ticketService.deleteLink(id, linkId)
    const state = get()
    if (state.linksIssueId === id) set({ links: state.links.filter((link) => link.id !== linkId) })
  },

  updateRank: async (id, rank) => {
    const updated = await ticketService.updateRank(id, rank)
    applyUpdate(set, get, updated)
  },
}))

/**
 * Assigns (or clears, with null) optimistically: the issue shows the new assignee at once and the
 * previous state comes back if the API rejects the change. The error is rethrown for the caller to notify.
 */
async function assignWithRollback(
  set: (partial: Partial<TicketsState>) => void,
  get: () => TicketsState,
  id: string,
  assigneeId: string | null,
) {
  const previous = get().selected?.id === id
    ? get().selected
    : get().tickets.find((ticket) => ticket.id === id)
  if (previous) {
    // The name is unknown until the API answers, so it is cleared rather than shown stale.
    applyUpdate(set, get, { ...previous, assigneeId, assigneeName: null })
  }
  try {
    const updated = await ticketService.assign(id, assigneeId)
    applyUpdate(set, get, updated)
  } catch (error) {
    if (previous) applyUpdate(set, get, previous)
    throw error
  }
}

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
