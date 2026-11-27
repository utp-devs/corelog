import type { BoardColumn } from '../models/board.model';

/** A column is full when it has a WIP limit and already holds that many cards. */
export function isColumnFull(column: Pick<BoardColumn, 'count' | 'wipLimit'>): boolean {
  return column.wipLimit !== undefined && column.count >= column.wipLimit;
}

/** Result of reading a WIP limit typed by the user. */
export type WipLimitInput =
  { readonly valid: true; readonly limit: number | null } | { readonly valid: false };

/**
 * Reads a WIP limit typed in a field. An empty field means "no limit" (null). Any other value must
 * be a whole number greater than zero.
 */
export function parseWipLimit(raw: string): WipLimitInput {
  const trimmed = raw.trim();
  if (trimmed === '') {
    return { valid: true, limit: null };
  }
  const value = Number(trimmed);
  if (!Number.isInteger(value) || value < 1) {
    return { valid: false };
  }
  return { valid: true, limit: value };
}
