import { describe, expect, it } from 'vitest'

import { authPathWithRedirect, safeRedirect } from './safe-redirect'

describe('safeRedirect', () => {
  it('keeps in-app paths, query and hash', () => {
    expect(safeRedirect('/invite/abc')).toBe('/invite/abc')
    expect(safeRedirect('/projects?x=1#top')).toBe('/projects?x=1#top')
  })

  it('falls back to /issues for missing or external targets', () => {
    expect(safeRedirect(null)).toBe('/issues')
    expect(safeRedirect('')).toBe('/issues')
    expect(safeRedirect('https://evil.example')).toBe('/issues')
    expect(safeRedirect('//evil.example')).toBe('/issues')
    expect(safeRedirect('/\\evil.example')).toBe('/issues')
    expect(safeRedirect('projects')).toBe('/issues')
  })
})

describe('authPathWithRedirect', () => {
  it('encodes the target into the redirect param', () => {
    expect(authPathWithRedirect('/login', '/invite/tok?a=1')).toBe('/login?redirect=%2Finvite%2Ftok%3Fa%3D1')
  })

  it('returns the bare path without a target', () => {
    expect(authPathWithRedirect('/register', null)).toBe('/register')
  })
})
