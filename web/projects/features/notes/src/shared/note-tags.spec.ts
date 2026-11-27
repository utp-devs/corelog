import { addNoteTags, TAG_TOO_LONG, TOO_MANY_TAGS } from './note-tags';

describe('addNoteTags', () => {
  it('normalizes typed tags and ignores repeats', () => {
    expect(addNoteTags(['trabajo'], ' Trabajo, #Ideas ,, urgente ')).toEqual({
      tags: ['trabajo', 'ideas', 'urgente'],
      error: null,
    });
  });

  it('rejects a tag over 30 characters and keeps the rest', () => {
    const result = addNoteTags([], `${'a'.repeat(31)}, ok`);
    expect(result.tags).toEqual(['ok']);
    expect(result.error).toBe(TAG_TOO_LONG);
  });

  it('accepts exactly 30 characters', () => {
    expect(addNoteTags([], 'a'.repeat(30)).error).toBeNull();
  });

  it('stops at 10 tags with a message', () => {
    const current = Array.from({ length: 9 }, (_, i) => `t${i}`);
    const result = addNoteTags(current, 'uno, dos');
    expect(result.tags).toHaveLength(10);
    expect(result.tags).toContain('uno');
    expect(result.error).toBe(TOO_MANY_TAGS);
  });
});
