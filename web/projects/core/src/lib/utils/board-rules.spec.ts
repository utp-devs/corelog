import { isColumnFull, parseWipLimit } from './board-rules';

describe('board rules', () => {
  it('treats a column without limit as never full', () => {
    expect(isColumnFull({ count: 40 })).toBe(false);
  });

  it('treats a column as full when the count reaches the limit', () => {
    expect(isColumnFull({ count: 2, wipLimit: 3 })).toBe(false);
    expect(isColumnFull({ count: 3, wipLimit: 3 })).toBe(true);
    expect(isColumnFull({ count: 4, wipLimit: 3 })).toBe(true);
  });

  it('reads an empty field as no limit', () => {
    expect(parseWipLimit('   ')).toEqual({ valid: true, limit: null });
  });

  it('accepts whole numbers above zero', () => {
    expect(parseWipLimit(' 5 ')).toEqual({ valid: true, limit: 5 });
  });

  it('rejects zero, negatives, decimals and text', () => {
    expect(parseWipLimit('0')).toEqual({ valid: false });
    expect(parseWipLimit('-2')).toEqual({ valid: false });
    expect(parseWipLimit('2.5')).toEqual({ valid: false });
    expect(parseWipLimit('abc')).toEqual({ valid: false });
  });
});
