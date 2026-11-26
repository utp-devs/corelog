import { renderHook } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'

import { RECENT_STORAGE_KEY, getRecent, pushRecent, useTrackRecent, type RecentItem } from './recent'

function item(id: string, kind: RecentItem['kind'] = 'issue'): RecentItem {
  return { kind, id, title: `Titulo ${id}`, subtitle: 'CORE', link: `/${kind}s/${id}` }
}

describe('recent items', () => {
  beforeEach(() => {
    window.localStorage.clear()
  })

  it('returns an empty list when nothing was stored or the JSON is broken', () => {
    expect(getRecent()).toEqual([])
    window.localStorage.setItem(RECENT_STORAGE_KEY, '{not json')
    expect(getRecent()).toEqual([])
  })

  it('stores the newest item first', () => {
    pushRecent(item('1'))
    pushRecent(item('2'))
    expect(getRecent().map((entry) => entry.id)).toEqual(['2', '1'])
  })

  it('dedupes by kind and id, moving the existing entry to the top', () => {
    pushRecent(item('1'))
    pushRecent(item('2'))
    pushRecent(item('1'))
    expect(getRecent().map((entry) => entry.id)).toEqual(['1', '2'])
  })

  it('keeps a project and an issue with the same id apart', () => {
    pushRecent(item('7', 'project'))
    pushRecent(item('7', 'issue'))
    expect(getRecent()).toHaveLength(2)
  })

  it('caps the list at the last 10 items', () => {
    for (let index = 1; index <= 12; index += 1) pushRecent(item(String(index)))
    const ids = getRecent().map((entry) => entry.id)
    expect(ids).toHaveLength(10)
    expect(ids[0]).toBe('12')
    expect(ids).not.toContain('1')
    expect(ids).not.toContain('2')
  })

  it('ignores stored entries with the wrong shape', () => {
    window.localStorage.setItem(RECENT_STORAGE_KEY, JSON.stringify([item('ok'), { kind: 'user', id: 'x' }]))
    expect(getRecent().map((entry) => entry.id)).toEqual(['ok'])
  })

  it('records the item from useTrackRecent and nothing while the item is missing', () => {
    const { rerender } = renderHook(({ value }: { value: RecentItem | null }) => useTrackRecent(value), {
      initialProps: { value: null as RecentItem | null },
    })
    expect(getRecent()).toEqual([])
    rerender({ value: item('5') })
    expect(getRecent().map((entry) => entry.id)).toEqual(['5'])
  })
})
