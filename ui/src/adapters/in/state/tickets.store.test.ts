import { beforeEach, describe, expect, it, vi } from 'vitest'

const { getById, listComments } = vi.hoisted(() => ({ getById: vi.fn(), listComments: vi.fn() }))

vi.mock('@/adapters/out/http/ticket-http-repository', () => ({
  TicketHttpRepository: class {
    getById = getById
    listComments = listComments
  },
}))

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
    useTicketsStore.setState({ selected: null, status: 'idle', error: null, comments: [], commentsIssueId: null })
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
})
