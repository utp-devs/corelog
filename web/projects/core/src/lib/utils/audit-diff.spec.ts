import { auditDiff } from './audit-diff';

describe('auditDiff', () => {
  it('marks changed, added and removed fields', () => {
    const rows = auditDiff({ name: 'A', old: 1, keep: true }, { name: 'B', fresh: 2, keep: true });
    const byField = Object.fromEntries(rows.map((row) => [row.field, row.change]));
    expect(byField).toEqual({ name: 'changed', old: 'removed', keep: 'same', fresh: 'added' });
  });

  it('flattens nested values and shows them as text', () => {
    const rows = auditDiff(null, { a: { b: ['x', 'y'] }, html: '<b>x</b>' });
    expect(rows.find((row) => row.field === 'a.b')?.after).toBe('x, y');
    expect(rows.find((row) => row.field === 'html')?.after).toBe('<b>x</b>');
  });

  it('returns nothing when both sides are empty', () => {
    expect(auditDiff(null, null)).toEqual([]);
  });
});
