import {
  RECENT_LIMIT,
  RECENT_STORAGE_KEY,
  RecentItem,
  getRecent,
  pushRecent,
} from './recent-items';

function item(id: string, kind: RecentItem['kind'] = 'issue'): RecentItem {
  return { kind, id, title: `Titulo ${id}`, subtitle: 'CORE', link: `/${kind}s/${id}` };
}

describe('recent items', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it('returns an empty list when nothing was stored or the JSON is broken', () => {
    expect(getRecent()).toEqual([]);
    localStorage.setItem(RECENT_STORAGE_KEY, '{not json');
    expect(getRecent()).toEqual([]);
  });

  it('stores the newest item first', () => {
    pushRecent(item('1'));
    pushRecent(item('2'));
    expect(getRecent().map((entry) => entry.id)).toEqual(['2', '1']);
  });

  it('dedupes by kind and id, moving the existing entry to the top', () => {
    pushRecent(item('1'));
    pushRecent(item('2'));
    pushRecent(item('1'));
    expect(getRecent().map((entry) => entry.id)).toEqual(['1', '2']);
  });

  it('keeps a project and an issue with the same id apart', () => {
    pushRecent(item('7', 'project'));
    pushRecent(item('7', 'issue'));
    expect(getRecent()).toHaveLength(2);
  });

  it('caps the list at the last 10 items', () => {
    for (let index = 1; index <= 12; index += 1) pushRecent(item(String(index)));
    const ids = getRecent().map((entry) => entry.id);
    expect(ids).toHaveLength(RECENT_LIMIT);
    expect(ids[0]).toBe('12');
    expect(ids).not.toContain('1');
    expect(ids).not.toContain('2');
  });

  it('ignores stored entries with the wrong shape', () => {
    localStorage.setItem(
      RECENT_STORAGE_KEY,
      JSON.stringify([item('ok'), { kind: 'user', id: 'x' }]),
    );
    expect(getRecent().map((entry) => entry.id)).toEqual(['ok']);
  });

  it('stores only the known fields of the item', () => {
    pushRecent({ ...item('3'), extra: 'ignored' } as RecentItem);
    const [stored] = getRecent();
    expect(stored).toEqual(item('3'));
  });
});
