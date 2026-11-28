import type {
  DashboardGadget,
  DashboardGadgetInput,
  DashboardLayout,
} from '../models/dashboard-v2.model';

const DRAFT_PREFIX = 'draft-';

/** Number of columns of a layout. */
export function layoutColumnCount(layout: DashboardLayout): number {
  switch (layout) {
    case '1':
      return 1;
    case '3':
      return 3;
    default:
      return 2;
  }
}

/** CSS grid track list of a layout, for example `2fr 1fr`. */
export function layoutTracks(layout: DashboardLayout): string {
  switch (layout) {
    case '1':
      return 'minmax(0, 1fr)';
    case '3':
      return 'repeat(3, minmax(0, 1fr))';
    case '2-1':
      return 'minmax(0, 2fr) minmax(0, 1fr)';
    case '1-2':
      return 'minmax(0, 1fr) minmax(0, 2fr)';
    default:
      return 'repeat(2, minmax(0, 1fr))';
  }
}

export function newDraftId(): string {
  return `${DRAFT_PREFIX}${Math.random().toString(36).slice(2, 10)}`;
}

export function isDraftId(id: string): boolean {
  return id.startsWith(DRAFT_PREFIX);
}

function clampColumn(column: number, columns: number): number {
  return Math.min(Math.max(0, column), columns - 1);
}

/** Gadgets of each column in display order. Gadgets of a column that no longer exists go to the last one. */
export function gadgetsByColumn(
  gadgets: readonly DashboardGadget[],
  layout: DashboardLayout,
): DashboardGadget[][] {
  const count = layoutColumnCount(layout);
  const columns: DashboardGadget[][] = Array.from({ length: count }, () => []);
  for (const gadget of gadgets) columns[clampColumn(gadget.columnIndex, count)].push(gadget);
  // Gadgets that come from a removed column keep their original column order after the native ones.
  for (const column of columns) {
    column.sort((a, b) => a.columnIndex - b.columnIndex || a.position - b.position);
  }
  return columns;
}

/** Rebuilds the flat list from columns, numbering `columnIndex` and `position` without gaps. */
export function flattenColumns(
  columns: readonly (readonly DashboardGadget[])[],
): DashboardGadget[] {
  return columns.flatMap((column, columnIndex) =>
    column.map((gadget, position) => ({ ...gadget, columnIndex, position })),
  );
}

/** Moves a gadget to a column and an index (the index is clamped to the target column). */
export function moveGadget(
  gadgets: readonly DashboardGadget[],
  layout: DashboardLayout,
  id: string,
  toColumn: number,
  toIndex: number,
): DashboardGadget[] {
  const columns = gadgetsByColumn(gadgets, layout);
  let moving: DashboardGadget | undefined;
  for (const column of columns) {
    const at = column.findIndex((gadget) => gadget.id === id);
    if (at >= 0) moving = column.splice(at, 1)[0];
  }
  if (!moving) return flattenColumns(columns);
  const target = columns[clampColumn(toColumn, columns.length)];
  target.splice(Math.min(Math.max(0, toIndex), target.length), 0, moving);
  return flattenColumns(columns);
}

/** Adds a gadget at the end of a column. */
export function addGadget(
  gadgets: readonly DashboardGadget[],
  layout: DashboardLayout,
  gadget: Omit<DashboardGadget, 'columnIndex' | 'position'>,
  column = 0,
): DashboardGadget[] {
  const columns = gadgetsByColumn(gadgets, layout);
  const target = clampColumn(column, columns.length);
  columns[target].push({ ...gadget, columnIndex: target, position: columns[target].length });
  return flattenColumns(columns);
}

export function removeGadget(
  gadgets: readonly DashboardGadget[],
  layout: DashboardLayout,
  id: string,
): DashboardGadget[] {
  return flattenColumns(gadgetsByColumn(gadgets, layout).map((c) => c.filter((g) => g.id !== id)));
}

/** Replaces the title and configuration of a gadget. */
export function updateGadget(
  gadgets: readonly DashboardGadget[],
  id: string,
  change: Pick<DashboardGadget, 'title' | 'config'>,
): DashboardGadget[] {
  return gadgets.map((gadget) => (gadget.id === id ? { ...gadget, ...change } : gadget));
}

/** Gadgets after a layout change: those of removed columns are appended to the last one. */
export function applyLayout(
  gadgets: readonly DashboardGadget[],
  layout: DashboardLayout,
): DashboardGadget[] {
  return flattenColumns(gadgetsByColumn(gadgets, layout));
}

/** Column and index of a gadget, or null when it is not in the list. */
export function locateGadget(
  gadgets: readonly DashboardGadget[],
  layout: DashboardLayout,
  id: string,
): { column: number; index: number } | null {
  const columns = gadgetsByColumn(gadgets, layout);
  for (let column = 0; column < columns.length; column += 1) {
    const index = columns[column].findIndex((gadget) => gadget.id === id);
    if (index >= 0) return { column, index };
  }
  return null;
}

/** Body of a layout replacement: local drafts have no id, so the API creates them. */
export function toGadgetInputs(gadgets: readonly DashboardGadget[]): DashboardGadgetInput[] {
  return gadgets.map((gadget) => ({
    ...(isDraftId(gadget.id) ? {} : { id: gadget.id }),
    type: gadget.type,
    title: gadget.title,
    config: gadget.config,
    columnIndex: gadget.columnIndex,
    position: gadget.position,
  }));
}

/** Whether two gadget lists describe the same dashboard content (order, columns, titles, configuration). */
export function sameGadgets(a: readonly DashboardGadget[], b: readonly DashboardGadget[]): boolean {
  if (a.length !== b.length) return false;
  const left = [...a].sort((x, y) => x.id.localeCompare(y.id));
  const right = [...b].sort((x, y) => x.id.localeCompare(y.id));
  return left.every((gadget, index) => {
    const other = right[index];
    return (
      gadget.id === other.id &&
      gadget.type === other.type &&
      gadget.title === other.title &&
      gadget.columnIndex === other.columnIndex &&
      gadget.position === other.position &&
      JSON.stringify(gadget.config) === JSON.stringify(other.config)
    );
  });
}
