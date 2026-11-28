import type {
  BoardEstimation,
  BoardView,
  BoardViewCard,
  BoardViewColumn,
  BoardViewUnmapped,
  WipState,
} from '../models/board-v2.model';

/** Id of the implicit column that holds cards whose status no column maps. */
export const UNMAPPED_COLUMN_ID = 'unmapped';

const SECONDS_PER_HOUR = 3600;

/** Neighbors a card gets when it is dropped at `index` of a list that does not contain it. */
export interface DropNeighbors {
  readonly beforeIssueId?: string;
  readonly afterIssueId?: string;
}

/** State of a column against its limits, evaluated on the number of cards it holds. */
export function wipStateOf(total: number, min: number | null, max: number | null): WipState {
  if (max !== null && total > max) return 'over_max';
  if (max !== null && total === max) return 'at_max';
  if (min !== null && total < min) return 'below_min';
  return 'ok';
}

/** What one card adds to a column header under the given estimation. */
export function cardEstimate(card: BoardViewCard, estimation: BoardEstimation): number {
  switch (estimation) {
    case 'story_points':
      return card.storyPoints ?? 0;
    case 'issue_count':
      return 1;
    case 'time':
      return (card.remainingEstimateSeconds ?? 0) / SECONDS_PER_HOUR;
  }
}

/** Sum of the estimates of the cards, rounded to one decimal. */
export function estimateOf(cards: readonly BoardViewCard[], estimation: BoardEstimation): number {
  const sum = cards.reduce((total, card) => total + cardEstimate(card, estimation), 0);
  return Math.round(sum * 10) / 10;
}

/** The ids of the list around the position where a card is dropped: the anchor the server needs to rank it. */
export function dropNeighbors(ids: readonly string[], index: number): DropNeighbors {
  const next = ids[index];
  if (next !== undefined) return { beforeIssueId: next };
  const previous = ids[index - 1];
  return previous !== undefined ? { afterIssueId: previous } : {};
}

export interface MoveCardOptions extends DropNeighbors {
  readonly swimlaneId?: string;
}

function insertAt(
  cards: readonly BoardViewCard[],
  card: BoardViewCard,
  options: MoveCardOptions,
): BoardViewCard[] {
  const result = [...cards];
  let index = result.length;
  if (options.beforeIssueId) {
    const found = result.findIndex((candidate) => candidate.id === options.beforeIssueId);
    if (found >= 0) index = found;
  } else if (options.afterIssueId) {
    const found = result.findIndex((candidate) => candidate.id === options.afterIssueId);
    if (found >= 0) index = found + 1;
  }
  result.splice(index, 0, card);
  return result;
}

function withCards(
  column: BoardViewColumn,
  cards: readonly BoardViewCard[],
  delta: number,
  estimation: BoardEstimation,
  estimateDelta: number,
): BoardViewColumn {
  const total = column.total + delta;
  return {
    ...column,
    cards,
    count: column.count + delta,
    total,
    estimate: Math.round((column.estimate + estimateDelta) * 10) / 10,
    wipState: wipStateOf(total, column.minIssues, column.maxIssues),
  };
}

/**
 * The view after a card moved to another column (or to another place of its own column), computed
 * without the server: counts, estimates and WIP states follow. The server answer replaces it later.
 * Unknown cards or columns return the view unchanged.
 */
export function moveCardInView(
  view: BoardView,
  cardId: string,
  targetColumnId: string,
  options: MoveCardOptions = {},
): BoardView {
  const estimation = view.board.estimation;
  const all: readonly { id: string; cards: readonly BoardViewCard[] }[] = [
    ...view.columns,
    ...(view.unmappedColumn ? [view.unmappedColumn] : []),
  ];
  const source = all.find((column) => column.cards.some((card) => card.id === cardId));
  const target = view.columns.find((column) => column.id === targetColumnId);
  const found = source?.cards.find((card) => card.id === cardId);
  if (!source || !target || !found) return view;

  const targetStatus = view.board.workflowStatuses.find((status) =>
    target.statusIds.includes(status.id),
  );
  const moved: BoardViewCard = {
    ...found,
    status:
      source.id === target.id || !targetStatus
        ? found.status
        : { id: targetStatus.id, name: targetStatus.name, category: targetStatus.category },
    swimlaneId: options.swimlaneId ?? found.swimlaneId,
  };
  const weight = cardEstimate(found, estimation);
  const sameColumn = source.id === target.id;

  const columns = view.columns.map((column) => {
    if (column.id === target.id) {
      const base = column.cards.filter((card) => card.id !== cardId);
      return withCards(
        column,
        insertAt(base, moved, options),
        sameColumn ? 0 : 1,
        estimation,
        sameColumn ? 0 : weight,
      );
    }
    if (column.id === source.id) {
      return withCards(
        column,
        column.cards.filter((card) => card.id !== cardId),
        -1,
        estimation,
        -weight,
      );
    }
    return column;
  });

  let unmapped: BoardViewUnmapped | null = view.unmappedColumn;
  if (unmapped && unmapped.id === source.id) {
    unmapped = {
      ...unmapped,
      cards: unmapped.cards.filter((card) => card.id !== cardId),
      count: unmapped.count - 1,
      total: unmapped.total - 1,
      estimate: Math.round((unmapped.estimate - weight) * 10) / 10,
    };
  }
  return {
    ...view,
    columns,
    unmappedColumn: unmapped,
    unmappedCount: unmapped ? unmapped.count : view.unmappedCount,
  };
}

/** Column that follows (`1`) or precedes (`-1`) the given one, or null at the ends of the board. */
export function adjacentColumnId(
  columns: readonly { readonly id: string }[],
  currentId: string,
  direction: 1 | -1,
): string | null {
  const index = columns.findIndex((column) => column.id === currentId);
  if (index < 0) return null;
  return columns[index + direction]?.id ?? null;
}
