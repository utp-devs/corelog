import type { DashboardGadget, DashboardLayout } from '../models/dashboard-v2.model';
import {
  addGadget,
  applyLayout,
  gadgetsByColumn,
  isDraftId,
  layoutColumnCount,
  layoutTracks,
  locateGadget,
  moveGadget,
  newDraftId,
  removeGadget,
  sameGadgets,
  toGadgetInputs,
  updateGadget,
} from './gadget-layout';

function gadget(id: string, columnIndex: number, position: number): DashboardGadget {
  return { id, type: 'text', title: id, config: {}, columnIndex, position };
}

const BASE = [gadget('a', 0, 0), gadget('b', 0, 1), gadget('c', 1, 0)];
const ids = (list: readonly DashboardGadget[]) =>
  list.map((g) => `${g.id}@${g.columnIndex}.${g.position}`);

describe('gadget layout', () => {
  it('knows the columns and tracks of every layout', () => {
    const layouts: DashboardLayout[] = ['1', '2', '3', '2-1', '1-2'];
    expect(layouts.map(layoutColumnCount)).toEqual([1, 2, 3, 2, 2]);
    expect(layoutTracks('2-1')).toBe('minmax(0, 2fr) minmax(0, 1fr)');
    expect(layoutTracks('1')).toBe('minmax(0, 1fr)');
  });

  it('groups gadgets by column in display order and clamps removed columns', () => {
    const columns = gadgetsByColumn([gadget('x', 2, 0), gadget('b', 0, 1), gadget('a', 0, 0)], '2');
    expect(columns.map((c) => c.map((g) => g.id))).toEqual([['a', 'b'], ['x']]);
  });

  it('moves a gadget inside a column and across columns, numbering without gaps', () => {
    expect(ids(moveGadget(BASE, '2', 'a', 0, 1))).toEqual(['b@0.0', 'a@0.1', 'c@1.0']);
    expect(ids(moveGadget(BASE, '2', 'a', 1, 0))).toEqual(['b@0.0', 'a@1.0', 'c@1.1']);
    expect(ids(moveGadget(BASE, '2', 'b', 1, 99))).toEqual(['a@0.0', 'c@1.0', 'b@1.1']);
  });

  it('ignores a move of an unknown gadget', () => {
    expect(ids(moveGadget(BASE, '2', 'zzz', 1, 0))).toEqual(['a@0.0', 'b@0.1', 'c@1.0']);
  });

  it('adds at the end of a column and removes with renumbering', () => {
    const added = addGadget(BASE, '2', { id: 'n', type: 'pie', title: 'n', config: {} }, 1);
    expect(ids(added)).toEqual(['a@0.0', 'b@0.1', 'c@1.0', 'n@1.1']);
    expect(ids(removeGadget(added, '2', 'a'))).toEqual(['b@0.0', 'c@1.0', 'n@1.1']);
  });

  it('moves gadgets of removed columns to the last one when the layout shrinks', () => {
    const three = [gadget('a', 0, 0), gadget('b', 1, 0), gadget('c', 2, 0)];
    expect(ids(applyLayout(three, '2'))).toEqual(['a@0.0', 'b@1.0', 'c@1.1']);
    expect(ids(applyLayout(three, '1'))).toEqual(['a@0.0', 'b@0.1', 'c@0.2']);
  });

  it('updates the title and configuration of one gadget', () => {
    const changed = updateGadget(BASE, 'b', { title: 'Nuevo', config: { limit: 5 } });
    expect(changed[1]).toMatchObject({ title: 'Nuevo', config: { limit: 5 } });
    expect(changed[0]).toBe(BASE[0]);
  });

  it('locates a gadget by column and index', () => {
    expect(locateGadget(BASE, '2', 'b')).toEqual({ column: 0, index: 1 });
    expect(locateGadget(BASE, '2', 'nope')).toBeNull();
  });

  it('sends drafts without id and stored gadgets with it', () => {
    const draft = newDraftId();
    expect(isDraftId(draft)).toBe(true);
    const inputs = toGadgetInputs([gadget('a', 0, 0), gadget(draft, 1, 0)]);
    expect(inputs[0]).toMatchObject({ id: 'a', columnIndex: 0, position: 0 });
    expect('id' in inputs[1]).toBe(false);
  });

  it('compares gadget lists by content', () => {
    expect(sameGadgets(BASE, [...BASE].reverse())).toBe(true);
    expect(sameGadgets(BASE, moveGadget(BASE, '2', 'a', 1, 0))).toBe(false);
    expect(sameGadgets(BASE, updateGadget(BASE, 'a', { title: 'a', config: { x: 1 } }))).toBe(
      false,
    );
  });
});
