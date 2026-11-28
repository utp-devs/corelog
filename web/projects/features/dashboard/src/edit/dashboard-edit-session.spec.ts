import type { Dashboard, DashboardGadget } from '@corelog/core';

import { DashboardEditSession } from './dashboard-edit-session';

function gadget(id: string, column: number, position: number): DashboardGadget {
  return {
    id,
    type: 'text',
    title: id,
    config: { markdown: id },
    columnIndex: column,
    position,
  };
}

const STORED: Pick<Dashboard, 'layout' | 'gadgets'> = {
  layout: '2',
  gadgets: [gadget('a', 0, 0), gadget('b', 0, 1), gadget('c', 1, 0)],
};

function started(): DashboardEditSession {
  const session = new DashboardEditSession();
  session.begin(STORED);
  return session;
}

describe('DashboardEditSession', () => {
  it('starts clean and becomes dirty on any change', () => {
    const session = started();
    expect(session.dirty()).toBe(false);
    session.move('a', 1, 0);
    expect(session.dirty()).toBe(true);
    session.discard();
    expect(session.dirty()).toBe(false);
    expect(session.columns().map((c) => c.map((g) => g.id))).toEqual([['a', 'b'], ['c']]);
  });

  it('is clean again when a change is undone by hand', () => {
    const session = started();
    session.move('a', 1, 0);
    session.move('a', 0, 0);
    expect(session.dirty()).toBe(false);
  });

  it('adds a gadget with its defaults to the emptiest column', () => {
    const session = started();
    const added = session.add('pie');
    expect(added).toMatchObject({ type: 'pie', title: 'Gráfico circular', columnIndex: 1 });
    expect(added?.id.startsWith('draft-')).toBe(true);
    expect(session.isUnsaved(added as DashboardGadget)).toBe(true);
    expect(session.dirty()).toBe(true);
  });

  it('refuses to add more than the allowed number of gadgets', () => {
    const session = new DashboardEditSession();
    session.begin({ layout: '3', gadgets: [] });
    for (let i = 0; i < 20; i += 1) expect(session.add('text')).not.toBeNull();
    expect(session.canAdd()).toBe(false);
    expect(session.add('text')).toBeNull();
  });

  it('removes and nudges gadgets inside their column', () => {
    const session = started();
    expect(session.nudge('a', -1)).toBe(false);
    expect(session.nudge('a', 1)).toBe(true);
    expect(session.columns()[0].map((g) => g.id)).toEqual(['b', 'a']);
    session.remove('b');
    expect(session.columns()[0].map((g) => g.id)).toEqual(['a']);
  });

  it('marks a reconfigured gadget as unsaved and cleans the configuration', () => {
    const session = started();
    session.update('a', '  Nuevo  ', { markdown: 'otro', ignored: null });
    const updated = session.gadgets().find((g) => g.id === 'a') as DashboardGadget;
    expect(updated.title).toBe('Nuevo');
    expect(updated.config).toEqual({ markdown: 'otro' });
    expect(session.isUnsaved(updated)).toBe(true);
    expect(session.isUnsaved(session.gadgets().find((g) => g.id === 'b') as DashboardGadget)).toBe(
      false,
    );
  });

  it('keeps every gadget when the layout shrinks', () => {
    const session = started();
    session.setLayout('1');
    expect(session.columnCount()).toBe(1);
    expect(session.columns()[0].map((g) => g.id)).toEqual(['a', 'b', 'c']);
    expect(session.dirty()).toBe(true);
  });

  it('takes the saved dashboard as the new baseline after a commit', () => {
    const session = started();
    session.move('a', 1, 0);
    session.commit({ layout: '2', gadgets: session.gadgets() });
    expect(session.dirty()).toBe(false);
  });
});
