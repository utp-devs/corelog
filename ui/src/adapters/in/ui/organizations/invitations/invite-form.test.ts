import { describe, expect, it } from 'vitest'

import { inviteItemsOf, MAX_INVITATIONS_PER_REQUEST, parseInviteEmails } from './invite-form'

describe('parseInviteEmails', () => {
  it('splits by commas, semicolons, spaces and new lines and keeps the typed order', () => {
    const parsed = parseInviteEmails('ana@acme.test,luis@acme.test;marta@acme.test\ncarla@acme.test')
    expect(parsed.emails).toEqual(['ana@acme.test', 'luis@acme.test', 'marta@acme.test', 'carla@acme.test'])
    expect(parsed.invalid).toEqual([])
  })

  it('reports invalid entries and removes case-insensitive duplicates', () => {
    const parsed = parseInviteEmails('Ana@acme.test, ana@ACME.test, nope, @x.es')
    expect(parsed.emails).toEqual(['Ana@acme.test'])
    expect(parsed.duplicates).toEqual(['ana@ACME.test'])
    expect(parsed.invalid).toEqual(['nope', '@x.es'])
  })

  it('returns nothing for blank input', () => {
    expect(parseInviteEmails('  \n ,  ')).toEqual({ emails: [], invalid: [], duplicates: [] })
  })
})

describe('inviteItemsOf', () => {
  it('builds one item per valid email with the chosen role', () => {
    expect(inviteItemsOf({ emails: 'a@acme.test, b@acme.test', role: 'guest' })).toEqual([
      { email: 'a@acme.test', role: 'guest' },
      { email: 'b@acme.test', role: 'guest' },
    ])
  })

  it('keeps the API limit visible in the constant', () => {
    expect(MAX_INVITATIONS_PER_REQUEST).toBe(50)
  })
})
