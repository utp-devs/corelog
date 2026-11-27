import { toMentionUsers } from './mentions';

describe('toMentionUsers', () => {
  it('keeps only id, name and email of each user', () => {
    const users = toMentionUsers([{ id: 'user-1', name: 'Ana', email: 'ana@example.test' }]);
    expect(users).toEqual([{ id: 'user-1', name: 'Ana', email: 'ana@example.test' }]);
  });

  it('returns an empty list for no users', () => {
    expect(toMentionUsers([])).toEqual([]);
  });
});
