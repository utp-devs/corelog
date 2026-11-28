import {
  BoardEstimation,
  BoardView,
  BoardViewCard,
  BoardViewColumn,
  BoardViewUnmapped,
  UNMAPPED_COLUMN_ID,
  WipState,
  estimateOf,
} from '@corelog/core';

/** Cards of one column inside one swimlane: a place where cards can be dropped. */
export interface BoardCell {
  readonly columnId: string;
  readonly laneId: string;
  readonly cards: readonly BoardViewCard[];
  readonly wipState: WipState;
}

/** A swimlane with a cell per column; a board without swimlanes has one row without header. */
export interface BoardRow {
  readonly id: string;
  readonly name: string;
  /** Rows without lanes show no header. */
  readonly hasHeader: boolean;
  readonly count: number;
  readonly cells: readonly BoardCell[];
}

/** Column headers in display order, the implicit unmapped column last. */
export interface BoardHeader {
  readonly id: string;
  readonly name: string;
  readonly color: string;
  readonly count: number;
  readonly total: number;
  readonly estimate: number;
  readonly wipState: WipState;
  readonly min: number | null;
  readonly max: number | null;
  readonly implicit: boolean;
}

/** Headers of the columns of a view, plus the unmapped one when it has cards and the board shows it. */
export function boardHeaders(
  view: BoardView,
  estimation: BoardEstimation = view.board.estimation,
): BoardHeader[] {
  const headers: BoardHeader[] = view.columns.map((column: BoardViewColumn) => ({
    id: column.id,
    name: column.name,
    color: column.color,
    count: column.count,
    total: column.total,
    estimate: estimateOf(column.cards, estimation),
    wipState: column.wipState,
    min: column.minIssues,
    max: column.maxIssues,
    implicit: false,
  }));
  const unmapped = visibleUnmapped(view);
  if (unmapped) {
    headers.push({
      id: UNMAPPED_COLUMN_ID,
      name: unmapped.name || 'Sin asignar',
      color: 'gray',
      count: unmapped.count,
      total: unmapped.total,
      estimate: estimateOf(unmapped.cards, estimation),
      wipState: 'ok',
      min: null,
      max: null,
      implicit: true,
    });
  }
  return headers;
}

/** The unmapped column when the board shows it and it holds something. */
export function visibleUnmapped(view: BoardView): BoardViewUnmapped | null {
  const unmapped = view.unmappedColumn;
  return unmapped && view.board.showUnmappedLane && unmapped.cards.length > 0 ? unmapped : null;
}

/** Rows (swimlanes) of the board with the cards of every cell. */
export function boardRows(view: BoardView): BoardRow[] {
  const headers = boardHeaders(view);
  const cardsOf = new Map<string, readonly BoardViewCard[]>();
  for (const column of view.columns) cardsOf.set(column.id, column.cards);
  const unmapped = visibleUnmapped(view);
  if (unmapped) cardsOf.set(UNMAPPED_COLUMN_ID, unmapped.cards);

  const hasLanes = view.board.swimlaneMode !== 'none' && view.swimlanes.length > 0;
  if (!hasLanes) {
    const cells = headers.map((header) => ({
      columnId: header.id,
      laneId: '',
      cards: cardsOf.get(header.id) ?? [],
      wipState: header.wipState,
    }));
    return [
      {
        id: '',
        name: '',
        hasHeader: false,
        count: cells.reduce((total, cell) => total + cell.cards.length, 0),
        cells,
      },
    ];
  }

  const lanes = view.swimlanes.map((lane) => ({ id: lane.id, name: lane.name }));
  const known = new Set(lanes.map((lane) => lane.id));
  const stray = [...cardsOf.values()].some((cards) =>
    cards.some((card) => !known.has(card.swimlaneId)),
  );
  if (stray) lanes.push({ id: '__other', name: 'Otras' });

  return lanes.map((lane) => {
    const cells = headers.map((header) => ({
      columnId: header.id,
      laneId: lane.id,
      cards: (cardsOf.get(header.id) ?? []).filter((card) =>
        lane.id === '__other' ? !known.has(card.swimlaneId) : card.swimlaneId === lane.id,
      ),
      wipState: header.wipState,
    }));
    return {
      id: lane.id,
      name: lane.name,
      hasHeader: true,
      count: cells.reduce((total, cell) => total + cell.cards.length, 0),
      cells,
    };
  });
}

/** Spanish text for the live region after a move by keyboard or menu. */
export function moveAnnouncement(cardKey: string, columnName: string): string {
  return `${cardKey} movida a ${columnName}`;
}

/** Text of the card estimate a column header shows under the chosen estimation. */
export function estimateText(estimate: number, estimation: BoardEstimation): string {
  if (estimation === 'issue_count') return '';
  const rounded = Math.round(estimate * 10) / 10;
  const text = new Intl.NumberFormat('es', { maximumFractionDigits: 1 }).format(rounded);
  return estimation === 'time' ? `${text} h` : `${text} pts`;
}
