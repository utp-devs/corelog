import { beforeEach, describe, expect, it, vi } from 'vitest'

const { getById, listComments, assign } = vi.hoisted(() => ({
  getById: vi.fn(),
  listComments: vi.fn(),
  assign: vi.fn(),
}))

vi.mock('@/adapters/out/http/ticket-http-repository', () => ({
  TicketHttpRepository: class {
    getById = getById
    listComments = listComments
    assign = assign
  },
}))

import type { Ticket } from '@/core/tickets/domain/ticket'
import { useTicketsStore } from './tickets.store'

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((done) => { resolve = done })
  return { promise, resolve }
}

describe('tickets store issue-scoped collaboration', () => {
  beforeEach(() => {
    listComments.mockReset()
    getById.mockReset()
    assign.mockReset()
    useTicketsStore.setState({ selected: null, tickets: [], status: 'idle', error: null, comments: [], commentsIssueId: null })
  })

  it('ignores an older comment response after switching issues', async () => {
    const oldResponse = deferred<{ id: string }[]>()
    const newResponse = deferred<{ id: string }[]>()
    listComments.mockReturnValueOnce(oldResponse.promise).mockReturnValueOnce(newResponse.promise)

    const oldRequest = useTicketsStore.getState().fetchComments('issue-old')
    const newRequest = useTicketsStore.getState().fetchComments('issue-new')
    newResponse.resolve([{ id: 'new-comment' }])
    await newRequest
    oldResponse.resolve([{ id: 'old-comment' }])
    await oldRequest

    expect(useTicketsStore.getState().commentsIssueId).toBe('issue-new')
    expect(useTicketsStore.getState().comments.map((comment) => comment.id)).toEqual(['new-comment'])
  })

  it('does not replace the selected issue with a slower previous request', async () => {
    const oldResponse = deferred<{ id: string; title: string }>()
    const newResponse = deferred<{ id: string; title: string }>()
    getById.mockReturnValueOnce(oldResponse.promise).mockReturnValueOnce(newResponse.promise)

    const oldRequest = useTicketsStore.getState().fetchTicket('issue-old')
    const newRequest = useTicketsStore.getState().fetchTicket('issue-new')
    newResponse.resolve({ id: 'issue-new', title: 'new' })
    await newRequest
    oldResponse.resolve({ id: 'issue-old', title: 'old' })
    await oldRequest

    expect(useTicketsStore.getState().selected?.id).toBe('issue-new')
    expect(useTicketsStore.getState().status).toBe('idle')
  })

  describe('assign and unassign', () => {
    const issue = {
      id: 'issue-1', title: 'Error', assigneeId: 'user-2', assigneeName: 'Luis',
    } as unknown as Ticket

    beforeEach(() => {
      useTicketsStore.setState({ tickets: [issue], selected: issue })
    })

    it('clears the assignee optimistically and stores the server answer', async () => {
      const response = deferred<Ticket>()
      assign.mockReturnValueOnce(response.promise)

      const request = useTicketsStore.getState().unassign('issue-1')
      expect(useTicketsStore.getState().selected).toMatchObject({ assigneeId: null, assigneeName: null })
      expect(useTicketsStore.getState().tickets[0]).toMatchObject({ assigneeId: null })

      response.resolve({ ...issue, assigneeId: null, assigneeName: null })
      await request
      expect(assign).toHaveBeenCalledWith('issue-1', null)
      expect(useTicketsStore.getState().selected?.assigneeId).toBeNull()
    })

    it('rolls back the previous assignee when clearing fails and rethrows the error', async () => {
      assign.mockRejectedValueOnce(new Error('network down'))

      await expect(useTicketsStore.getState().unassign('issue-1')).rejects.toThrow('network down')

      expect(useTicketsStore.getState().selected).toMatchObject({ assigneeId: 'user-2', assigneeName: 'Luis' })
      expect(useTicketsStore.getState().tickets[0]).toMatchObject({ assigneeId: 'user-2' })
    })

    it('shows the new assignee optimistically and rolls back when assigning fails', async () => {
      assign.mockRejectedValueOnce(new Error('forbidden'))

      await expect(useTicketsStore.getState().assign('issue-1', 'user-3')).rejects.toThrow('forbidden')

      expect(assign).toHaveBeenCalledWith('issue-1', 'user-3')
      expect(useTicketsStore.getState().selected).toMatchObject({ assigneeId: 'user-2', assigneeName: 'Luis' })
    })

    it('stores the server answer when assigning succeeds', async () => {
      assign.mockResolvedValueOnce({ ...issue, assigneeId: 'user-3', assigneeName: 'Marta' })

      await useTicketsStore.getState().assign('issue-1', 'user-3')

      expect(useTicketsStore.getState().selected).toMatchObject({ assigneeId: 'user-3', assigneeName: 'Marta' })
    })
  })
})
